package channels

import (
	"fmt"
	"strings"

	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/common/model"
)

const (
	FooterIconURL      = "https://grafana.com/assets/img/fav32.png"
	ColorAlertFiring   = "#D63232"
	ColorAlertResolved = "#36a64f"
)

func getAlertStatusColor(status model.AlertStatus) string {
	if status == model.AlertFiring {
		return ColorAlertFiring
	}
	return ColorAlertResolved
}

func getTitleFromTemplateData(data *template.Data) string {
	title := "[" + data.Status
	if data.Status == string(model.AlertFiring) {
		title += fmt.Sprintf(":%d", len(data.Alerts.Firing()))
	}
	title += "] " + strings.Join(data.GroupLabels.SortedPairs().Values(), " ") + " "
	if len(data.CommonLabels) > len(data.GroupLabels) {
		title += "(" + strings.Join(data.CommonLabels.Remove(data.GroupLabels.Names()).Values(), " ") + ")"
	}
	return title
}
