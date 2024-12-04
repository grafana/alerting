package oncall_ng

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net/url"

	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"

	"github.com/grafana/alerting/images"
	"github.com/grafana/alerting/logging"
	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

// Notifier is responsible for sending
// alert notifications as webhooks.
type Notifier struct {
	*receivers.Base
	log      logging.Logger
	ns       receivers.WebhookSender
	images   images.Provider
	tmpl     *templates.Template
	orgID    int64
	settings Config
}

// New is the constructor for
// the WebHook notifier.
func New(cfg Config, meta receivers.Metadata, template *templates.Template, sender receivers.WebhookSender, images images.Provider, logger logging.Logger, orgID int64) *Notifier {
	return &Notifier{
		Base:     receivers.NewBase(meta),
		orgID:    orgID,
		log:      logger,
		ns:       sender,
		images:   images,
		tmpl:     template,
		settings: cfg,
	}
}

type chatOpsConfig struct {
	SlackChannelId    string `json:"slackChannelId"`
	MsTeamsChannelId  string `json:"msTeamsChannelId"`
	TelegramChannelId string `json:"telegramChannelId"`
}

type routingConfig struct {
	EscalationChainId string         `json:"escalationChainId"`
	ReceiveName       string         `json:"receiverName"`
	ReceiverUID       string         `json:"receiverUID"`
	TeamName          string         `json:"teamName,omitempty"`
	ChatOps           *chatOpsConfig `json:"chatOps,omitempty"`
}

// oncallMessage defines the JSON object send to Grafana on-call.
type oncallMessage struct {
	*templates.ExtendedData

	// The protocol version.
	Version         string        `json:"version"`
	GroupKey        string        `json:"groupKey"`
	OrgID           int64         `json:"orgId"`
	Title           string        `json:"title"`
	State           string        `json:"state"`
	Message         string        `json:"message"`
	TruncatedAlerts uint64        `json:"truncatedAlerts"`
	RoutingConfig   routingConfig `json:"routingConfig"`
}

// Notify implements the Notifier interface.
func (n *Notifier) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	groupKey, err := notify.ExtractGroupKey(ctx)
	if err != nil {
		return false, err
	}

	var numFiring, numResolved uint64
	for _, a := range as {
		if a.Resolved() {
			numResolved++
		} else {
			numFiring++
		}
	}

	var tmplErr error
	tmpl, data := templates.TmplText(ctx, n.tmpl, as, n.log, &tmplErr)

	// Augment our Alert data with ImageURLs if available.
	_ = images.WithStoredImages(ctx, n.log, n.images,
		func(index int, image images.Image) error {
			if len(image.URL) != 0 {
				data.Alerts[index].ImageURL = image.URL
			}
			return nil
		},
		as...)

	var chatOps *chatOpsConfig
	if n.settings.MsTeamsChannelId != "" || n.settings.TelegramChannelId != "" || n.settings.SlackChannelID != "" {
		chatOps = &chatOpsConfig{
			SlackChannelId:    n.settings.SlackChannelID,
			MsTeamsChannelId:  n.settings.MsTeamsChannelId,
			TelegramChannelId: n.settings.TelegramChannelId,
		}
	}

	msg := &oncallMessage{
		Version:      "1",
		ExtendedData: data,
		GroupKey:     groupKey.String(),
		OrgID:        n.orgID,
		Title:        tmpl(n.settings.Title),
		Message:      tmpl(n.settings.Message),
		RoutingConfig: routingConfig{
			EscalationChainId: n.settings.EscalationChainID,
			ReceiveName:       n.Name,
			ReceiverUID:       base64.RawURLEncoding.EncodeToString([]byte(n.Name)),
			TeamName:          n.settings.TeamName,
			ChatOps:           chatOps,
		},
	}

	if types.Alerts(as...).Status() == model.AlertFiring {
		msg.State = string(receivers.AlertStateAlerting)
	} else {
		msg.State = string(receivers.AlertStateOK)
	}

	if tmplErr != nil {
		n.log.Warn("failed to template oncall message", "error", tmplErr.Error())
		tmplErr = nil
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return false, err
	}

	headers := make(map[string]string)
	if n.settings.AuthorizationScheme != "" && n.settings.AuthorizationCredentials != "" {
		headers["Authorization"] = fmt.Sprintf("%s %s", n.settings.AuthorizationScheme, n.settings.AuthorizationCredentials)
	}

	u, _ := url.Parse(n.settings.APIURL)
	key := n.UID
	if key == "" { // IF UID is empty, fallback to name
		f := fnv.New64()
		_, _ = f.Write([]byte(n.Name))
		key = fmt.Sprintf("%x", f.Sum(nil))
	}
	u.Path = fmt.Sprintf("/integrations/v1/adaptive_grafana_alerting/%s/", key)

	cmd := &receivers.SendWebhookSettings{
		URL:        u.String(),
		Body:       string(body),
		HTTPMethod: "POST",
		HTTPHeader: headers,
	}

	if err := n.ns.SendWebhook(ctx, cmd); err != nil {
		return false, err
	}

	return true, nil
}

func (n *Notifier) SendResolved() bool {
	return true
}