package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"

	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

const (
	matrixSendPathFormat = "/_matrix/client/v3/rooms/%s/send/m.room.message/%s"
	matrixFormatHTML     = "org.matrix.custom.html"
	// maxFormattedBodyBytes keeps the event under the default Matrix 64 KiB cap
	// with headroom for the JSON envelope and the plaintext body.
	maxFormattedBodyBytes = 48 * 1024
)

type matrixMessage struct {
	MsgType       string `json:"msgtype"`
	Body          string `json:"body"`
	Format        string `json:"format,omitempty"`
	FormattedBody string `json:"formatted_body,omitempty"`
}

type matrixError struct {
	ErrCode string `json:"errcode"`
	Error   string `json:"error"`
}

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

func (n *Notifier) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	l := n.GetLogger(ctx)
	var tmplErr error
	tmpl, _ := templates.TmplText(ctx, n.tmpl, as, l, &tmplErr)

	title := tmpl(n.settings.Title)
	body := tmpl(n.settings.Message)
	if tmplErr != nil {
		level.Warn(l).Log("msg", "failed to template Matrix message", "err", tmplErr.Error())
	}

	msg := matrixMessage{
		MsgType:       n.settings.MessageType,
		Body:          joinTitleBody(title, body),
		Format:        matrixFormatHTML,
		FormattedBody: renderHTML(title, as),
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return false, fmt.Errorf("failed to marshal Matrix message: %w", err)
	}

	cmd := &receivers.SendWebhookSettings{
		URL:         n.buildSendURL(),
		HTTPMethod:  "PUT",
		ContentType: "application/json",
		Body:        string(payload),
		HTTPHeader: map[string]string{
			"Authorization": "Bearer " + n.settings.AccessToken,
		},
		Validation: validateResponse,
	}

	if err := n.ns.SendWebhook(ctx, l, cmd); err != nil {
		return false, fmt.Errorf("send notification to Matrix: %w", err)
	}
	return true, nil
}

func (n *Notifier) SendResolved() bool {
	return !n.GetDisableResolveMessage()
}

func (n *Notifier) buildSendURL() string {
	txnID := fmt.Sprintf("grafana-%d", time.Now().UnixNano())
	return n.settings.HomeserverURL + fmt.Sprintf(matrixSendPathFormat, url.PathEscape(n.settings.RoomID), url.PathEscape(txnID))
}

func joinTitleBody(title, body string) string {
	switch {
	case title == "" && body == "":
		return ""
	case title == "":
		return body
	case body == "":
		return title
	default:
		return title + "\n\n" + body
	}
}

func renderHTML(title string, alerts []*types.Alert) string {
	var b strings.Builder
	if title != "" {
		b.WriteString("<h4>")
		b.WriteString(html.EscapeString(title))
		b.WriteString("</h4>")
	}

	firing, resolved := splitByStatus(alerts)
	if len(firing) > 0 {
		b.WriteString("<p><strong>Firing</strong></p>")
		writeAlertList(&b, firing)
	}
	if len(resolved) > 0 {
		b.WriteString("<p><strong>Resolved</strong></p>")
		writeAlertList(&b, resolved)
	}

	return truncateUTF8(b.String(), maxFormattedBodyBytes)
}

// truncateUTF8 returns s trimmed to at most max bytes without splitting a
// multi-byte rune. If trimming is needed, the result ends with an ellipsis.
func truncateUTF8(s string, max int) string {
	if len(s) <= max {
		return s
	}
	const marker = "…"
	budget := max - len(marker)
	if budget <= 0 {
		return ""
	}
	end := 0
	for end < len(s) {
		_, size := utf8.DecodeRuneInString(s[end:])
		if end+size > budget {
			break
		}
		end += size
	}
	return s[:end] + marker
}

func splitByStatus(alerts []*types.Alert) (firing, resolved []*types.Alert) {
	for _, a := range alerts {
		if a.Status() == model.AlertResolved {
			resolved = append(resolved, a)
		} else {
			firing = append(firing, a)
		}
	}
	return firing, resolved
}

func writeAlertList(b *strings.Builder, alerts []*types.Alert) {
	b.WriteString("<ul>")
	for _, a := range alerts {
		b.WriteString("<li>")
		b.WriteString(html.EscapeString(a.Name()))

		if summary, ok := a.Annotations["summary"]; ok && summary != "" {
			b.WriteString(": ")
			b.WriteString(html.EscapeString(string(summary)))
		} else if desc, ok := a.Annotations["description"]; ok && desc != "" {
			b.WriteString(": ")
			b.WriteString(html.EscapeString(string(desc)))
		}

		if labels := formatLabels(a.Labels); labels != "" {
			b.WriteString(" <code>")
			b.WriteString(html.EscapeString(labels))
			b.WriteString("</code>")
		}

		if a.GeneratorURL != "" {
			b.WriteString(` [<a href="`)
			b.WriteString(html.EscapeString(a.GeneratorURL))
			b.WriteString(`">source</a>]`)
		}
		b.WriteString("</li>")
	}
	b.WriteString("</ul>")
}

func formatLabels(ls model.LabelSet) string {
	keys := make([]string, 0, len(ls))
	for k := range ls {
		if k == model.AlertNameLabel {
			continue
		}
		keys = append(keys, string(k))
	}
	if len(keys) == 0 {
		return ""
	}
	sort.Strings(keys)

	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(string(ls[model.LabelName(k)]))
	}
	return b.String()
}

func validateResponse(body []byte, statusCode int) error {
	if statusCode/100 == 2 {
		return nil
	}
	var merr matrixError
	if err := json.Unmarshal(body, &merr); err == nil && merr.ErrCode != "" {
		return fmt.Errorf("matrix API responded with %d %s: %s", statusCode, merr.ErrCode, merr.Error)
	}
	return fmt.Errorf("unexpected status %d from matrix", statusCode)
}
