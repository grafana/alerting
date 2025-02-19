package images

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"

	"github.com/grafana/alerting/logging"
	"github.com/grafana/alerting/models"
)

func TestWithStoredImages(t *testing.T) {
	ctx := context.Background()
	alerts := []*types.Alert{{
		Alert: model.Alert{
			Annotations: model.LabelSet{
				models.ImageTokenAnnotation: "test-image-1",
			},
		},
	}, {
		Alert: model.Alert{
			Annotations: model.LabelSet{
				models.ImageTokenAnnotation: "test-image-2",
			},
		},
	}}
	imageProvider := &TokenProvider{
		store: NewFakeTokenStoreFromImages(map[string]*Image{
			"test-image-1": {
				URL: "https://www.example.com/test-image-1.jpg",
			},
			"test-image-2": {
				URL: "https://www.example.com/test-image-2.jpg",
			},
		},
		),
		logger: &logging.FakeLogger{},
	}

	var (
		err error
		i   int
	)

	// should iterate all images
	err = WithStoredImages(ctx, &logging.FakeLogger{}, imageProvider, func(_ int, _ Image) error {
		i++
		return nil
	}, alerts...)
	require.NoError(t, err)
	assert.Equal(t, 2, i)

	// should iterate just the first image
	i = 0
	err = WithStoredImages(ctx, &logging.FakeLogger{}, imageProvider, func(_ int, _ Image) error {
		i++
		return ErrImagesDone
	}, alerts...)
	require.NoError(t, err)
	assert.Equal(t, 1, i)
}
