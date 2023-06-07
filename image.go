package fog

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
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
	Name     string   `yaml:"name"`
	Url      string   `yaml:"url"`
	Checksum string   `yaml:"checksum"`
	Arch     string   `yaml:"arch"`
	Tags     []string `yaml:"tags"`
}

type ImagePullOptions struct {
	onProgress func()
}

type ImageRepository struct {
	dataDir string
	dataFs  fs.FS
}

func NewImageRepository() *ImageRepository {
	dataDir := path.Join(xdg.DataHome, "fog")

	dataFs := os.DirFS(dataDir)

	r := &ImageRepository{
		dataDir,
		dataFs,
	}

	return r
}

func (r ImageRepository) Find(ctx context.Context, rawImage string) (*Image, error) {
	name, tag, err := parseImageName(rawImage)

	if err != nil {
		return &Image{}, fmt.Errorf("parsing image name: %w", err)
	}

	imgs, err := loadManifests()

	if err != nil {
		return &Image{}, fmt.Errorf("loading image manifests: %w", err)
	}

	for _, img := range imgs {
		if img.Name != name {
			continue
		}

		for _, t := range img.Tags {
			if t == tag {
				return &img, nil
			}
		}
	}

	return &Image{}, fmt.Errorf("Image not found")
}

func (r ImageRepository) Pull(ctx context.Context, img *Image, opts ImagePullOptions) error {
	fmt.Printf("Trying to pull %s...\n", img.Name)

	// TODO: Check if we have it already first

	err := os.MkdirAll(path.Join(r.dataDir, "images"), os.ModePerm)

	if err != nil {
		return fmt.Errorf("creating image directory: %w", err)
	}

	err = DownloadFile(path.Join(r.dataDir, "images", img.Checksum+".qcow2"), img.Url, img.Checksum)

	if err != nil {
		return fmt.Errorf("downloading image: %w", err)
	}

	return nil
}

func loadManifests() ([]Image, error) {
	var imgs []Image

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

		imgs = append(imgs, img)

		return nil
	})

	return imgs, err
}

func parseImageName(name string) (string, string, error) {
	if !strings.Contains(name, ":") {
		return name, "latest", nil
	}

	parts := strings.SplitN(name, ":", 2)

	if len(parts) != 2 {
		return "", "", fmt.Errorf("Invalid image name '%s'", name)
	}

	return parts[0], parts[1], nil
}

func DownloadFile(filepath string, url string, checksum string) error {
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
		log.Fatalf("Error closing reader: %s", err)
	}

	h := sha256.New()

	if _, err := io.Copy(h, tmpFile); err != nil {
		return fmt.Errorf("verify checksum: %w", err)
	}

	fmt.Printf("%x", h.Sum(nil))

	sum := hex.EncodeToString(h.Sum(nil))

	if sum != checksum {
		return fmt.Errorf("Checksum %s does not match expected sum %s", sum, checksum)
	}

	tmpFile.Close()

	if err = os.Rename(filepath+".tmp", filepath); err != nil {
		return err
	}

	return nil
}
