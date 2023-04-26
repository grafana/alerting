package images

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/grafana/alerting/models"
	"github.com/prometheus/alertmanager/types"
)

type FakeImageStore struct {
	Images []*Image
}

// GetImage returns an image with the same token.
func (f *FakeImageStore) GetImage(_ context.Context, token string) (*Image, error) {
	for _, img := range f.Images {
		if img.Token == token {
			return img, nil
		}
	}
	return nil, ErrImageNotFound
}

// GetImageURL returns the URL of the image associated with a given alert.
func (f *FakeImageStore) GetImageURL(_ context.Context, alert types.Alert) (string, error) {
	uri, err := getImageURI(alert)
	if err != nil {
		return "", err
	}

	for _, img := range f.Images {
		if img.Token == uri || img.URL == uri {
			return img.URL, nil
		}
	}
	return "", ErrImageNotFound
}

// GetRawImage returns an io.Reader to read the bytes of the image associated with a given alert.
func (f *FakeImageStore) GetRawImage(_ context.Context, alert types.Alert) (io.Reader, error) {
	uri, err := getImageURI(alert)
	if err != nil {
		return nil, err
	}

	uriString := string(uri)
	for _, img := range f.Images {
		if img.Token == uriString || img.URL == uriString {
			return strings.NewReader("test"), nil
		}
	}
	return nil, ErrImageNotFound
}

// getImageURI is a helper function to retrieve the image URI from the alert annotations as a string.
func getImageURI(alert types.Alert) (string, error) {
	uri, ok := alert.Annotations[models.ImageTokenAnnotation]
	if !ok {
		return "", fmt.Errorf("no image uri in annotations")
	}
	return string(uri), nil
}

// NewFakeImageStore returns an image store with N test images.
// Each image has a token and a URL, but does not have a file on disk.
func NewFakeImageStore(n int) ImageStore {
	s := FakeImageStore{}
	for i := 1; i <= n; i++ {
		s.Images = append(s.Images, &Image{
			Token:     fmt.Sprintf("test-image-%d", i),
			URL:       fmt.Sprintf("https://www.example.com/test-image-%d.jpg", i),
			CreatedAt: time.Now().UTC(),
		})
	}
	return &s
}

// NewFakeImageStoreWithFile returns an image store with N test images.
// Each image has a token, path and a URL, where the path is 1x1 transparent
// PNG on disk. The test should call deleteFunc to delete the images from disk
// at the end of the test.
// nolint:deadcode,unused
func NewFakeImageStoreWithFile(t *testing.T, n int) ImageStore {
	var (
		files []string
		s     FakeImageStore
	)

	t.Cleanup(func() {
		// remove all files from disk
		for _, f := range files {
			if err := os.Remove(f); err != nil {
				t.Logf("failed to delete file: %s", err)
			}
		}
	})

	for i := 1; i <= n; i++ {
		file, err := newTestImage()
		if err != nil {
			t.Fatalf("failed to create test image: %s", err)
		}
		files = append(files, file)
		s.Images = append(s.Images, &Image{
			Token:     fmt.Sprintf("test-image-%d", i),
			Path:      file,
			URL:       fmt.Sprintf("https://www.example.com/test-image-%d", i),
			CreatedAt: time.Now().UTC(),
		})
	}

	return &s
}

func newTestImage() (string, error) {
	f, err := os.CreateTemp("", "test-image-*.png")
	if err != nil {
		return "", fmt.Errorf("failed to create temp image: %s", err)
	}

	// 1x1 transparent PNG
	b, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII=")
	if err != nil {
		return f.Name(), fmt.Errorf("failed to decode PNG data: %s", err)
	}

	if _, err := f.Write(b); err != nil {
		return f.Name(), fmt.Errorf("failed to write to file: %s", err)
	}

	if err := f.Close(); err != nil {
		return f.Name(), fmt.Errorf("failed to close file: %s", err)
	}

	return f.Name(), nil
}
