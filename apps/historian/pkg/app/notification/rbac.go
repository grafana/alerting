package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	authtypes "github.com/grafana/authlib/types"
	"github.com/grafana/grafana-app-sdk/k8s"
	"github.com/grafana/grafana-app-sdk/logging"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

const (
	// folderAPIGroup is the Kubernetes API group serving Grafana folders.
	folderAPIGroup = "folder.grafana.app"
	// folderAPIVersion is the version of the folder API used to enumerate folders.
	folderAPIVersion = "v1beta1"
	// foldersResource is the plural resource name of the Folder kind. A folder's
	// metadata.name is the folder UID stored in notification history (folder_uids).
	foldersResource = "folders"
	// folderListPageSize is the page size used when listing folders.
	folderListPageSize = 500

	// rulesAPIGroup is the Kubernetes API group of Grafana alert rules. It is not
	// queried directly (the historian never lists rules); it identifies the
	// resource for the alert.rules:read authorization check performed per folder.
	rulesAPIGroup = "rules.alerting.grafana.app"
	// alertRulesResource is the plural resource name of the AlertRule kind, used as
	// the resource for the per-folder alert.rules:read authorization check.
	alertRulesResource = "alertrules"
)

// folderAccessReader resolves which folders' alert rules the caller (identified
// by ctx) is allowed to read within a namespace.
type folderAccessReader interface {
	// AccessibleFolders returns the set of folder UIDs whose alert rules the
	// caller may read. Both the folder enumeration and the per-folder
	// authorization are RBAC-enforced, so the returned set only ever contains
	// folders the caller is permitted to see.
	AccessibleFolders(ctx context.Context, namespace string) (ruleUIDSet, error)
}

// ruleFilter constrains a query to what the caller can access. A nil *ruleFilter
// disables RBAC filtering. A non-nil filter whose folder keys are empty means the
// caller has access to nothing, and the query must return nothing.
//
// The LogQL push-down matches accessible folder UIDs (folder_uids/folder_uid
// structured metadata): folders are far fewer than rules, so the matcher stays
// small and cheap for Loki even for large tenants. Alert-rule RBAC is strictly
// folder-scoped, so a caller who can read a folder can read every alert rule in
// it; folder-level filtering is therefore equivalent to per-rule RBAC without
// enumerating individual rules.
type ruleFilter struct {
	// folderKeys are the accessible folder UIDs used in the push-down matcher,
	// sorted and non-empty.
	folderKeys []string
}

// newRuleFilter builds a ruleFilter from the set of accessible folder UIDs.
func newRuleFilter(folders ruleUIDSet) *ruleFilter {
	return &ruleFilter{folderKeys: folders.sorted()}
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

	// Guard against a misconfigured max query size that leaves no room for even a
	// single folder key. Without this every RBAC query would fail with the more
	// confusing per-folder error below; surface it as a configuration problem.
	if budget <= fixed {
		return nil, fmt.Errorf("%w: the configured Loki max query size (%d bytes) is too small to build notification history RBAC queries; increase the loki max-query-size setting", ErrInvalidQuery, maxQuerySize)
	}

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

// k8sFolderAccessReader resolves the folders whose alert rules a caller can read.
// It enumerates the tenant's folders through the multi-tenant folder API and then
// confirms alert.rules:read on each folder via the authz AccessClient.
type k8sFolderAccessReader struct {
	client rest.Interface
	access authtypes.AccessClient
	logger logging.Logger
}

// newFolderAccessReader builds a folder access reader from a base kube config and
// an authz access client. The base config is expected to route requests to an API
// server that serves the multi-tenant folder API, forwarding the request identity
// from the context. access must be non-nil.
func newFolderAccessReader(kubeConfig rest.Config, access authtypes.AccessClient, logger logging.Logger) (*k8sFolderAccessReader, error) {
	if access == nil {
		return nil, fmt.Errorf("access client is required for RBAC")
	}

	cfg := kubeConfig
	cfg.APIPath = "/apis"
	cfg.GroupVersion = &schema.GroupVersion{
		Group:   folderAPIGroup,
		Version: folderAPIVersion,
	}
	if cfg.NegotiatedSerializer == nil {
		cfg.NegotiatedSerializer = &k8s.GenericNegotiatedSerializer{}
	}

	client, err := rest.RESTClientFor(&cfg)
	if err != nil {
		return nil, fmt.Errorf("build folder API client: %w", err)
	}

	return &k8sFolderAccessReader{client: client, access: access, logger: logger}, nil
}

// partialFolderList is a minimal projection of a Folder list response. Only the
// resource name (the folder UID) and the pagination cursor are needed.
type partialFolderList struct {
	Metadata struct {
		Continue string `json:"continue"`
	} `json:"metadata"`
	Items []struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
	} `json:"items"`
}

