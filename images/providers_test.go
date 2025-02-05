package images

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/grafana/alerting/logging"
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

func TestTokenProvider_GetImage(t *testing.T) {
	tests := []struct {
		name         string
		storedImages map[string]*Image
		alert        types.Alert
		expImage     *Image
	}{
		{
			name: "Given alert without image token, expect nil",
			storedImages: map[string]*Image{
				"test-token": {URL: "https://test"},
			},
			alert:    types.Alert{},
			expImage: nil,
		},
		{
			name: "Given alert with image token, expect image",
			storedImages: map[string]*Image{
				"test-token": {URL: "https://test"},
			},
			alert:    newAlertWithImageToken("test-token"),
			expImage: &Image{URL: "https://test"},
		},
		{
			name: "Given alert with invalid image token, expect nil",
			storedImages: map[string]*Image{
				"test-token": {URL: "https://test"},
			},
			alert:    newAlertWithImageToken("invalid"),
			expImage: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(tt *testing.T) {
			p := NewTokenProvider(FakeTokenStore{
				Images: test.storedImages,
			}, &logging.FakeLogger{})
			img, err := p.GetImage(context.Background(), test.alert)
			assert.NoError(tt, err)
			if test.expImage == nil {
				assert.Nil(tt, img)
				return
			}
			assert.Equal(tt, test.expImage.URL, img.URL)
		})
	}
}

type FakeTokenStore struct {
	Images map[string]*Image
}

var _ TokenStore = (*FakeTokenStore)(nil)

func (f FakeTokenStore) GetImage(ctx context.Context, token string) (*Image, error) {
	return f.Images[token], nil
}
