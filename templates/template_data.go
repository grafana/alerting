package templates

import (
	"context"
	"encoding/json"
	"github.com/grafana/alerting/receivers" // LOGZ.IO GRAFANA CHANGE :: DEV-45466: complete fix switch to account query param functionality
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"

	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"

	"github.com/grafana/alerting/models"
)

type Template = template.Template
type KV = template.KV
type Data = template.Data

var FromGlobs = template.FromGlobs

type ExtendedAlert struct {
	Status        string             `json:"status"`
	Labels        KV                 `json:"labels"`
	Annotations   KV                 `json:"annotations"`
	StartsAt      time.Time          `json:"startsAt"`
	EndsAt        time.Time          `json:"endsAt"`
	GeneratorURL  string             `json:"generatorURL"`
	Fingerprint   string             `json:"fingerprint"`
	SilenceURL    string             `json:"silenceURL"`
	DashboardURL  string             `json:"dashboardURL"`
	PanelURL      string             `json:"panelURL"`
	Values        map[string]float64 `json:"values"`
	ValueString   string             `json:"valueString"` // TODO: Remove in Grafana 10
	ImageURL      string             `json:"imageURL,omitempty"`
	EmbeddedImage string             `json:"embeddedImage,omitempty"`
}

type ExtendedAlerts []ExtendedAlert

type ExtendedData struct {
	Receiver string         `json:"receiver"`
	Status   string         `json:"status"`
	Alerts   ExtendedAlerts `json:"alerts"`

	GroupLabels       KV `json:"groupLabels"`
	CommonLabels      KV `json:"commonLabels"`
	CommonAnnotations KV `json:"commonAnnotations"`

	ExternalURL string `json:"externalURL"`
}

func removePrivateItems(kv template.KV) template.KV {
	for key := range kv {
		if strings.HasPrefix(key, "__") && strings.HasSuffix(key, "__") {
			kv = kv.Remove([]string{key})
		}
	}
	return kv
}

func extendAlert(alert template.Alert, externalURL string, logger log.Logger) *ExtendedAlert {
	// LOGZ.IO GRAFANA CHANGE :: DEV-45466: complete fix switch to account query param functionality
	accountId := alert.Annotations[models.LogzioAccountIdAnnotation]
	var generatorUrl string
	parsedGeneratorUrl, err := receivers.ParseLogzioAppPath(alert.GeneratorURL)
	if err == nil {
		parsedGeneratorUrl = receivers.AppendSwitchToAccountQueryParam(parsedGeneratorUrl, accountId)
		generatorUrl = receivers.ToLogzioAppPath(parsedGeneratorUrl.String())
	} else {
		generatorUrl = alert.GeneratorURL
	}
	// LOGZ.IO GRAFANA CHANGE :: end
	// remove "private" annotations & labels so they don't show up in the template
	extended := &ExtendedAlert{
		Status:       alert.Status,
		Labels:       removePrivateItems(alert.Labels),
		Annotations:  removePrivateItems(alert.Annotations),
		StartsAt:     alert.StartsAt,
		EndsAt:       alert.EndsAt,
		GeneratorURL: generatorUrl, // LOGZ.IO GRAFANA CHANGE :: DEV-45466: complete fix switch to account query param functionality
		Fingerprint:  alert.Fingerprint,
	}
	//generatorURL, err := url.Parse(extended.GeneratorURL)	// LOGZ.IO GRAFANA CHANGE :: DEV-45707: remove org id query param from notification urls
	// fill in some grafana-specific urls
	if len(externalURL) == 0 {
		return extended
	}
	u, err := url.Parse(externalURL)
	if err != nil {
		level.Debug(logger).Log("msg", "failed to parse external URL while extending template data", "url", externalURL, "error", err.Error())
		return extended
	}
	// LOGZ.IO GRAFANA CHANGE :: DEV-45707: remove org id query param from notification urls
	/*orgID := alert.Annotations[models.OrgIDAnnotation]
	if len(orgID) > 0 {
		extended.GeneratorURL = setOrgIDQueryParam(generatorURL, orgID)
	}*/
	// LOGZ.IO GRAFANA CHANGE :: end

	externalPath := u.Path

	if err != nil {
		level.Debug(logger).Log("msg", "failed to parse generator URL while extending template data", "url", extended.GeneratorURL, "error", err.Error())
		return extended
	}

	dashboardUID := alert.Annotations[models.DashboardUIDAnnotation]
	if len(dashboardUID) > 0 {
		u.Path = path.Join(externalPath, "/d/", dashboardUID)
		extended.DashboardURL = receivers.ToLogzioAppPath(receivers.AppendSwitchToAccountQueryParam(u, accountId).String()) // LOGZ.IO GRAFANA CHANGE :: DEV-45466: complete fix switch to account query param functionality
		panelID := alert.Annotations[models.PanelIDAnnotation]
		if len(panelID) > 0 {
			u.RawQuery = "viewPanel=" + panelID
			extended.PanelURL = receivers.ToLogzioAppPath(receivers.AppendSwitchToAccountQueryParam(u, accountId).String()) // LOGZ.IO GRAFANA CHANGE :: DEV-45466: complete fix switch to account query param functionality
		}
		//dashboardURL, err := url.Parse(extended.DashboardURL)	// LOGZ.IO GRAFANA CHANGE :: DEV-45707: remove org id query param from notification urls
		if err != nil {
			level.Debug(logger).Log("msg", "failed to parse dashboard URL while extending template data", "url", extended.DashboardURL, "error", err.Error())
			return extended
		}
		/* LOGZ.IO GRAFANA CHANGE :: DEV-45707: remove org id query param from notification urls
		if len(orgID) > 0 {
			extended.DashboardURL = setOrgIDQueryParam(dashboardURL, orgID)
			extended.PanelURL = setOrgIDQueryParam(u, orgID)
		}
			LOGZ.IO GRAFANA CHANGE :: end
		*/
	}

	if alert.Annotations != nil {
		if s, ok := alert.Annotations[models.ValuesAnnotation]; ok {
			if err := json.Unmarshal([]byte(s), &extended.Values); err != nil {
				level.Warn(logger).Log("msg", "failed to unmarshal values annotation", "error", err.Error())
			}
		}

		// TODO: Remove in Grafana 10
		extended.ValueString = alert.Annotations[models.ValueStringAnnotation]
	}

	matchers := make([]string, 0)
	for key, value := range alert.Labels {
		if !(strings.HasPrefix(key, "__") && strings.HasSuffix(key, "__")) {
			matchers = append(matchers, key+"="+value)
		}
	}
	sort.Strings(matchers)
	u.Path = path.Join(externalPath, "/alerting/silence/new")

	query := make(url.Values)
	query.Add("alertmanager", "grafana")
	for _, matcher := range matchers {
		query.Add("matcher", matcher)
	}

	u.RawQuery = query.Encode()
	u = receivers.AppendSwitchToAccountQueryParam(u, accountId) // LOGZ.IO GRAFANA CHANGE :: DEV-45466: complete fix switch to account query param functionality
	/* LOGZ.IO GRAFANA CHANGE :: DEV-45707: remove org id query param from notification urls
	if len(orgID) > 0 {
		extended.SilenceURL = setOrgIDQueryParam(u, orgID)
	} else {
		extended.SilenceURL = u.String()
	}
	*/
	extended.SilenceURL = receivers.ToLogzioAppPath(u.String()) // LOGZ.IO GRAFANA CHANGE :: DEV-45466: complete fix switch to account query param functionality
	return extended
}