// AccessibleFolders enumerates the tenant's folders and returns the subset whose
// alert rules the caller may read. It lists folders via the folder API (RBAC
// enforced by that API server) and then confirms alert.rules:read on each folder
// through the AccessClient, so the result reflects alert-rule read access rather
// than mere folder visibility.
func (r *k8sFolderAccessReader) AccessibleFolders(ctx context.Context, namespace string) (ruleUIDSet, error) {
	candidates, err := r.listFolders(ctx, namespace)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return ruleUIDSet{}, nil
	}
	return r.filterReadableFolders(ctx, namespace, candidates)
}

// listFolders lists all folder UIDs visible to the caller in the namespace,
// following pagination until the full set is collected.
func (r *k8sFolderAccessReader) listFolders(ctx context.Context, namespace string) ([]string, error) {
	var folders []string
	cont := ""
	for {
		req := r.client.Get().
			Namespace(namespace).
			Resource(foldersResource).
			// Request only object metadata to avoid transferring full folder specs.
			SetHeader("Accept", "application/json;as=PartialObjectMetadataList;g=meta.k8s.io;v=v1,application/json").
			Param("limit", strconv.Itoa(folderListPageSize))
		if cont != "" {
			req = req.Param("continue", cont)
		}

		raw, err := req.Do(ctx).Raw()
		if err != nil {
			return nil, fmt.Errorf("list folders: %w", err)
		}

		var page partialFolderList
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, fmt.Errorf("unmarshal folder list: %w", err)
		}

		for _, item := range page.Items {
			if item.Metadata.Name != "" {
				folders = append(folders, item.Metadata.Name)
			}
		}

		cont = page.Metadata.Continue
		if cont == "" {
			break
		}
	}

	return folders, nil
}

// filterReadableFolders returns the subset of candidate folders in which the
// caller may read alert rules (alert.rules:read), determined via the AccessClient.
// Checks are issued in batches bounded by authtypes.MaxBatchCheckItems.
func (r *k8sFolderAccessReader) filterReadableFolders(ctx context.Context, namespace string, candidates []string) (ruleUIDSet, error) {
	info, ok := authtypes.AuthInfoFrom(ctx)
	if !ok {
		return nil, fmt.Errorf("no auth info in context")
	}

	accessible := make(ruleUIDSet, len(candidates))
	for start := 0; start < len(candidates); start += authtypes.MaxBatchCheckItems {
		end := start + authtypes.MaxBatchCheckItems
		if end > len(candidates) {
			end = len(candidates)
		}
		batch := candidates[start:end]

		checks := make([]authtypes.BatchCheckItem, len(batch))
		for i, uid := range batch {
			checks[i] = authtypes.BatchCheckItem{
				// The folder UID uniquely identifies the check within the batch.
				CorrelationID: uid,
				Verb:          "list",
				Group:         rulesAPIGroup,
				Resource:      alertRulesResource,
				// A folder-scoped question ("may the caller read alert rules in this
				// folder"): no rule name, only the parent folder.
				Folder: uid,
			}
		}

		resp, err := r.access.BatchCheck(ctx, info, authtypes.BatchCheckRequest{
			Namespace: namespace,
			Checks:    checks,
		})
		if err != nil {
			return nil, fmt.Errorf("check folder alert rule access: %w", err)
		}

		for _, uid := range batch {
			result := resp.Results[uid]
			if result.Error != nil {
				return nil, fmt.Errorf("check folder %q alert rule access: %w", uid, result.Error)
			}
			if result.Allowed {
				accessible[uid] = struct{}{}
			}
		}
	}

	return accessible, nil
}
