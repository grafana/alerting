package channels

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
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
	"gopkg.in/yaml.v2"

	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/services/ngalert/models"
	"github.com/grafana/grafana/pkg/util"

	"github.com/grafana/grafana/pkg/components/simplejson"
)

const (
	FooterIconURL      = "https://grafana.com/assets/img/fav32.png"
	ColorAlertFiring   = "#D63232"
	ColorAlertResolved = "#36a64f"

	// ImageStoreTimeout should be used by all callers for calles to `Images`
	ImageStoreTimeout time.Duration = 500 * time.Millisecond
)

var (
	// Provides current time. Can be overwritten in tests.
	timeNow = time.Now

	// ErrImagesDone is used to stop iteration of subsequent images. It should be
	// returned from forEachFunc when either the intended image has been found or
	// the maximum number of images has been iterated.
	ErrImagesDone        = errors.New("images done")
	ErrImagesUnavailable = errors.New("alert screenshots are unavailable")
)

type forEachImageFunc func(index int, image models.Image) error

// getImage returns the image for the alert or an error. It returns a nil
// image if the alert does not have an image token or the image does not exist.
func getImage(ctx context.Context, l log.Logger, imageStore ImageStore, alert types.Alert) (*models.Image, error) {
	token := getTokenFromAnnotations(alert.Annotations)
	if token == "" {
		return nil, nil
	}

	ctx, cancelFunc := context.WithTimeout(ctx, ImageStoreTimeout)
	defer cancelFunc()

	img, err := imageStore.GetImage(ctx, token)
	if errors.Is(err, models.ErrImageNotFound) || errors.Is(err, ErrImagesUnavailable) {
		return nil, nil
	} else if err != nil {
		l.Warn("failed to get image with token", "token", token, "err", err)
		return nil, err
	} else {
		return img, nil
	}
}

// withStoredImages retrieves the image for each alert and then calls forEachFunc
// with the index of the alert and the retrieved image struct. If the alert does
// not have an image token, or the image does not exist then forEachFunc will not be
// called for that alert. If forEachFunc returns an error, withStoredImages will return
// the error and not iterate the remaining alerts. A forEachFunc can return ErrImagesDone
// to stop the iteration of remaining alerts if the intended image or maximum number of
// images have been found.
func withStoredImages(ctx context.Context, l log.Logger, imageStore ImageStore, forEachFunc forEachImageFunc, alerts ...*types.Alert) error {
	for index, alert := range alerts {
		img, err := getImage(ctx, l, imageStore, *alert)
		if err != nil {
			return err
		} else if img != nil {
			if err := forEachFunc(index, *img); err != nil {
				if errors.Is(err, ErrImagesDone) {
					return nil
				}
				return err
			}
		}
	}
	return nil
}

// The path argument here comes from reading internal image storage, not user
// input, so we ignore the security check here.
//
//nolint:gosec
func openImage(path string) (io.ReadCloser, error) {
	fp := filepath.Clean(path)
	_, err := os.Stat(fp)
	if os.IsNotExist(err) || os.IsPermission(err) {
		return nil, models.ErrImageNotFound
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

type UnavailableImageStore struct{}

// Get returns the image with the corresponding token, or ErrImageNotFound.
func (u *UnavailableImageStore) GetImage(ctx context.Context, token string) (*models.Image, error) {
	return nil, ErrImagesUnavailable
}

type receiverInitError struct {
	Reason string
	Err    error
	Cfg    NotificationChannelConfig
}

func (e receiverInitError) Error() string {
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

func (e receiverInitError) Unwrap() error { return e.Err }

func getAlertStatusColor(status model.AlertStatus) string {
	if status == model.AlertFiring {
		return ColorAlertFiring
	}
	return ColorAlertResolved
}

type NotificationChannel interface {
	notify.Notifier
	notify.ResolvedSender
}
type NotificationChannelConfig struct {
	OrgID                 int64             // only used internally
	UID                   string            `json:"uid"`
	Name                  string            `json:"name"`
	Type                  string            `json:"type"`
	DisableResolveMessage bool              `json:"disableResolveMessage"`
	Settings              *simplejson.Json  `json:"settings"`
	SecureSettings        map[string][]byte `json:"secureSettings"`
}

func (c NotificationChannelConfig) unmarshalSettings(v interface{}) error {
	ser, err := c.Settings.Encode()
	if err != nil {
		return err
	}
	err = json.Unmarshal(ser, v)
	if err != nil {
		return err
	}
	return nil
}

type httpCfg struct {
	body     []byte
	user     string
	password string
}

// sendHTTPRequest sends an HTTP request.
// Stubbable by tests.
var sendHTTPRequest = func(ctx context.Context, url *url.URL, cfg httpCfg, logger log.Logger) ([]byte, error) {
	var reader io.Reader
	if len(cfg.body) > 0 {
		reader = bytes.NewReader(cfg.body)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url.String(), reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	if cfg.user != "" && cfg.password != "" {
		request.Header.Set("Authorization", util.GetBasicAuthHeader(cfg.user, cfg.password))
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
			logger.Warn("failed to close response body", "err", err)
		}
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode/100 != 2 {
		logger.Warn("HTTP request failed", "url", request.URL.String(), "statusCode", resp.Status, "body",
			string(respBody))
		return nil, fmt.Errorf("failed to send HTTP request - status code %d", resp.StatusCode)
	}

	logger.Debug("sending HTTP request succeeded", "url", request.URL.String(), "statusCode", resp.Status)
	return respBody, nil
}

func joinUrlPath(base, additionalPath string, logger log.Logger) string {
	u, err := url.Parse(base)
	if err != nil {
		logger.Debug("failed to parse URL while joining URL", "url", base, "err", err.Error())
		return base
	}

	u.Path = path.Join(u.Path, additionalPath)

	return u.String()
}

// GetBoundary is used for overriding the behaviour for tests
// and set a boundary for multipart body. DO NOT set this outside tests.
var GetBoundary = func() string {
	return ""
}

type CommaSeparatedStrings []string

func (r *CommaSeparatedStrings) UnmarshalJSON(b []byte) error {
	var str string
	if err := json.Unmarshal(b, &str); err != nil {
		return err
	}
	if len(str) > 0 {
		res := CommaSeparatedStrings(splitCommaDelimitedString(str))
		*r = res
	}
	return nil
}

func (r *CommaSeparatedStrings) MarshalJSON() ([]byte, error) {
	if r == nil {
		return nil, nil
	}
	str := strings.Join(*r, ",")
	return json.Marshal(str)
}

func (r *CommaSeparatedStrings) UnmarshalYAML(b []byte) error {
	var str string
	if err := yaml.Unmarshal(b, &str); err != nil {
		return err
	}
	if len(str) > 0 {
		res := CommaSeparatedStrings(splitCommaDelimitedString(str))
		*r = res
	}
	return nil
}

func (r *CommaSeparatedStrings) MarshalYAML() ([]byte, error) {
	if r == nil {
		return nil, nil
	}
	str := strings.Join(*r, ",")
	return yaml.Marshal(str)
}

func splitCommaDelimitedString(str string) []string {
	split := strings.Split(str, ",")
	res := make([]string, 0, len(split))
	for _, s := range split {
		if tr := strings.TrimSpace(s); tr != "" {
			res = append(res, tr)
		}
	}
	return res
}
