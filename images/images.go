package images

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/prometheus/alertmanager/types"
)

var (
	ErrImageNotFound = errors.New("image not found")

	// ErrImagesDone is used to stop iteration of subsequent images. It should be
	// returned from forEachFunc when either the intended image has been found or
	// the maximum number of images has been iterated.
	ErrImagesDone = errors.New("images done")

	ErrImagesUnavailable = errors.New("alert screenshots are unavailable")
)

type Image struct {
	Token     string
	Path      string
	URL       string
	CreatedAt time.Time
}

func (i Image) HasURL() bool {
	return i.URL != ""
}

type ImageStore interface {
	GetImage(ctx context.Context, token string) (*Image, error)
	GetImageURL(ctx context.Context, alert types.Alert) (string, error)
	GetRawImage(ctx context.Context, alert types.Alert) (io.Reader, error)
}

type UnavailableImageStore struct{}

// GetImage returns the image with the corresponding token, or ErrImageNotFound.
func (u *UnavailableImageStore) GetImage(context.Context, string) (*Image, error) {
	return nil, ErrImagesUnavailable
}

// GetImageURL returns the URL of the image associated with a given alert.
func (u *UnavailableImageStore) GetImageURL(context.Context, types.Alert) (string, error) {
	return "", ErrImagesUnavailable
}

// GetRawImage returns an io.Reader to read the bytes of the image associated with a given alert.
func (u *UnavailableImageStore) GetRawImage(context.Context, types.Alert) (io.Reader, error) {
	return nil, ErrImagesUnavailable
}
