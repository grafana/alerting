package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/grafana/grafana-app-sdk/k8s"
	"github.com/grafana/grafana-app-sdk/logging"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

const (
	// rulesAPIGroup is the Kubernetes API group serving Grafana alert rules.
	rulesAPIGroup = "rules.alerting.grafana.app"
	// rulesAPIVersion is the version of the alert rules API used for RBAC lookups.
	rulesAPIVersion = "v0alpha1"
	// alertRulesResource is the plural resource name of the AlertRule kind.
	// An AlertRule's metadata.name is the rule UID stored in notification history.
	alertRulesResource = "alertrules"
	// rulesListPageSize is the page size used when listing accessible rules.
	rulesListPageSize = 500
)

// ruleAccessReader resolves which alert rule UIDs the caller (identified by ctx)
// is allowed to read within a namespace.
type ruleAccessReader interface {
	// AccessibleRuleUIDs returns the set of alert rule UIDs the caller may read.
	// RBAC is enforced by the API server, so the returned set only ever contains
	// rules the caller is permitted to see.
	AccessibleRuleUIDs(ctx context.Context, namespace string) (ruleUIDSet, error)
}

// ruleFilter constrains a query to the set of alert rule UIDs the caller can
// access. A nil *ruleFilter disables RBAC filtering. A non-nil filter with no
// UIDs means the caller has access to no rules, and the query must return nothing.
type ruleFilter struct {
	// uids is the sorted set of accessible rule UIDs.
	uids []string
	// access is the set form of uids, used for post-processing (e.g. counts by rule).
	access ruleUIDSet
}

// newRuleFilter builds a ruleFilter from an accessible rule UID set.
func newRuleFilter(access ruleUIDSet) *ruleFilter {
	return &ruleFilter{uids: access.sorted(), access: access}
}

// empty reports whether the caller can access no rules.
func (f *ruleFilter) empty() bool {
	return f == nil || len(f.uids) == 0
}

// ruleUIDSet is a set of alert rule UIDs.
type ruleUIDSet map[string]struct{}

// Has reports whether uid is in the set.
func (s ruleUIDSet) Has(uid string) bool {
	_, ok := s[uid]
	return ok
}

// sorted returns the UIDs as a sorted slice for stable query construction.
func (s ruleUIDSet) sorted() []string {
	out := make([]string, 0, len(s))
	for uid := range s {
		out = append(out, uid)
	}
	sort.Strings(out)
	return out
}

// k8sRuleAccessReader lists AlertRule resources through the Kubernetes rules API.
// The request identity is carried on the context and forwarded to the API server,
// which applies RBAC and only returns rules the caller can access.
type k8sRuleAccessReader struct {
	client rest.Interface
	logger logging.Logger
}

// newK8sRuleAccessReader builds a rule access reader from a base kube config.
// The base config is expected to route requests to the same API server that
// serves the rules API (for in-process Grafana this is the loopback config,
// which forwards the request identity from the context).
func newK8sRuleAccessReader(kubeConfig rest.Config, logger logging.Logger) (*k8sRuleAccessReader, error) {
	cfg := kubeConfig
	cfg.APIPath = "/apis"
	cfg.GroupVersion = &schema.GroupVersion{
		Group:   rulesAPIGroup,
		Version: rulesAPIVersion,
	}
	if cfg.NegotiatedSerializer == nil {
		cfg.NegotiatedSerializer = &k8s.GenericNegotiatedSerializer{}
	}

	client, err := rest.RESTClientFor(&cfg)
	if err != nil {
		return nil, fmt.Errorf("build rules API client: %w", err)
	}

	return &k8sRuleAccessReader{client: client, logger: logger}, nil
}

// partialRuleList is a minimal projection of an AlertRule list response. Only the
// resource name (the rule UID) and the pagination cursor are needed.
type partialRuleList struct {
	Metadata struct {
		Continue string `json:"continue"`
	} `json:"metadata"`
	Items []struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
	} `json:"items"`
}

// AccessibleRuleUIDs lists all alert rules the caller can read in the namespace and
// returns their UIDs. It follows pagination until the full set is collected.
func (r *k8sRuleAccessReader) AccessibleRuleUIDs(ctx context.Context, namespace string) (ruleUIDSet, error) {
	set := make(ruleUIDSet)
	cont := ""
	for {
		req := r.client.Get().
			Namespace(namespace).
			Resource(alertRulesResource).
			// Request only object metadata to avoid transferring full rule specs.
			SetHeader("Accept", "application/json;as=PartialObjectMetadataList;g=meta.k8s.io;v=v1,application/json").
			Param("limit", strconv.Itoa(rulesListPageSize))
		if cont != "" {
			req = req.Param("continue", cont)
		}

		raw, err := req.Do(ctx).Raw()
		if err != nil {
			return nil, fmt.Errorf("list alert rules: %w", err)
		}

		var page partialRuleList
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, fmt.Errorf("unmarshal alert rule list: %w", err)
		}

		for _, item := range page.Items {
			if item.Metadata.Name != "" {
				set[item.Metadata.Name] = struct{}{}
			}
		}

		cont = page.Metadata.Continue
		if cont == "" {
			break
		}
	}

	return set, nil
}
