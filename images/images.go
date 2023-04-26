package images

import (
	"context"
	"errors"
	"time"
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

type Provider interface {
	GetImage(ctx context.Context, token string) (*Image, error)
}

type UnavailableProvider struct{}

// GetImage returns the image with the corresponding token, or ErrImageNotFound.
func (u *UnavailableProvider) GetImage(context.Context, string) (*Image, error) {
	return nil, ErrImagesUnavailable
}
