package alerting

import (
	apimodels "github.com/grafana/grafana/pkg/services/ngalert/api/tooling/definitions"
)

func (am *GrafanaAlertmanager) GetStatus() apimodels.GettableStatus {
	am.reloadConfigMtx.RLock()
	defer am.reloadConfigMtx.RUnlock()

	config := apimodels.PostableApiAlertingConfig{}
	if am.ready() {
		config = am.config.AlertmanagerConfig
	}
	return *apimodels.NewGettableStatus(&config)
}
