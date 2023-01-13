package receivers

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"

	"github.com/grafana/alerting/images"
	"github.com/grafana/alerting/logging"
	"github.com/grafana/alerting/models"
)

type AlertStateType string

const (
	FooterIconURL      = "https://grafana.com/static/assets/img/fav32.png"
	ColorAlertFiring   = "#D63232"
	ColorAlertResolved = "#36a64f"

	AlertStateAlerting AlertStateType = "alerting"
	AlertStateOK       AlertStateType = "ok"

	// ImageStoreTimeout should be used by all callers for calles to `Images`
	ImageStoreTimeout time.Duration = 500 * time.Millisecond
)

type forEachImageFunc func(index int, image images.Image) error

// getImage returns the image for the alert or an error. It returns a nil
// image if the alert does not have an image token or the image does not exist.
func getImage(ctx context.Context, l logging.Logger, imageStore images.ImageStore, alert types.Alert) (*images.Image, error) {
	token := getTokenFromAnnotations(alert.Annotations)
	if token == "" {
		return nil, nil
	}

	ctx, cancelFunc := context.WithTimeout(ctx, ImageStoreTimeout)
	defer cancelFunc()

	img, err := imageStore.GetImage(ctx, token)
	if errors.Is(err, images.ErrImageNotFound) || errors.Is(err, images.ErrImagesUnavailable) {
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
func WithStoredImages(ctx context.Context, l logging.Logger, imageStore images.ImageStore, forEachFunc forEachImageFunc, alerts ...*types.Alert) error {
	for index, alert := range alerts {
		logger := l.New("alert", alert.String())
		img, err := getImage(ctx, logger, imageStore, *alert)
		if err != nil {
			return err
		} else if img != nil {
			if err := forEachFunc(index, *img); err != nil {
				if errors.Is(err, images.ErrImagesDone) {
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
//nolint:gosec, unused, deadcode //TODO yuri. Remove unused and deadcode after migration is done
func OpenImage(path string) (io.ReadCloser, error) {
	fp := filepath.Clean(path)
	_, err := os.Stat(fp)
	if os.IsNotExist(err) || os.IsPermission(err) {
		return nil, images.ErrImageNotFound
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

type ReceiverInitError struct {
	Reason string
	Err    error
	Cfg    NotificationChannelConfig
}

func (e ReceiverInitError) Error() string {
	name := ""
	if e.Cfg.Name != "" {
		name = fmt.Sprintf("%q ", e.Cfg.Name)
	}

	s := fmt.Sprintf("failed to validate receiver %sof type %q: %s", name, e.Cfg.Type, e.Reason)
	if e.Err != nil {
		return fmt.Sprintf("%s: %s", s, e.Err.Error())
	}

	return s
}

func (e ReceiverInitError) Unwrap() error { return e.Err }

func GetAlertStatusColor(status model.AlertStatus) string {
	if status == model.AlertFiring {
		return ColorAlertFiring
	}
	return ColorAlertResolved
}

type NotificationChannel interface {
	notify.Notifier
	notify.ResolvedSender
}

type HTTPCfg struct {
	Body     []byte
	User     string
	Password string
}

// SendHTTPRequest sends an HTTP request.
// Stubbable by tests.
//
//nolint:unused, varcheck
var SendHTTPRequest = func(ctx context.Context, url *url.URL, cfg HTTPCfg, logger logging.Logger) ([]byte, error) {
	var reader io.Reader
	if len(cfg.Body) > 0 {
		reader = bytes.NewReader(cfg.Body)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url.String(), reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	if cfg.User != "" && cfg.Password != "" {
		request.SetBasicAuth(cfg.User, cfg.Password)
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "Grafana")
	netTransport := &http.Transport{
		TLSClientConfig: &tls.Config{
			Renegotiation: tls.RenegotiateFreelyAsClient,
		},
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout: 5 * time.Second,
	}
	netClient := &http.Client{
		Timeout:   time.Second * 30,
		Transport: netTransport,
	}
	resp, err := netClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warn("failed to close response Body", "error", err)
		}
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response Body: %w", err)
	}

	if resp.StatusCode/100 != 2 {
		logger.Warn("HTTP request failed", "url", request.URL.String(), "statusCode", resp.Status, "Body",
			string(respBody))
		return nil, fmt.Errorf("failed to send HTTP request - status code %d", resp.StatusCode)
	}

	logger.Debug("sending HTTP request succeeded", "url", request.URL.String(), "statusCode", resp.Status)
	return respBody, nil
}

func JoinURLPath(base, additionalPath string, logger logging.Logger) string {
	u, err := url.Parse(base)
	if err != nil {
		logger.Debug("failed to parse URL while joining URL", "url", base, "error", err.Error())
		return base
	}

	u.Path = path.Join(u.Path, additionalPath)

	return u.String()
}

// GetBoundary is used for overriding the behaviour for tests
// and set a boundary for multipart Body. DO NOT set this outside tests.
var GetBoundary = func() string {
	return ""
}

// Copied from https://github.com/prometheus/alertmanager/blob/main/notify/util.go, please remove once we're on-par with upstream.
// truncationMarker is the character used to represent a truncation.
const truncationMarker = "…"

// Copied from https://github.com/prometheus/alertmanager/blob/main/notify/util.go, please remove once we're on-par with upstream.
// TruncateInrunes truncates a string to fit the given size in Runes.
func TruncateInRunes(s string, n int) (string, bool) {
	r := []rune(s)
	if len(r) <= n {
		return s, false
	}

	if n <= 3 {
		return string(r[:n]), true
	}

	return string(r[:n-1]) + truncationMarker, true
}

// TruncateInBytes truncates a string to fit the given size in Bytes.
// TODO: This is more advanced than the upstream's TruncateInBytes. We should consider upstreaming this, and removing it from here.
func TruncateInBytes(s string, n int) (string, bool) {
	// First, measure the string the w/o a to-rune conversion.
	if len(s) <= n {
		return s, false
	}

	// The truncationMarker itself is 3 bytes, we can't return any part of the string when it's less than 3.
	if n <= 3 {
		switch n {
		case 3:
			return truncationMarker, true
		default:
			return strings.Repeat(".", n), true
		}
	}

	// Now, to ensure we don't butcher the string we need to remove using runes.
	r := []rune(s)
	truncationTarget := n - 3

	// Next, let's truncate the runes to the lower possible number.
	truncatedRunes := r[:truncationTarget]
	for len(string(truncatedRunes)) > truncationTarget {
		truncatedRunes = r[:len(truncatedRunes)-1]
	}

	return string(truncatedRunes) + truncationMarker, true
}
