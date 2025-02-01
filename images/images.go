package images

import (
	"context"
	"errors"

	"github.com/prometheus/alertmanager/types"
)

var (
	ErrImageNotFound = errors.New("image not found")

	// ErrImagesDone is used to stop iteration of subsequent images. It should be
	// returned from forEachFunc when either the intended image has been found or
	// the maximum number of images has been iterated.
	ErrImagesDone = errors.New("images done")

	// ErrImagesNoPath is returned whenever an image is found but has no path on disk.
	ErrImagesNoPath = errors.New("no path for image")

	// ErrImagesNoURL is returned whenever an image is found but has no URL.
	ErrImagesNoURL = errors.New("no URL for image")

	ErrImagesUnavailable = errors.New("alert screenshots are unavailable")

	// ErrNoImageForAlert is returned when no image is associated to a given alert.
	ErrNoImageForAlert = errors.New("no image for alert")
)

type ImageContent struct {
	// Name is the unique identifier for the image. Usually this will be an image filename, but is not required to be.
	Name string
	// Content is the raw image data.
	Content []byte
}

type Image struct {
	// URL is the public URL of the image. This URL should not be treated as a trusted source and should not be
	// downloaded directly. RawData should be used to retrieve the image data.
	URL string
	// RawData returns the raw image data. Depending on the provider, this may be a file read, a network request, or
	// unsupported. It's the responsibility of the Provider to ensure that the data is safe to read.
	RawData func(ctx context.Context) (ImageContent, error)
}

func (i Image) HasURL() bool {
	return i.URL != ""
}

type Provider interface {
	// GetImage takes an alert and returns its associated image.
	GetImage(ctx context.Context, alert types.Alert) (*Image, error)
}
