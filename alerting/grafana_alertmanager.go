package alerting

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	amv2 "github.com/prometheus/alertmanager/api/v2/models"
	"github.com/prometheus/alertmanager/cluster"
	"github.com/prometheus/alertmanager/config"
	"github.com/prometheus/alertmanager/dispatch"
	"github.com/prometheus/alertmanager/inhibit"
	"github.com/prometheus/alertmanager/nflog"
	"github.com/prometheus/alertmanager/nflog/nflogpb"
	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/provider/mem"
	"github.com/prometheus/alertmanager/silence"
	pb "github.com/prometheus/alertmanager/silence/silencepb"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/timeinterval"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
)

const (
	// defaultResolveTimeout is the default timeout used for resolving an alert
	// if the end time is not specified.
	defaultResolveTimeout = 5 * time.Minute
	// memoryAlertsGCInterval is the interval at which we'll remove resolved alerts from memory.
	memoryAlertsGCInterval = 30 * time.Minute
)

func init() {
	silence.ValidateMatcher = func(m *pb.Matcher) error {
		switch m.Type {
		case pb.Matcher_EQUAL, pb.Matcher_NOT_EQUAL:
			if !model.LabelValue(m.Pattern).IsValid() {
				return fmt.Errorf("invalid label value %q", m.Pattern)
			}
		case pb.Matcher_REGEXP, pb.Matcher_NOT_REGEXP:
			if _, err := regexp.Compile(m.Pattern); err != nil {
				return fmt.Errorf("invalid regular expression %q: %s", m.Pattern, err)
			}
		default:
			return fmt.Errorf("unknown matcher type %q", m.Type)
		}
		return nil
	}
}

type ClusterPeer interface {
	AddState(string, cluster.State, prometheus.Registerer) cluster.ClusterChannel
	Position() int
	WaitReady(context.Context) error
}

type GrafanaAlertmanager struct {
	logger  log.Logger
	Metrics *GrafanaAlertmanagerMetrics

	tenantID int64

	marker      types.Marker
	alerts      *mem.Alerts
	route       *dispatch.Route
	peer        ClusterPeer
	peerTimeout time.Duration

	// wg is for dispatcher, inhibitor, silences and notifications
	// Across configuration changes dispatcher and inhibitor are completely replaced, however, silences, notification log and alerts remain the same.
	// stopc is used to let silences and notifications know we are done.
	wg    sync.WaitGroup
	stopc chan struct{}

	notificationLog *nflog.Log
	dispatcher      *dispatch.Dispatcher
	inhibitor       *inhibit.Inhibitor
	silencer        *silence.Silencer
	silences        *silence.Silences
	templates       *Template

	// muteTimes is a map where the key is the name of the mute_time_interval
	// and the value represents all configured time_interval(s)
	muteTimes map[string][]timeinterval.TimeInterval

	stageMetrics      *notify.Metrics
	dispatcherMetrics *dispatch.DispatcherMetrics

	reloadConfigMtx sync.RWMutex
	configHash      [16]byte
	config          []byte
}

// MaintenanceOptions represent the configuration options available for executing maintenance of Silences and the Notification log that the Alertmanager uses.
type MaintenanceOptions interface {
	// Filepath returns the string representation of the filesystem path of the file to do maintenance on.
	Filepath() string
	// Retention represents for how long should we keep the artefacts under maintenance.
	Retention() time.Duration
	// MaintenanceFrequency represents how often should we execute the maintenance.
	MaintenanceFrequency() time.Duration
	// MaintenanceFunc returns the function to execute as part of the maintenance process.
	// It returns the size of the file in bytes or an error if the maintenance fails.
	MaintenanceFunc() (int64, error)
}

type Template = template.Template
type InhibitRule = config.InhibitRule
type MuteTimeInterval = config.MuteTimeInterval
type Route = config.Route
type Integration = notify.Integration
type DispatcherLimits = dispatch.Limits
type Notifier = notify.Notifier

// Configuration is an interface for accessing Alertmanager configuration.
type Configuration interface {
	DispatcherLimits() DispatcherLimits
	InhibitRules() []*InhibitRule
	MuteTimeIntervals() []MuteTimeInterval
	ReceiverIntegrations() (map[string][]Integration, error)
	RoutingTree() *Route
	Templates() *Template

	Hash() [16]byte
	Raw() []byte
}

type GrafanaAlertmanagerConfig struct {
	WorkingDirectory   string
	AlertStoreCallback mem.AlertStoreCallback
	PeerTimeout        time.Duration

	Silences MaintenanceOptions
	Nflog    MaintenanceOptions
}

