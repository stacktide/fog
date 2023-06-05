package fog

import (
	"context"
	"fmt"
	"path"

	"github.com/adrg/xdg"
	// "gopkg.in/yaml.v3"
)

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
}

func NewImageRepository() *ImageRepository {
	dataDir := path.Join(xdg.DataHome, "fog")

	r := &ImageRepository{
		dataDir,
	}

	return r
}

// Sync the image manifests
func (r ImageRepository) Sync(ctx context.Context) error {
	// TODO: sync from the github releases, then unzip into the data directory

	return nil
}

// Sync the image manifests, but only if no manifests are found
func (r ImageRepository) SyncIfMissing(ctx context.Context) error {
	return nil
}

func (r ImageRepository) Find(ctx context.Context, rawImage string) (*Image, error) {
	// TODO: find

	return &Image{}, fmt.Errorf("Image not found")
}

func (r ImageRepository) Pull(ctx context.Context, img *Image, opts ImagePullOptions) error {
	fmt.Printf("The data dir is %s\n", r.dataDir)

	// If we have no manifests, sync

	// Lookup in registry

	// Check if we have it already first

	return nil
}
