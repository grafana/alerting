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

	lc, err := s.NewLokiClient()
	require.NoError(t, err)

	timeout := time.After(time.Minute * 10)
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	// notifications only start arriving after 2 to 3 minutes so we wait for that
	time.Sleep(time.Minute * 2)

	for {
		select {
		case <-ticker.C:
			ar, err := wc.GetAlerts()
			if err != nil {
				fmt.Printf("failed to get alert notifications: %v\n", err)
				continue
			}

			st, err := lc.GetCurrentAlertState()
			if err != nil {
				fmt.Printf("failed to get alert notifications: %v\n", err)
				continue
			}

			// we want to fetch all notifications after the last state change
			var i int
			for i = range ar.History {
				if ar.History[i].TimeNow.Before(st.Timestamp) {
					continue
				}
			}

			for ; i < len(ar.History); i++ {
				alert := ar.History[i]
				if st.State != AlertStateAlerting && alert.Status == "firing" {
					t.Errorf("flapping notifications - got firing notification when alert state wasn't firing anymore, state = %v, notification = %v", st, alert)
				}
			}

		case <-timeout:
			t.Error("test timedout")
			t.FailNow()
		}
	}
}
