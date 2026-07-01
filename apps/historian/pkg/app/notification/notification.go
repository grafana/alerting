package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/grafana/grafana-app-sdk/app"
	"github.com/grafana/grafana-app-sdk/logging"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/trace"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	"github.com/grafana/alerting/apps/historian/pkg/apis/alertinghistorian/v0alpha1"
	"github.com/grafana/alerting/apps/historian/pkg/app/config"
)

type Notification struct {
	loki   *LokiReader
	logger logging.Logger

	// rbacEnabled indicates results must be restricted to the accessible rules.
	rbacEnabled bool
	// ruleAccess resolves the alert rule UIDs the requester can access. When
	// rbacEnabled is true this must be non-nil for queries to be served.
	ruleAccess ruleAccessReader
}

func New(cfg config.NotificationConfig, kubeConfig rest.Config, reg prometheus.Registerer, logger logging.Logger, tracer trace.Tracer) *Notification {
	if !cfg.Enabled {
		return &Notification{}
	}

	n := &Notification{
		loki:        NewLokiReader(cfg.Loki, reg, logger, tracer),
		logger:      logger,
		rbacEnabled: cfg.RBACEnabled,
	}

	if cfg.RBACEnabled {
		reader, err := newK8sRuleAccessReader(kubeConfig, logger)
		if err != nil {
			// Leave ruleAccess nil; handlers fail closed when RBAC is enabled but
			// the reader could not be constructed.
			logger.Error("Failed to construct rules access reader; RBAC-protected notification queries will be rejected", "err", err)
		} else {
			n.ruleAccess = reader
		}
	}

	return n
}

// resolveRuleFilter returns the RBAC rule filter for the request. It returns a
// nil filter when RBAC is disabled (no filtering). When RBAC is enabled it lists
// the rules the requester can access and returns a filter restricting results to
// them.
func (n *Notification) resolveRuleFilter(ctx context.Context, namespace string) (*ruleFilter, error) {
	if !n.rbacEnabled {
		return nil, nil
	}
	if n.ruleAccess == nil {
		return nil, errors.New("rule access reader is not configured")
	}
	access, err := n.ruleAccess.AccessibleRuleUIDs(ctx, namespace)
	if err != nil {
		return nil, err
	}
	return newRuleFilter(access), nil
}

func (n *Notification) QueryAlertsHandler(ctx context.Context, writer app.CustomRouteResponseWriter, request *app.CustomRouteRequest) error {
	start := time.Now()

	if n.loki == nil {
		const msg = "Notification alerts query whilst disabled"
		n.logger.Debug(msg)
		return &apierrors.StatusError{
			ErrStatus: metav1.Status{
				Status:  metav1.StatusFailure,
				Code:    http.StatusUnprocessableEntity,
				Message: msg,
			}}
	}

	var body v0alpha1.CreateNotificationsqueryalertsRequestBody
	err := json.NewDecoder(request.Body).Decode(&body)
	if err != nil {
		const msg = "Notification alerts query malformed"
		n.logger.Debug(msg, "err", err)
		return &apierrors.StatusError{
			ErrStatus: metav1.Status{
				Status:  metav1.StatusFailure,
				Code:    http.StatusBadRequest,
				Message: fmt.Sprintf("%s: %s", msg, err.Error()),
			}}
	}

	filter, err := n.resolveRuleFilter(ctx, request.ResourceIdentifier.Namespace)
	if err != nil {
		const msg = "Notification alerts query authorization failed"
		n.logger.Error(msg, "err", err, "duration", time.Since(start))
		return &apierrors.StatusError{
			ErrStatus: metav1.Status{
				Status:  metav1.StatusFailure,
				Code:    http.StatusInternalServerError,
				Message: fmt.Sprintf("%s: %s", msg, err.Error()),
			}}
	}

	response, err := n.loki.QueryAlerts(ctx, body, filter)
	if err != nil {
		if errors.Is(err, ErrInvalidQuery) {
			const msg = "Notification alerts query invalid"
			n.logger.Debug(msg, "err", err)
			return &apierrors.StatusError{
				ErrStatus: metav1.Status{
					Status:  metav1.StatusFailure,
					Code:    http.StatusBadRequest,
					Message: fmt.Sprintf("%s: %s", msg, err.Error()),
				}}
		}
		const msg = "Notification alerts query failed"
		n.logger.Error(msg, "err", err, "duration", time.Since(start))
		return &apierrors.StatusError{
			ErrStatus: metav1.Status{
				Status:  metav1.StatusFailure,
				Code:    http.StatusInternalServerError,
				Message: fmt.Sprintf("%s: %s", msg, err.Error()),
			}}
	}

	n.logger.Debug("Notification alerts query success",
		"alerts", len(response.Alerts),
		"duration", time.Since(start))

	writer.Header().Add("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	return json.NewEncoder(writer).Encode(response)
}

func (n *Notification) QueryHandler(ctx context.Context, writer app.CustomRouteResponseWriter, request *app.CustomRouteRequest) error {
	start := time.Now()

	if n.loki == nil {
		const msg = "Notification history query whilst disabled"
		n.logger.Debug(msg)
		return &apierrors.StatusError{
			ErrStatus: metav1.Status{
				Status:  metav1.StatusFailure,
				Code:    http.StatusUnprocessableEntity,
				Message: msg,
			}}
	}

	var body v0alpha1.CreateNotificationqueryRequestBody
	err := json.NewDecoder(request.Body).Decode(&body)
	if err != nil {
		const msg = "Notification history query malformed"
		n.logger.Debug(msg, "err", err)
		return &apierrors.StatusError{
			ErrStatus: metav1.Status{
				Status:  metav1.StatusFailure,
				Code:    http.StatusBadRequest,
				Message: fmt.Sprintf("%s: %s", msg, err.Error()),
			}}
	}

	filter, err := n.resolveRuleFilter(ctx, request.ResourceIdentifier.Namespace)
	if err != nil {
		const msg = "Notification history query authorization failed"
		n.logger.Error(msg, "err", err, "duration", time.Since(start))
		return &apierrors.StatusError{
			ErrStatus: metav1.Status{
				Status:  metav1.StatusFailure,
				Code:    http.StatusInternalServerError,
				Message: fmt.Sprintf("%s: %s", msg, err.Error()),
			}}
	}

	response, err := n.loki.Query(ctx, body, filter)
	if err != nil {
		if errors.Is(err, ErrInvalidQuery) {
			const msg = "Notification history query invalid"
			n.logger.Debug(msg, "err", err)
			return &apierrors.StatusError{
				ErrStatus: metav1.Status{
					Status:  metav1.StatusFailure,
					Code:    http.StatusBadRequest,
					Message: fmt.Sprintf("%s: %s", msg, err.Error()),
				}}
		}
		const msg = "Notification history query failed"
		n.logger.Error(msg, "err", err, "duration", time.Since(start))
		return &apierrors.StatusError{
			ErrStatus: metav1.Status{
				Status:  metav1.StatusFailure,
				Code:    http.StatusInternalServerError,
				Message: fmt.Sprintf("%s: %s", msg, err.Error()),
			}}
	}

	n.logger.Debug("Notification history query success",
		"entries", len(response.Entries),
		"counts", len(response.Counts),
		"duration", time.Since(start))

	writer.Header().Add("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	return json.NewEncoder(writer).Encode(response)
}