/* LOGZ.IO GRAFANA CHANGE :: DEV-45707: remove org id query param from notification urls
func setOrgIDQueryParam(url *url.URL, orgID string) string {
	q := url.Query()
	q.Set("orgId", orgID)
	url.RawQuery = q.Encode()
	return url.String()
}
LOGZ.IO GRAFANA CHANGE :: end
*/

func ExtendData(data *Data, logger log.Logger) *ExtendedData {
	alerts := make([]ExtendedAlert, 0, len(data.Alerts))

	for _, alert := range data.Alerts {
		extendedAlert := extendAlert(alert, data.ExternalURL, logger)
		alerts = append(alerts, *extendedAlert)
	}

	extended := &ExtendedData{
		Receiver:          data.Receiver,
		Status:            data.Status,
		Alerts:            alerts,
		GroupLabels:       data.GroupLabels,
		CommonLabels:      removePrivateItems(data.CommonLabels),
		CommonAnnotations: removePrivateItems(data.CommonAnnotations),

		ExternalURL: data.ExternalURL,
	}
	return extended
}

func TmplText(ctx context.Context, tmpl *Template, alerts []*types.Alert, l log.Logger, tmplErr *error) (func(string) string, *ExtendedData) {
	promTmplData := notify.GetTemplateData(ctx, tmpl, alerts, l)
	data := ExtendData(promTmplData, l)

	return func(name string) (s string) {
		if *tmplErr != nil {
			return
		}
		s, *tmplErr = tmpl.ExecuteTextString(name, data)
		return s
	}, data
}

// Firing returns the subset of alerts that are firing.
func (as ExtendedAlerts) Firing() []ExtendedAlert {
	res := []ExtendedAlert{}
	for _, a := range as {
		if a.Status == string(model.AlertFiring) {
			res = append(res, a)
		}
	}
	return res
}

// Resolved returns the subset of alerts that are resolved.
func (as ExtendedAlerts) Resolved() []ExtendedAlert {
	res := []ExtendedAlert{}
	for _, a := range as {
		if a.Status == string(model.AlertResolved) {
			res = append(res, a)
		}
	}
	return res
}
