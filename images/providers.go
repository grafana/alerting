package images

import (
	"context"
	"errors"

	"github.com/grafana/alerting/models"
	"github.com/prometheus/alertmanager/types"
)

// ErrImageUploadNotSupported is returned when image uploading is not supported.
var ErrImageUploadNotSupported = errors.New("image upload is not supported")

type UnavailableProvider struct{}

var _ Provider = (*UnavailableProvider)(nil)

func (u *UnavailableProvider) GetImage(_ context.Context, _ types.Alert) (*Image, error) {
	return nil, ErrImagesUnavailable
}

// URLProvider is a provider that stores a direct reference to an image's public URL in an alert's annotations.
// The URL is not validated against a database record, so retrieving raw image data is blocked in an attempt
// to prevent malicious access to untrusted URLs.
type URLProvider struct{}

var _ Provider = (*URLProvider)(nil)

// GetImage returns the image associated with a given alert.
// The URL should be treated as untrusted and notifiers should pass the URL directly without attempting to download
// the image data.
func (u *URLProvider) GetImage(_ context.Context, alert types.Alert) (*Image, error) {
	url := GetImageURL(alert)
	if url == "" {
		return nil, nil
	}

	return &Image{
		URL: url,
		RawData: func(_ context.Context) (ImageContent, error) {
			// Raw images are not available for alerts provided directly by annotations as the image data is non-local.
			// While it might be possible to download the image data, it's generally not safe to do so as the URL is
			// not guaranteed to be trusted.
			return ImageContent{}, ErrImageUploadNotSupported
		},
	}, nil
}

// GetImageURL is a helper function to retrieve the image url from the alert annotations.
func GetImageURL(alert types.Alert) string {
	return string(alert.Annotations[models.ImageURLAnnotation])
}
