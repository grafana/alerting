package setting

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend/gtime"

	"github.com/grafana/grafana/pkg/util"

	"github.com/prometheus/alertmanager/cluster"
	"gopkg.in/ini.v1"
)

const (
	alertmanagerDefaultClusterAddr        = "0.0.0.0:9094"
	alertmanagerDefaultPeerTimeout        = 15 * time.Second
	alertmanagerDefaultGossipInterval     = cluster.DefaultGossipInterval
	alertmanagerDefaultPushPullInterval   = cluster.DefaultPushPullInterval
	alertmanagerDefaultConfigPollInterval = 60 * time.Second
	// To start, the alertmanager needs at least one route defined.
	// TODO: we should move this to Grafana settings and define this as the default.
	alertmanagerDefaultConfiguration = `{
	"alertmanager_config": {
		"route": {
			"receiver": "grafana-default-email",
			"group_by": ["grafana_folder", "alertname"]
		},
		"receivers": [{
			"name": "grafana-default-email",
			"grafana_managed_receiver_configs": [{
				"uid": "",
				"name": "email receiver",
				"type": "email",
				"isDefault": true,
				"settings": {
					"addresses": "<example@email.com>"
				}
			}]
		}]
	}
}
`
	evaluatorDefaultEvaluationTimeout       = 30 * time.Second
	schedulerDefaultAdminConfigPollInterval = 60 * time.Second
	schedulereDefaultExecuteAlerts          = true
	schedulerDefaultMaxAttempts             = 3
	schedulerDefaultLegacyMinInterval       = 1
	screenshotsDefaultCapture               = false
	screenshotsDefaultMaxConcurrent         = 5
	screenshotsDefaultUploadImageStorage    = false
	// SchedulerBaseInterval base interval of the scheduler. Controls how often the scheduler fetches database for new changes as well as schedules evaluation of a rule
	// changing this value is discouraged because this could cause existing alert definition
	// with intervals that are not exactly divided by this number not to be evaluated
	SchedulerBaseInterval = 10 * time.Second
	// DefaultRuleEvaluationInterval indicates a default interval of for how long a rule should be evaluated to change state from Pending to Alerting
	DefaultRuleEvaluationInterval = SchedulerBaseInterval * 6 // == 60 seconds
)

type UnifiedAlertingSettings struct {
	AdminConfigPollInterval        time.Duration
	AlertmanagerConfigPollInterval time.Duration
	HAListenAddr                   string
	HAAdvertiseAddr                string
	HAPeers                        []string
	HAPeerTimeout                  time.Duration
	HAGossipInterval               time.Duration
	HAPushPullInterval             time.Duration
	MaxAttempts                    int64
	MinInterval                    time.Duration
	EvaluationTimeout              time.Duration
	ExecuteAlerts                  bool
	DefaultConfiguration           string
	Enabled                        *bool // determines whether unified alerting is enabled. If it is nil then user did not define it and therefore its value will be determined during migration. Services should not use it directly.
	DisabledOrgs                   map[int64]struct{}
	// BaseInterval interval of time the scheduler updates the rules and evaluates rules.
	// Only for internal use and not user configuration.
	BaseInterval time.Duration
	// DefaultRuleEvaluationInterval default interval between evaluations of a rule.
	DefaultRuleEvaluationInterval time.Duration
	Screenshots                   UnifiedAlertingScreenshotSettings
	ReservedLabels                UnifiedAlertingReservedLabelSettings
}

type UnifiedAlertingScreenshotSettings struct {
	Capture                    bool
	MaxConcurrentScreenshots   int64
	UploadExternalImageStorage bool
}

type UnifiedAlertingReservedLabelSettings struct {
	DisabledLabels map[string]struct{}
}

// IsEnabled returns true if UnifiedAlertingSettings.Enabled is either nil or true.
// It hides the implementation details of the Enabled and simplifies its usage.
func (u *UnifiedAlertingSettings) IsEnabled() bool {
	return u.Enabled == nil || *u.Enabled
}

// IsReservedLabelDisabled returns true if UnifiedAlertingReservedLabelSettings.DisabledLabels contains the given reserved label.
func (u *UnifiedAlertingReservedLabelSettings) IsReservedLabelDisabled(label string) bool {
	_, ok := u.DisabledLabels[label]
	return ok
}

