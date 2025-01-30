package integration

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFlappingAlerts(t *testing.T) {
	s, err := NewAlertmanagerScenario()
	require.NoError(t, err)
	defer s.Close()

	s.Start(t, 20, "15s")
	s.Provision(t, provisionCfg{
		alertRuleConfig: alertRuleConfig{
			pendingPeriod:                  "30s",
			groupEvaluationIntervalSeconds: 10,
		},
		notificationPolicyCfg: notificationPolicyCfg{
			groupWait:      "30s",
			groupInterval:  "1m",
			repeatInterval: "30m",
		},
	})

	wc, err := s.NewWebhookClient()
	require.NoError(t, err)

	timeout := time.After(time.Minute * 10)
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// as, err := c.AlertmanagerGetAlertsNames()
			as, err := wc.GetAlerts()
			// as, err := c.AlertQuery()
			// require.NoError(t, err)
			if err != nil {
				fmt.Printf("failed to get alert notifications: %v\n", err)
				continue
			}
			//if err = s.Grafanas["grafana-1"].Pause(); err != nil {
			//	fmt.Printf("err = %v\n", err)
			//}
			fmt.Println()
			fmt.Printf("alerts: %v\n", as)
			fmt.Println()

		case <-timeout:
			t.FailNow()
		}
	}
}
