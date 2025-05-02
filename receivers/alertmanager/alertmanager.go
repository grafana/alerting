package alertmanager

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"

	"github.com/grafana/alerting/images"
	"github.com/grafana/alerting/receivers"
)

// Notifier sends alert notifications to the alert manager
type Notifier struct {
	*receivers.Base
	images   images.Provider
	settings Config
	logger   log.Logger
	sender   receivers.Sender
}

func New(cfg Config, meta receivers.Metadata, images images.Provider, logger log.Logger) *Notifier {
	return &Notifier{
		Base:     receivers.NewBase(meta, logger),
		images:   images,
		settings: cfg,
		logger:   logger,
		sender:   receivers.NewSender(logger),
	}
}

// Notify sends alert notifications to Alertmanager.
func (n *Notifier) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	l := n.GetLogger(ctx)
	level.Debug(l).Log("msg", "sending notification")
	if len(as) == 0 {
		return true, nil
	}

	_ = images.WithStoredImages(ctx, l, n.images,
		func(index int, image images.Image) error {
			// If there is an image for this alert and the image has been uploaded
			// to a public URL then include it as an annotation
			if image.URL != "" {
				as[index].Annotations["image"] = model.LabelValue(image.URL)
			}
			return nil
		}, as...)

	body, err := json.Marshal(as)
	if err != nil {
		return false, err
	}

	var (
		lastErr error
		numErrs int
	)
	for _, u := range n.settings.URLs {
		if _, err := n.sender.SendHTTPRequest(ctx, u, receivers.HTTPCfg{
			User:     n.settings.User,
			Password: n.settings.Password,
			Body:     body,
		}); err != nil {
			level.Warn(n.logger).Log("failed to send to Alertmanager", "error", err, "alertmanager", n.Name, "url", u.String())
			lastErr = err
			numErrs++
		}
	}

	if numErrs == len(n.settings.URLs) {
		// All attempts to send alerts have failed
		level.Warn(l).Log("msg", "all attempts to send to Alertmanager failed")
		return false, fmt.Errorf("failed to send alert to Alertmanager: %w", lastErr)
	}

	return true, nil
}

func (n *Notifier) SendResolved() bool {
	return !n.GetDisableResolveMessage()
}
