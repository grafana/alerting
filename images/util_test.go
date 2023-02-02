package images

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	imageStore := &FakeImageStore{Images: []*Image{{
		Token:     "test-image-1",
		URL:       "https://www.example.com/test-image-1.jpg",
		CreatedAt: time.Now().UTC(),
	}, {
		Token:     "test-image-2",
		URL:       "https://www.example.com/test-image-2.jpg",
		CreatedAt: time.Now().UTC(),
	}}}

	var (
		err error
		i   int
	)

	// should iterate all images
	err = WithStoredImages(ctx, &logging.FakeLogger{}, imageStore, func(index int, image Image) error {
		i++
		return nil
	}, alerts...)
	require.NoError(t, err)
	assert.Equal(t, 2, i)

	// should iterate just the first image
	i = 0
	err = WithStoredImages(ctx, &logging.FakeLogger{}, imageStore, func(index int, image Image) error {
		i++
		return ErrImagesDone
	}, alerts...)
	require.NoError(t, err)
	assert.Equal(t, 1, i)
}