// readUnifiedAlertingEnabledSettings reads the settings for unified alerting.
// It returns a non-nil bool and a nil error when unified alerting is enabled either
// because it has been enabled in the settings or by default. It returns nil and
// a non-nil error both unified alerting and legacy alerting are enabled at the same time.
func (cfg *Cfg) readUnifiedAlertingEnabledSetting(section *ini.Section) (*bool, error) {
	// At present an invalid value is considered the same as no value. This means that a
	// spelling mistake in the string "false" could enable unified alerting rather
	// than disable it. This issue can be found here
	hasEnabled := section.Key("enabled").Value() != ""
	if !hasEnabled {
		// TODO: Remove in Grafana v9
		if cfg.IsFeatureToggleEnabled("ngalert") {
			cfg.Logger.Warn("ngalert feature flag is deprecated: use unified alerting enabled setting instead")
			// feature flag overrides the legacy alerting setting
			legacyAlerting := false
			AlertingEnabled = &legacyAlerting
			unifiedAlerting := true
			return &unifiedAlerting, nil
		}

		// if legacy alerting has not been configured then enable unified alerting
		if AlertingEnabled == nil {
			unifiedAlerting := true
			return &unifiedAlerting, nil
		}

		// enable unified alerting and disable legacy alerting
		legacyAlerting := false
		AlertingEnabled = &legacyAlerting
		unifiedAlerting := true
		return &unifiedAlerting, nil
	}

	unifiedAlerting, err := section.Key("enabled").Bool()
	if err != nil {
		// the value for unified alerting is invalid so disable all alerting
		legacyAlerting := false
		AlertingEnabled = &legacyAlerting
		return nil, fmt.Errorf("invalid value %s, should be either true or false", section.Key("enabled"))
	}

	// If both legacy and unified alerting are enabled then return an error
	if AlertingEnabled != nil && *AlertingEnabled && unifiedAlerting {
		return nil, errors.New("legacy and unified alerting cannot both be enabled at the same time, please disable one of them and restart Grafana")
	}

	if AlertingEnabled == nil {
		legacyAlerting := !unifiedAlerting
		AlertingEnabled = &legacyAlerting
	}

	return &unifiedAlerting, nil
}

