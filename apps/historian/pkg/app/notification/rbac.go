package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

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
	// folderLabelKey is the label the rules API sets on every AlertRule to record
	// the UID of the folder it lives in. It is returned in PartialObjectMetadataList
	// responses, so folder UIDs can be collected during the rule enumeration at no
	// extra cost. Mirrors grafana.app/folder (apps/alerting/rules ext.go).
	folderLabelKey = "grafana.app/folder"
)

// ruleAccessReader resolves which alert rules (and their folders) the caller
// (identified by ctx) is allowed to read within a namespace.
type ruleAccessReader interface {
	// AccessibleScope returns the set of alert rule UIDs the caller may read plus
	// the set of folder UIDs those rules live in. RBAC is enforced by the API
	// server, so the returned sets only ever contain rules the caller is
	// permitted to see.
	AccessibleScope(ctx context.Context, namespace string) (accessScope, error)
}

// accessScope is the set of alert rules a caller can read together with the
// folders those rules belong to. Folders are always derived from the accessible
// rules, so the two sets describe the same visibility at different granularities.
type accessScope struct {
	rules   ruleUIDSet
	folders ruleUIDSet
}

// ruleFilter constrains a query to what the caller can access. A nil *ruleFilter
// disables RBAC filtering. A non-nil filter whose folder keys are empty means the
// caller has access to nothing, and the query must return nothing.
//
// The LogQL push-down matches accessible folder UIDs (folder_uids/folder_uid
// structured metadata): folders are far fewer than rules, so the matcher stays
// small and cheap for Loki even for large tenants. Folders are derived from the
// rules the caller can access, so visibility is exactly per-rule RBAC. The rule
// set is retained so groupBy.ruleUID counts still strip individual inaccessible
// rules co-referenced by a notification.
type ruleFilter struct {
	// folderKeys are the accessible folder UIDs used in the push-down matcher,
	// sorted and non-empty.
	folderKeys []string
	// rules is the set of accessible rule UIDs, used to strip inaccessible rules
	// from groupBy.ruleUID counts.
	rules ruleUIDSet
}

// newRuleFilter builds a ruleFilter from an accessible scope.
func newRuleFilter(scope accessScope) *ruleFilter {
	return &ruleFilter{folderKeys: scope.folders.sorted(), rules: scope.rules}
}

// empty reports whether the caller can access nothing (no folder keys).
func (f *ruleFilter) empty() bool {
	return f == nil || len(f.folderKeys) == 0
}

// folderBatchReservedBytes is subtracted from the configured Loki max query size
// when splitting the accessible folder set into batches. It leaves headroom for
// the parts of the final query that are added around the folder push-down after
// batching (e.g. the metric-query wrappers sum()/topk()/count_over_time() and
// their range selectors), so a batch sized against the log filter still fits once
// wrapped.
const folderBatchReservedBytes = 1024

// folderFilterSpec describes how to render an accessible-folder push-down for a
// particular stream: the structured-metadata field plus the regex fragments that
// wrap the alternation of folder UIDs. It lets the notifications stream (multi-
// valued folder_uids) and the alerts stream (single-valued folder_uid) share the
// same rendering and size-aware batching logic.
type folderFilterSpec struct {
	// label is the structured-metadata field the push-down matches against.
	label string
	// prefix is the regex emitted before the "a|b|c" folder alternation.
	prefix string
	// suffix is the regex emitted after the folder alternation.
	suffix string
}

var (
	// notificationFolderSpec matches notifications referencing at least one
	// accessible folder. folder_uids is stored as a comma-separated list, so each
	// UID is anchored to a comma or the start/end of the value.
	notificationFolderSpec = folderFilterSpec{label: "folder_uids", prefix: "(^|.*,)(", suffix: ")($|,.*)"}
	// alertFolderSpec matches alerts belonging to an accessible folder. The alerts
	// stream stores a single folder_uid per entry.
	alertFolderSpec = folderFilterSpec{label: "folder_uid", prefix: "^(", suffix: ")$"}
)

// render builds the LogQL folder push-down fragment (with its leading " | ") for
// the given already-escaped folder keys. An empty key list yields the scaffolding
// with an empty alternation, which is also how fixedLen measures per-fragment
// overhead.
func (s folderFilterSpec) render(escaped []string) string {
	return fmt.Sprintf(` | %s =~ "%s%s%s"`, s.label, s.prefix, strings.Join(escaped, "|"), s.suffix)
}