func (c *GrafanaAlertmanagerConfig) Validate() error {
	if c.Silences == nil {
		return errors.New("silence maintenance options must be present")
	}

	if c.Nflog == nil {
		return errors.New("notification log maintenance options must be present")
	}

	return nil
}

// NewGrafanaAlertmanager creates a new Grafana-specific Alertmanager.
func NewGrafanaAlertmanager(tenantKey string, tenantID int64, config *GrafanaAlertmanagerConfig, peer ClusterPeer, logger log.Logger, m *GrafanaAlertmanagerMetrics) (*GrafanaAlertmanager, error) {
	// TODO: Remove the context.
	am := &GrafanaAlertmanager{
		stopc:             make(chan struct{}),
		logger:            log.With(logger, "component", "alertmanager", tenantKey, tenantID),
		marker:            types.NewMarker(m.Registerer),
		stageMetrics:      notify.NewMetrics(m.Registerer),
		dispatcherMetrics: dispatch.NewDispatcherMetrics(false, m.Registerer),
		peer:              peer,
		peerTimeout:       config.PeerTimeout,
		Metrics:           m,
		tenantID:          tenantID,
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	var err error

	// Initialize the notification log
	am.wg.Add(1)
	am.notificationLog, err = nflog.New(
		nflog.WithRetention(config.Nflog.Retention()),
		nflog.WithSnapshot(config.Nflog.Filepath()),
		nflog.WithMaintenance(config.Nflog.MaintenanceFrequency(), am.stopc, am.wg.Done, func() (int64, error) {
			//TODO: There's a bug here, we need to call GC to ensure we cleanup old entries: https://github.com/grafana/alerting/issues/3
			return config.Nflog.MaintenanceFunc()
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize the notification log component of alerting: %w", err)
	}
	c := am.peer.AddState(fmt.Sprintf("notificationlog:%d", am.tenantID), am.notificationLog, m.Registerer)
	am.notificationLog.SetBroadcast(c.Broadcast)

	// Initialize silences
	am.silences, err = silence.New(silence.Options{
		Metrics:      m.Registerer,
		SnapshotFile: config.Silences.Filepath(),
		Retention:    config.Silences.Retention(),
	})
	if err != nil {
		return nil, fmt.Errorf("unable to initialize the silencing component of alerting: %w", err)
	}

	c = am.peer.AddState(fmt.Sprintf("silences:%d", am.tenantID), am.silences, m.Registerer)
	am.silences.SetBroadcast(c.Broadcast)

	am.wg.Add(1)
	go func() {
		am.silences.Maintenance(config.Silences.MaintenanceFrequency(), config.Silences.Filepath(), am.stopc, func() (int64, error) {
			// Delete silences older than the retention period.
			if _, err := am.silences.GC(); err != nil {
				level.Error(am.logger).Log("silence garbage collection", "err", err)
				// Don't return here - we need to snapshot our state first.
			}

			// Snapshot our silences to the Grafana KV store
			return config.Silences.MaintenanceFunc()
		})
		am.wg.Done()
	}()

	// Initialize in-memory alerts
	am.alerts, err = mem.NewAlerts(context.Background(), am.marker, memoryAlertsGCInterval, config.AlertStoreCallback, am.logger)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize the alert provider component of alerting: %w", err)
	}

	return am, nil
}

func (am *GrafanaAlertmanager) Ready() bool {
	// We consider AM as ready only when the config has been
	// applied at least once successfully. Until then, some objects
	// can still be nil.
	am.reloadConfigMtx.RLock()
	defer am.reloadConfigMtx.RUnlock()

	return am.ready()
}

func (am *GrafanaAlertmanager) ready() bool {
	return am.config != nil
}

func (am *GrafanaAlertmanager) StopAndWait() {
	if am.dispatcher != nil {
		am.dispatcher.Stop()
	}

	if am.inhibitor != nil {
		am.inhibitor.Stop()
	}

	am.alerts.Close()

	close(am.stopc)

	am.wg.Wait()
}

func (am *GrafanaAlertmanager) ConfigHash() [16]byte {
	return am.configHash
}

func (am *GrafanaAlertmanager) WithReadLock(fn func()) {
	am.reloadConfigMtx.RLock()
	defer am.reloadConfigMtx.RUnlock()
	fn()
}

func (am *GrafanaAlertmanager) WithLock(fn func()) {
	am.reloadConfigMtx.Lock()
	defer am.reloadConfigMtx.Unlock()
	fn()
}

// TemplateFromPaths returns a set of *Templates based on the paths given.
func (am *GrafanaAlertmanager) TemplateFromPaths(u string, paths ...string) (*Template, error) {
	tmpl, err := template.FromGlobs(paths...)
	if err != nil {
		return nil, err
	}
	externalURL, err := url.Parse(u)
	if err != nil {
		return nil, err
	}
	tmpl.ExternalURL = externalURL
	return tmpl, nil
}

func (am *GrafanaAlertmanager) buildMuteTimesMap(muteTimeIntervals []config.MuteTimeInterval) map[string][]timeinterval.TimeInterval {
	muteTimes := make(map[string][]timeinterval.TimeInterval, len(muteTimeIntervals))
	for _, ti := range muteTimeIntervals {
		muteTimes[ti.Name] = ti.TimeIntervals
	}
	return muteTimes
}

// ApplyConfig applies a new configuration by re-initializing all components using the configuration provided.
// It is not safe to call concurrently.
func (am *GrafanaAlertmanager) ApplyConfig(cfg Configuration) (err error) {
	// Finally, build the integrations map using the receiver configuration and templates.

	integrationsMap, err := cfg.ReceiverIntegrations()
	if err != nil {
		return fmt.Errorf("failed to build integration map: %w", err)
	}

	// Now, let's put together our notification pipeline
	routingStage := make(notify.RoutingStage, len(integrationsMap))

	if am.inhibitor != nil {
		am.inhibitor.Stop()
	}
	if am.dispatcher != nil {
		am.dispatcher.Stop()
	}

	am.inhibitor = inhibit.NewInhibitor(am.alerts, cfg.InhibitRules(), am.marker, am.logger)
	am.muteTimes = am.buildMuteTimesMap(cfg.MuteTimeIntervals())
	am.silencer = silence.NewSilencer(am.silences, am.marker, am.logger)

	meshStage := notify.NewGossipSettleStage(am.peer)
	inhibitionStage := notify.NewMuteStage(am.inhibitor)
	timeMuteStage := notify.NewTimeMuteStage(am.muteTimes)
	silencingStage := notify.NewMuteStage(am.silencer)
	for name := range integrationsMap {
		stage := am.createReceiverStage(name, integrationsMap[name], am.waitFunc, am.notificationLog)
		routingStage[name] = notify.MultiStage{meshStage, silencingStage, timeMuteStage, inhibitionStage, stage}
	}

	am.route = dispatch.NewRoute(cfg.RoutingTree(), nil)
	am.dispatcher = dispatch.NewDispatcher(am.alerts, am.route, routingStage, am.marker, am.timeoutFunc, cfg.DispatcherLimits(), am.logger, am.dispatcherMetrics)

	am.wg.Add(1)
	go func() {
		defer am.wg.Done()
		am.dispatcher.Run()
	}()

	am.wg.Add(1)
	go func() {
		defer am.wg.Done()
		am.inhibitor.Run()
	}()

	am.configHash = cfg.Hash()
	am.config = cfg.Raw()
	am.templates = cfg.Templates()

	return nil
}

// PutAlerts receives the alerts and then sends them through the corresponding route based on whenever the alert has a receiver embedded or not
func (am *GrafanaAlertmanager) PutAlerts(postableAlerts amv2.PostableAlerts) error {
	now := time.Now()
	alerts := make([]*types.Alert, 0, len(postableAlerts))
	var validationErr *AlertValidationError
	for _, a := range postableAlerts {
		alert := &types.Alert{
			Alert: model.Alert{
				Labels:       model.LabelSet{},
				Annotations:  model.LabelSet{},
				StartsAt:     time.Time(a.StartsAt),
				EndsAt:       time.Time(a.EndsAt),
				GeneratorURL: a.GeneratorURL.String(),
			},
			UpdatedAt: now,
		}

		for k, v := range a.Labels {
			if len(v) == 0 { // Skip empty labels.
				continue
			}

			alert.Alert.Labels[model.LabelName(k)] = model.LabelValue(v)
		}

		for k, v := range a.Annotations {
			if len(v) == 0 { // Skip empty annotation.
				continue
			}
			alert.Alert.Annotations[model.LabelName(k)] = model.LabelValue(v)
		}

		// Ensure StartsAt is set.
		if alert.StartsAt.IsZero() {
			if alert.EndsAt.IsZero() {
				alert.StartsAt = now
			} else {
				alert.StartsAt = alert.EndsAt
			}
		}
		// If no end time is defined, set a timeout after which an alert
		// is marked resolved if it is not updated.
		if alert.EndsAt.IsZero() {
			alert.Timeout = true
			alert.EndsAt = now.Add(defaultResolveTimeout)
		}

		if alert.EndsAt.After(now) {
			am.Metrics.Firing().Inc()
		} else {
			am.Metrics.Resolved().Inc()
		}

		if err := validateAlert(alert); err != nil {
			if validationErr == nil {
				validationErr = &AlertValidationError{}
			}
			validationErr.Alerts = append(validationErr.Alerts, a)
			validationErr.Errors = append(validationErr.Errors, err)
			am.Metrics.Invalid().Inc()
			continue
		}

		alerts = append(alerts, alert)
	}

	if err := am.alerts.Put(alerts...); err != nil {
		// Notification sending alert takes precedence over validation errors.
		return err
	}
	if validationErr != nil {
		// Even if validationErr is nil, the require.NoError fails on it.
		return validationErr
	}
	return nil
}

// validateAlert is a.Validate() while additionally allowing
// space for label and annotation names.
func validateAlert(a *types.Alert) error {
	if a.StartsAt.IsZero() {
		return fmt.Errorf("start time missing")
	}
	if !a.EndsAt.IsZero() && a.EndsAt.Before(a.StartsAt) {
		return fmt.Errorf("start time must be before end time")
	}
	if err := validateLabelSet(a.Labels); err != nil {
		return fmt.Errorf("invalid label set: %s", err)
	}
	if len(a.Labels) == 0 {
		return fmt.Errorf("at least one label pair required")
	}
	if err := validateLabelSet(a.Annotations); err != nil {
		return fmt.Errorf("invalid annotations: %s", err)
	}
	return nil
}

// validateLabelSet is ls.Validate() while additionally allowing
// space for label names.
func validateLabelSet(ls model.LabelSet) error {
	for ln, lv := range ls {
		if !isValidLabelName(ln) {
			return fmt.Errorf("invalid name %q", ln)
		}
		if !lv.IsValid() {
			return fmt.Errorf("invalid value %q", lv)
		}
	}
	return nil
}

// isValidLabelName is ln.IsValid() without restrictions other than it can not be empty.
// The regex for Prometheus data model is ^[a-zA-Z_][a-zA-Z0-9_]*$.
func isValidLabelName(ln model.LabelName) bool {
	if len(ln) == 0 {
		return false
	}

	return utf8.ValidString(string(ln))
}

// AlertValidationError is the error capturing the validation errors
// faced on the alerts.
type AlertValidationError struct {
	Alerts amv2.PostableAlerts
	Errors []error // Errors[i] refers to Alerts[i].
}

func (e AlertValidationError) Error() string {
	errMsg := ""
	if len(e.Errors) != 0 {
		errMsg = e.Errors[0].Error()
		for _, e := range e.Errors[1:] {
			errMsg += ";" + e.Error()
		}
	}
	return errMsg
}

// createReceiverStage creates a pipeline of stages for a receiver.
func (am *GrafanaAlertmanager) createReceiverStage(name string, integrations []notify.Integration, wait func() time.Duration, notificationLog notify.NotificationLog) notify.Stage {
	var fs notify.FanoutStage
	for i := range integrations {
		recv := &nflogpb.Receiver{
			GroupName:   name,
			Integration: integrations[i].Name(),
			Idx:         uint32(integrations[i].Index()),
		}
		var s notify.MultiStage
		s = append(s, notify.NewWaitStage(wait))
		s = append(s, notify.NewDedupStage(&integrations[i], notificationLog, recv))
		s = append(s, notify.NewRetryStage(integrations[i], name, am.stageMetrics))
		s = append(s, notify.NewSetNotifiesStage(notificationLog, recv))

		fs = append(fs, s)
	}
	return fs
}

func (am *GrafanaAlertmanager) waitFunc() time.Duration {
	return time.Duration(am.peer.Position()) * am.peerTimeout
}

func (am *GrafanaAlertmanager) timeoutFunc(d time.Duration) time.Duration {
	// time.Duration d relates to the receiver's group_interval. Even with a group interval of 1s,
	// we need to make sure (non-position-0) peers in the cluster wait before flushing the notifications.
	if d < notify.MinTimeout {
		d = notify.MinTimeout
	}
	return d + am.waitFunc()
}

func (am *GrafanaAlertmanager) getTemplate() (*template.Template, error) {
	am.reloadConfigMtx.RLock()
	defer am.reloadConfigMtx.RUnlock()
	if !am.ready() {
		return nil, errors.New("alertmanager is not initialized")
	}

	return am.templates, nil
}

// TODO: This needs an implementation
func (am *GrafanaAlertmanager) buildReceiverIntegration(next *GrafanaReceiver, tmpl *template.Template) (Notifier, error) {
	return nil, nil
}