// ReadUnifiedAlertingSettings reads both the `unified_alerting` and `alerting` sections of the configuration while preferring configuration the `alerting` section.
// It first reads the `unified_alerting` section, then looks for non-defaults on the `alerting` section and prefers those.
func (cfg *Cfg) ReadUnifiedAlertingSettings(iniFile *ini.File) error {
	var err error
	uaCfg := UnifiedAlertingSettings{}
	ua := iniFile.Section("unified_alerting")
	uaCfg.Enabled, err = cfg.readUnifiedAlertingEnabledSetting(ua)
	if err != nil {
		return fmt.Errorf("failed to read unified alerting enabled setting: %w", err)
	}

	uaCfg.DisabledOrgs = make(map[int64]struct{})
	orgsStr := valueAsString(ua, "disabled_orgs", "")
	for _, org := range util.SplitString(orgsStr) {
		orgID, err := strconv.ParseInt(org, 10, 64)
		if err != nil {
			return err
		}
		uaCfg.DisabledOrgs[orgID] = struct{}{}
	}

	uaCfg.AdminConfigPollInterval, err = gtime.ParseDuration(valueAsString(ua, "admin_config_poll_interval", (schedulerDefaultAdminConfigPollInterval).String()))
	if err != nil {
		return err
	}
	uaCfg.AlertmanagerConfigPollInterval, err = gtime.ParseDuration(valueAsString(ua, "alertmanager_config_poll_interval", (alertmanagerDefaultConfigPollInterval).String()))
	if err != nil {
		return err
	}
	uaCfg.HAPeerTimeout, err = gtime.ParseDuration(valueAsString(ua, "ha_peer_timeout", (alertmanagerDefaultPeerTimeout).String()))
	if err != nil {
		return err
	}
	uaCfg.HAGossipInterval, err = gtime.ParseDuration(valueAsString(ua, "ha_gossip_interval", (alertmanagerDefaultGossipInterval).String()))
	if err != nil {
		return err
	}
	uaCfg.HAPushPullInterval, err = gtime.ParseDuration(valueAsString(ua, "ha_push_pull_interval", (alertmanagerDefaultPushPullInterval).String()))
	if err != nil {
		return err
	}
	uaCfg.HAListenAddr = ua.Key("ha_listen_address").MustString(alertmanagerDefaultClusterAddr)
	uaCfg.HAAdvertiseAddr = ua.Key("ha_advertise_address").MustString("")
	peers := ua.Key("ha_peers").MustString("")
	uaCfg.HAPeers = make([]string, 0)
	if peers != "" {
		for _, peer := range strings.Split(peers, ",") {
			peer = strings.TrimSpace(peer)
			uaCfg.HAPeers = append(uaCfg.HAPeers, peer)
		}
	}

	// TODO load from ini file
	uaCfg.DefaultConfiguration = alertmanagerDefaultConfiguration

	alerting := iniFile.Section("alerting")

	uaExecuteAlerts := ua.Key("execute_alerts").MustBool(schedulereDefaultExecuteAlerts)
	if uaExecuteAlerts { // unified option equals the default (true)
		legacyExecuteAlerts := alerting.Key("execute_alerts").MustBool(schedulereDefaultExecuteAlerts)
		if !legacyExecuteAlerts {
			cfg.Logger.Warn("falling back to legacy setting of 'execute_alerts'; please use the configuration option in the `unified_alerting` section if Grafana 8 alerts are enabled.")
		}
		uaExecuteAlerts = legacyExecuteAlerts
	}
	uaCfg.ExecuteAlerts = uaExecuteAlerts

	// if the unified alerting options equal the defaults, apply the respective legacy one
	uaEvaluationTimeout, err := gtime.ParseDuration(valueAsString(ua, "evaluation_timeout", evaluatorDefaultEvaluationTimeout.String()))
	if err != nil || uaEvaluationTimeout == evaluatorDefaultEvaluationTimeout { // unified option is invalid duration or equals the default
		legaceEvaluationTimeout := time.Duration(alerting.Key("evaluation_timeout_seconds").MustInt64(int64(evaluatorDefaultEvaluationTimeout.Seconds()))) * time.Second
		if legaceEvaluationTimeout != evaluatorDefaultEvaluationTimeout {
			cfg.Logger.Warn("falling back to legacy setting of 'evaluation_timeout_seconds'; please use the configuration option in the `unified_alerting` section if Grafana 8 alerts are enabled.")
		}
		uaEvaluationTimeout = legaceEvaluationTimeout
	}
	uaCfg.EvaluationTimeout = uaEvaluationTimeout

	uaMaxAttempts := ua.Key("max_attempts").MustInt64(schedulerDefaultMaxAttempts)
	if uaMaxAttempts == schedulerDefaultMaxAttempts { // unified option or equals the default
		legacyMaxAttempts := alerting.Key("max_attempts").MustInt64(schedulerDefaultMaxAttempts)
		if legacyMaxAttempts != schedulerDefaultMaxAttempts {
			cfg.Logger.Warn("falling back to legacy setting of 'max_attempts'; please use the configuration option in the `unified_alerting` section if Grafana 8 alerts are enabled.")
		}
		uaMaxAttempts = legacyMaxAttempts
	}
	uaCfg.MaxAttempts = uaMaxAttempts

	uaCfg.BaseInterval = SchedulerBaseInterval

	uaMinInterval, err := gtime.ParseDuration(valueAsString(ua, "min_interval", uaCfg.BaseInterval.String()))
	if err != nil || uaMinInterval == uaCfg.BaseInterval { // unified option is invalid duration or equals the default
		// if the legacy option is invalid, fallback to 10 (unified alerting min interval default)
		legacyMinInterval := time.Duration(alerting.Key("min_interval_seconds").MustInt64(int64(uaCfg.BaseInterval.Seconds()))) * time.Second
		if legacyMinInterval > uaCfg.BaseInterval {
			cfg.Logger.Warn("falling back to legacy setting of 'min_interval_seconds'; please use the configuration option in the `unified_alerting` section if Grafana 8 alerts are enabled.")
			uaMinInterval = legacyMinInterval
		} else {
			// if legacy interval is smaller than the base interval, adjust it to the base interval
			uaMinInterval = uaCfg.BaseInterval
		}
	}

	if uaMinInterval < uaCfg.BaseInterval {
		return fmt.Errorf("value of setting 'min_interval' should be greater than the base interval (%v)", uaCfg.BaseInterval)
	}
	if uaMinInterval%uaCfg.BaseInterval != 0 {
		return fmt.Errorf("value of setting 'min_interval' should be times of base interval (%v)", uaCfg.BaseInterval)
	}
	uaCfg.MinInterval = uaMinInterval

	uaCfg.DefaultRuleEvaluationInterval = DefaultRuleEvaluationInterval
	if uaMinInterval > uaCfg.DefaultRuleEvaluationInterval {
		uaCfg.DefaultRuleEvaluationInterval = uaMinInterval
	}

	screenshots := iniFile.Section("unified_alerting.screenshots")
	uaCfgScreenshots := uaCfg.Screenshots

	uaCfgScreenshots.Capture = screenshots.Key("capture").MustBool(screenshotsDefaultCapture)
	uaCfgScreenshots.MaxConcurrentScreenshots = screenshots.Key("max_concurrent_screenshots").MustInt64(screenshotsDefaultMaxConcurrent)
	uaCfgScreenshots.UploadExternalImageStorage = screenshots.Key("upload_external_image_storage").MustBool(screenshotsDefaultUploadImageStorage)
	uaCfg.Screenshots = uaCfgScreenshots

	reservedLabels := iniFile.Section("unified_alerting.reserved_labels")
	uaCfgReservedLabels := UnifiedAlertingReservedLabelSettings{
		DisabledLabels: make(map[string]struct{}),
	}
	for _, label := range util.SplitString(reservedLabels.Key("disabled_labels").MustString("")) {
		uaCfgReservedLabels.DisabledLabels[label] = struct{}{}
	}
	uaCfg.ReservedLabels = uaCfgReservedLabels

	cfg.UnifiedAlerting = uaCfg
	return nil
}

func GetAlertmanagerDefaultConfiguration() string {
	return alertmanagerDefaultConfiguration
}
