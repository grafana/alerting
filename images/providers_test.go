package images

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/prometheus/alertmanager/types"
)

func TestUnavailableProvider_GetImage(t *testing.T) {
	tests := []struct {
		name     string
		alert    types.Alert
		expImage *Image
		expError error
	}{
		{
			name:     "Given alert, expect error",
			alert:    newAlertWithImageURL("https://test"),
			expImage: nil,
			expError: ErrImagesUnavailable,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(tt *testing.T) {
			p := &UnavailableProvider{}
			img, err := p.GetImage(context.Background(), test.alert)
			assert.Equal(tt, test.expImage, img)
			assert.Equal(tt, test.expError, err)
		})
	}
}

func TestURLProvider_GetImage(t *testing.T) {
	tests := []struct {
		name     string
		alert    types.Alert
		expImage *Image
	}{
		{
			name:     "Given alert without image URI, expect nil",
			alert:    types.Alert{},
			expImage: nil,
		},
		{
			name:     "Given alert with image URI, expect image",
			alert:    newAlertWithImageURL("https://test"),
			expImage: &Image{URL: "https://test"},
		},
		{
			name:     "Given alert with image token, expect nil", // Token is irrelevant for this provider.
			alert:    newAlertWithImageToken("test-token"),
			expImage: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(tt *testing.T) {
			p := &URLProvider{}
			img, err := p.GetImage(context.Background(), test.alert)
			assert.NoError(tt, err)
			if test.expImage == nil {
				assert.Nil(tt, img)
				return
			}
			assert.Equal(tt, test.expImage.URL, img.URL)
			_, err = img.RawData(context.Background())
			assert.Equal(tt, ErrImageUploadNotSupported, err)
		})
	}
}
