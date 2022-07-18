package alerting

import (
	"context"
	"fmt"

	"github.com/grafana/grafana/pkg/components/simplejson"
	"github.com/grafana/grafana/pkg/models"
)

// AlertTest makes a test alert.
func (e *AlertEngine) AlertTest(orgID int64, dashboard *simplejson.Json, panelID int64, user *models.SignedInUser) (*EvalContext, error) {
	dash := models.NewDashboardFromJson(dashboard)
	dashInfo := DashAlertInfo{
		User:  user,
		Dash:  dash,
		OrgID: orgID,
	}
	alerts, err := e.dashAlertExtractor.GetAlerts(context.Background(), dashInfo)
	if err != nil {
		return nil, err
	}

	for _, alert := range alerts {
		if alert.PanelId != panelID {
			continue
		}
		rule, err := NewRuleFromDBAlert(context.Background(), e.sqlStore, alert, true)
		if err != nil {
			return nil, err
		}

		handler := NewEvalHandler(e.DataService)

		context := NewEvalContext(context.Background(), rule, fakeRequestValidator{}, e.sqlStore, nil)
		context.IsTestRun = true
		context.IsDebug = true

		handler.Eval(context)
		context.Rule.State = context.GetNewState()
		return context, nil
	}

	return nil, fmt.Errorf("could not find alert with panel ID %d", panelID)
}
