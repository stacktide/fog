package fog

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/adrg/xdg"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
	"gopkg.in/yaml.v3"
)

// manifests holds our static image manifests from build time.
//
//go:embed images/*
var manifests embed.FS

// Image defines a virtual machine image.
type Image struct {
	Name     string
	Url      string
	Checksum string
	Arch     string
	Tags     []string
}

type ImagePullOptions struct {
	//
}

type ImageRepository struct {
	// dataDir is the directory to use for image data
	dataDir string
	// dataFs is a filesystem containing the image data
	dataFs fs.FS
	imgs   []*Image
	pullMu sync.Mutex
	// pulls tracks the image SHAs we are currently pulling
	pulls map[string]int
}

func NewImageRepository() *ImageRepository {
	dataDir := path.Join(xdg.DataHome, "fog")

	dataFs := os.DirFS(dataDir)

	r := &ImageRepository{
		dataDir: dataDir,
		dataFs:  dataFs,
	}

	return r
}

func (r *ImageRepository) LoadManifests() error {
	imgs, err := loadManifests()

	if err != nil {
		return fmt.Errorf("loading image manifests: %w", err)
	}

	r.imgs = imgs

	return nil
}

func (r *ImageRepository) Find(ctx context.Context, rawImage string) (*Image, error) {
	name, tag, err := ParseImageName(rawImage)

	if err != nil {
		return &Image{}, fmt.Errorf("parsing image name: %w", err)
	}

	for _, img := range r.imgs {
		if img.Name != name {
			continue
		}

		for _, t := range img.Tags {
			if t == tag {
				return img, nil
			}
		}
	}

	return &Image{}, fmt.Errorf("Image not found")
}

func (r *ImageRepository) ImagePath(img *Image) string {
	return path.Join(r.dataDir, "images", img.Checksum+".qcow2")
}

func (r *ImageRepository) Pull(ctx context.Context, img *Image, opts ImagePullOptions) error {
	r.pullMu.Lock()

	if _, prs := r.pulls[img.Checksum]; prs {
		r.pullMu.Unlock()
		return nil
	}

	r.pullMu.Unlock()

	defer func() {
		r.pullMu.Lock()
		delete(r.pulls, img.Checksum)
		r.pullMu.Unlock()
	}()

	fmt.Printf("Trying to pull %s %s...\n", img.Name, img.Checksum[0:8])

	destFile := r.ImagePath(img)

	if _, err := os.Stat(destFile); err == nil {
		fmt.Println("Already exists")

		return nil
	}

	err := os.MkdirAll(path.Join(r.dataDir, "images"), os.ModePerm)

	if err != nil {
		return fmt.Errorf("creating image directory: %w", err)
	}

	err = DownloadFile(destFile, img.Url, img.Checksum)

	if err != nil {
		return fmt.Errorf("downloading image: %w", err)
	}

	return nil
}

func loadManifests() ([]*Image, error) {
	var imgs []*Image

	err := fs.WalkDir(manifests, ".", func(filepath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if path.Ext(filepath) != ".yaml" {
			return nil
		}

		buf, err := os.ReadFile(filepath)

		if err != nil {
			return fmt.Errorf("reading file %s: %w", filepath, err)
		}

		img := Image{}

		err = yaml.Unmarshal(buf, &img)

		if err != nil {
			return fmt.Errorf("parsing YAML: %w", err)
		}

		imgs = append(imgs, &img)

		return nil
	})

	return imgs, err
}

func ParseImageName(name string) (string, string, error) {
	if !strings.Contains(name, ":") {
		return name, "latest", nil
	}

	parts := strings.SplitN(name, ":", 2)

	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid image name '%s'", name)
	}

	return parts[0], parts[1], nil
}

func DownloadFile(filepath string, url string, checksum string) error {
	// TODO: use ctx here

	tmpFile, err := os.Create(filepath + ".tmp")

	if err != nil {
		return err
	}

	resp, err := http.Get(url)

	if err != nil {
		tmpFile.Close()
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading image %s: HTTP error %s", url, resp.Status)
	}

	byteSize := resp.ContentLength

	prefix := "Downloading image"

	onComplete := prefix + ": done"

	p := mpb.New(
		mpb.WithWidth(80),
		mpb.WithRefreshRate(180*time.Millisecond),
	)

	bar := p.AddBar(byteSize,
		mpb.BarFillerClearOnComplete(),
		mpb.PrependDecorators(
			decor.OnComplete(decor.Name(prefix), onComplete),
		),
		mpb.AppendDecorators(
			decor.OnComplete(decor.CountersKibiByte("%.1f / %.1f"), ""),
		),
	)

	proxyReader := bar.ProxyReader(resp.Body)

	if _, err := io.Copy(tmpFile, proxyReader); err != nil {
		_ = proxyReader.Close()
		return err
	}

	p.Wait()

	if err := proxyReader.Close(); err != nil {
		return fmt.Errorf("closing image download reader: %w", err)
	}

	if _, err = tmpFile.Seek(0, 0); err != nil {
		return fmt.Errorf("seeking image file: %w", err)
	}

	h := sha256.New()

	if _, err := io.Copy(h, tmpFile); err != nil {
		return fmt.Errorf("verify checksum: %w", err)
	}

	sum := hex.EncodeToString(h.Sum(nil))

	if sum != checksum {
		return fmt.Errorf("checksum %s does not match expected sum %s", sum, checksum)
	}

	tmpFile.Close()

	if err = os.Rename(filepath+".tmp", filepath); err != nil {
		return err
	}

	return nil
}
