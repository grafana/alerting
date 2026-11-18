package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/go-kit/log/level"
	"github.com/prometheus/alertmanager/types"

	"github.com/go-kit/log"

	"github.com/grafana/alerting/models"
	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

const (
	// alertRulesListPath is the path to the page listing all alert rules.
	alertRulesListPath = "/alerting/list"
	// alertRuleViewPathTemplate is the path to the page showing a single Grafana-managed alert rule.
	alertRuleViewPathTemplate = "/alerting/grafana/%s/view"
)

// Notifier is responsible for sending alert notifications to ding ding.
type Notifier struct {
	*receivers.Base
	ns       receivers.WebhookSender
	tmpl     *templates.Template
	settings Config
}

func New(cfg Config, meta receivers.Metadata, template *templates.Template, sender receivers.WebhookSender, logger log.Logger) *Notifier {
	return &Notifier{
		Base:     receivers.NewBase(meta, logger),
		ns:       sender,
		tmpl:     template,
		settings: cfg,
	}
}

// Notify sends the alert notification to dingding.
func (dd *Notifier) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	l := dd.GetLogger(ctx)
	level.Info(l).Log("msg", "sending dingding")

	dingDingURL := buildDingDingURL(dd.tmpl.ExternalURL, alertRuleUID(as, l), l)

	var tmplErr error
	tmpl, data := templates.TmplText(ctx, dd.tmpl, as, l, &tmplErr)

	receivers.ApplyExtraData(ctx, data.Alerts)

	message := tmpl(dd.settings.Message)
	title := tmpl(dd.settings.Title)

	msgType := tmpl(dd.settings.MessageType)
	b, err := buildBody(dingDingURL, msgType, title, message)
	if err != nil {
		return false, err
	}

	if tmplErr != nil {
		level.Warn(l).Log("msg", "failed to template DingDing message", "err", tmplErr.Error())
		tmplErr = nil
	}

	u := tmpl(dd.settings.URL)
	if tmplErr != nil {
		level.Warn(l).Log("msg", "failed to template DingDing URL", "err", tmplErr.Error(), "fallback", dd.settings.URL)
		u = dd.settings.URL
	}

	cmd := &receivers.SendWebhookSettings{URL: u, Body: b}

	if err := dd.ns.SendWebhook(ctx, l, cmd); err != nil {
		return false, fmt.Errorf("send notification to dingding: %w", err)
	}

	return true, nil
}

func (dd *Notifier) SendResolved() bool {
	return !dd.GetDisableResolveMessage()
}

// alertRuleUID returns the UID of the alert rule that produced the given alerts. A single notification can group
// alerts coming from different rules, or alerts that are not tied to a Grafana rule at all, in which case an empty
// string is returned so that the caller can fall back to a link that is guaranteed to be valid.
func alertRuleUID(alerts []*types.Alert, l log.Logger) string {
	var uid string
	for _, alert := range alerts {
		if alert == nil {
			continue
		}
		current := string(alert.Labels[models.RuleUIDLabel])
		switch {
		case current == "":
			return ""
		case uid == "":
			uid = current
		case uid != current:
			level.Debug(l).Log("msg", "alerts in notification belong to multiple alert rules", "expected_rule_uid", uid, "rule_uid", current)
			return ""
		}
	}
	return uid
}

func buildDingDingURL(externalURL *url.URL, ruleUID string, l log.Logger) string {
	target := alertRulesListPath
	if ruleUID != "" {
		target = fmt.Sprintf(alertRuleViewPathTemplate, ruleUID)
	}

	q := url.Values{
		"pc_slide": {"false"},
		"url":      {receivers.JoinURLPath(externalURL.String(), target, l)},
	}

	// Use special link to auto open the message url outside Dingding
	// Refer: https://open-doc.dingtalk.com/docs/doc.htm?treeId=385&articleId=104972&docType=1#s9
	return "dingtalk://dingtalkclient/page/link?" + q.Encode()
}

func buildBody(url string, msgType string, title string, msg string) (string, error) {
	var bodyMsg map[string]interface{}
	if msgType == "actionCard" {
		bodyMsg = map[string]interface{}{
			"msgtype": "actionCard",
			"actionCard": map[string]string{
				"text":        msg,
				"title":       title,
				"singleTitle": "More",
				"singleURL":   url,
			},
		}
	} else {
		bodyMsg = map[string]interface{}{
			"msgtype": "link",
			"link": map[string]string{
				"text":       msg,
				"title":      title,
				"messageUrl": url,
			},
		}
	}
	body, err := json.Marshal(bodyMsg)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