// fixedLen is the byte cost of a fragment excluding the folder keys and their
// separators (label, operator, quotes, prefix/suffix anchors).
func (s folderFilterSpec) fixedLen() int {
	return len(s.render(nil))
}

// splitFolderKeys splits folderKeys into batches whose rendered push-down
// fragment, added to a query of overhead bytes, keeps each query within
// maxQuerySize. Sizing accounts for regex escaping of each key. It mirrors the
// batching in Grafana alert-state history's BuildLogQuery: a tenant with more
// accessible folders than fit in one query is served by several batched queries
// instead of having the query rejected by Loki.
//
// maxQuerySize <= 0 disables batching (all keys in a single batch). It returns
// ErrInvalidQuery if a single folder key cannot fit even on its own, matching
// state history's fail-fast behaviour rather than emitting a query Loki will
// reject opaquely.
func splitFolderKeys(folderKeys []string, spec folderFilterSpec, overhead, maxQuerySize int) ([][]string, error) {
	if len(folderKeys) == 0 {
		return [][]string{nil}, nil
	}
	if maxQuerySize <= 0 {
		return [][]string{folderKeys}, nil
	}

	budget := maxQuerySize - folderBatchReservedBytes
	// Per-fragment fixed cost: the surrounding query plus the fragment scaffolding
	// (label, operator, anchors) with an empty alternation.
	fixed := overhead + spec.fixedLen()

	var batches [][]string
	remaining := folderKeys
	for len(remaining) > 0 {
		cur := fixed
		var batch []string
		for len(remaining) > 0 {
			add := len(regexp.QuoteMeta(remaining[0]))
			if len(batch) > 0 {
				add++ // '|' separator between alternatives
			}
			if cur+add > budget {
				if len(batch) == 0 {
					return nil, fmt.Errorf("%w: accessible folder %q is too large to query within the Loki max query size (%d bytes)", ErrInvalidQuery, remaining[0], maxQuerySize)
				}
				break
			}
			cur += add
			batch = append(batch, remaining[0])
			remaining = remaining[1:]
		}
		batches = append(batches, batch)
	}
	return batches, nil
}

// ruleUIDSet is a set of UIDs (rule or folder).
type ruleUIDSet map[string]struct{}

// Has reports whether uid is in the set.
func (s ruleUIDSet) Has(uid string) bool {
	_, ok := s[uid]
	return ok
}

// sorted returns the non-empty UIDs as a sorted slice for stable query
// construction. Empty values are skipped so a rule in an unknown folder never
// produces an empty matcher alternative (which would over-match).
func (s ruleUIDSet) sorted() []string {
	out := make([]string, 0, len(s))
	for uid := range s {
		if uid == "" {
			continue
		}
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
// resource name (the rule UID), the folder label and the pagination cursor are
// needed.
type partialRuleList struct {
	Metadata struct {
		Continue string `json:"continue"`
	} `json:"metadata"`
	Items []struct {
		Metadata struct {
			Name   string            `json:"name"`
			Labels map[string]string `json:"labels"`
		} `json:"metadata"`
	} `json:"items"`
}

// AccessibleScope lists all alert rules the caller can read in the namespace and
// returns their UIDs together with the folder UIDs those rules live in (read from
// the grafana.app/folder label). It follows pagination until the full set is
// collected.
func (r *k8sRuleAccessReader) AccessibleScope(ctx context.Context, namespace string) (accessScope, error) {
	scope := accessScope{rules: make(ruleUIDSet), folders: make(ruleUIDSet)}
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
			return accessScope{}, fmt.Errorf("list alert rules: %w", err)
		}

		var page partialRuleList
		if err := json.Unmarshal(raw, &page); err != nil {
			return accessScope{}, fmt.Errorf("unmarshal alert rule list: %w", err)
		}

		for _, item := range page.Items {
			if item.Metadata.Name != "" {
				scope.rules[item.Metadata.Name] = struct{}{}
			}
			if folder := item.Metadata.Labels[folderLabelKey]; folder != "" {
				scope.folders[folder] = struct{}{}
			}
		}

		cont = page.Metadata.Continue
		if cont == "" {
			break
		}
	}

	return scope, nil
}
