package images

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"

	"github.com/grafana/alerting/logging"
	"github.com/grafana/alerting/models"
)

const (
	// ImageStoreTimeout should be used by all callers for calles to `Images`
	ImageStoreTimeout time.Duration = 500 * time.Millisecond
)

type forEachImageFunc func(index int, image Image) error

// getImage returns the image for the alert or an error. It returns a nil
// image if the alert does not have an image token or the image does not exist.
func getImage(ctx context.Context, l logging.Logger, imageStore ImageStore, alert types.Alert) (*Image, error) {
	token := getTokenFromAnnotations(alert.Annotations)
	if token == "" {
		return nil, nil
	}

	ctx, cancelFunc := context.WithTimeout(ctx, ImageStoreTimeout)
	defer cancelFunc()

	img, err := imageStore.GetImage(ctx, token)
	if errors.Is(err, ErrImageNotFound) || errors.Is(err, ErrImagesUnavailable) {
		return nil, nil
	} else if err != nil {
		l.Warn("failed to get image with token", "token", token, "error", err)
		return nil, err
	} else {
		return img, nil
	}
}

// WithStoredImages retrieves the image for each alert and then calls forEachFunc
// with the index of the alert and the retrieved image struct. If the alert does
// not have an image token, or the image does not exist then forEachFunc will not be
// called for that alert. If forEachFunc returns an error, withStoredImages will return
// the error and not iterate the remaining alerts. A forEachFunc can return ErrImagesDone
// to stop the iteration of remaining alerts if the intended image or maximum number of
// images have been found.
func WithStoredImages(ctx context.Context, l logging.Logger, imageStore ImageStore, forEachFunc forEachImageFunc, alerts ...*types.Alert) error {
	for index, alert := range alerts {
		logger := l.New("alert", alert.String())
		img, err := getImage(ctx, logger, imageStore, *alert)
		if err != nil {
			return err
		} else if img != nil {
			if err := forEachFunc(index, *img); err != nil {
				if errors.Is(err, ErrImagesDone) {
					return nil
				}
				logger.Error("Failed to attach image to notification", "error", err)
				return err
			}
		}
	}
	return nil
}

// The path argument here comes from reading internal image storage, not User
// input, so we ignore the security check here.
//
//nolint:gosec
func OpenImage(path string) (io.ReadCloser, error) {
	fp := filepath.Clean(path)
	_, err := os.Stat(fp)
	if os.IsNotExist(err) || os.IsPermission(err) {
		return nil, ErrImageNotFound
	}

	f, err := os.Open(fp)
	if err != nil {
		return nil, err
	}

	return f, nil
}

func getTokenFromAnnotations(annotations model.LabelSet) string {
	if value, ok := annotations[models.ImageTokenAnnotation]; ok {
		return string(value)
	}
	return ""
}
