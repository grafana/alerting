package generate

import (
	"strings"
	"time"

	"github.com/go-openapi/strfmt"
	models "github.com/grafana/grafana-openapi-client-go/models"
	"pgregory.net/rapid"
)

// Helpers
func buildQuery(dsUID, refID string) *models.AlertQuery {
	// __expr__ math or a basic prom query
	model := map[string]any{
		"refId":      refID,
		"type":       "math",
		"expression": "1 == 1",
		"datasource": map[string]any{"type": "__expr__", "uid": "__expr__"},
	}
	if dsUID != "__expr__" {
		model = map[string]any{
			"refId":      refID,
			"type":       "query",
			"datasource": map[string]any{"uid": dsUID},
			"expr":       "vector(1)",
		}
	}
	return &models.AlertQuery{
		DatasourceUID:     dsUID,
		Model:             model,
		QueryType:         "",
		RefID:             refID,
		RelativeTimeRange: &models.RelativeTimeRange{From: models.Duration(600), To: models.Duration(0)},
	}
}

func genTitle() *rapid.Generator[string] {
	return rapid.StringMatching(`[A-Za-z][A-Za-z0-9_\-]{3,20}`)
}

func genSummary() *rapid.Generator[string] {
	return rapid.StringMatching(`.{10,60}`)
}

func genDurationStr() *rapid.Generator[string] {
	return rapid.SampledFrom([]string{"0s", "30s", "1m", "5m", "10m"})
}

func genLabels() *rapid.Generator[map[string]string] {
	keys := []string{"team", "service", "env", "region"}
	val := rapid.StringMatching(`[a-z][a-z0-9\-]{2,10}`)
	keyFn := func(s string) string {
		if len(s) == 0 {
			return keys[0]
		}
		var h uint32
		for i := 0; i < len(s); i++ {
			h = h*16777619 ^ uint32(s[i])
		}
		return keys[int(h)%len(keys)]
	}
	return rapid.MapOfValues(val, keyFn)
}

func genMetricName() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		prefix := rapid.SampledFrom([]string{"grafana", "app", "service", "custom"}).Draw(t, "prefix")
		parts := rapid.SliceOfN(rapid.StringMatching(`[a-z][a-z0-9_]{2,8}`), 1, 3).Draw(t, "parts")
		return prefix + "_" + strings.Join(parts, "_")
	})
}

func RandomUID() *rapid.Generator[string] {
	return rapid.StringMatching(`[A-Za-z0-9\-_]{8,16}`)
}

// genAdditionalAnnotations returns a random small map of annotation keys -> values (excluding summary)
func genAdditionalAnnotations() *rapid.Generator[map[string]string] {
	keys := []string{"runbook_url", "dashboard", "description", "priority", "owner", "ticket"}
	return rapid.Custom(func(t *rapid.T) map[string]string {
		n := rapid.IntRange(0, 4).Draw(t, "ann_n")
		m := make(map[string]string)
		for i := 0; i < n; i++ {
			k := rapid.SampledFrom(keys).Draw(t, "ann_key")
			if _, exists := m[k]; exists {
				continue
			}
			var v string
			switch k {
			case "runbook_url", "dashboard":
				v = genURL().Draw(t, "ann_url")
			case "priority":
				v = rapid.SampledFrom([]string{"P1", "P2", "P3", "P4"}).Draw(t, "ann_priority")
			case "owner":
				v = rapid.StringMatching(`[a-z][a-z0-9_\-]{2,12}`).Draw(t, "ann_owner")
			case "ticket":
				v = rapid.StringMatching(`[A-Z]{2,4}-[0-9]{2,5}`).Draw(t, "ann_ticket")
			default:
				v = genSummary().Draw(t, "ann_desc")
			}
			m[k] = v
		}
		return m
	})
}

func genURL() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		host := rapid.SampledFrom([]string{"example.com", "grafana.net", "runbooks.local"}).Draw(t, "url_host")
		segs := rapid.SliceOfN(rapid.StringMatching(`[a-z0-9\-]{3,10}`), 1, 3).Draw(t, "url_segs")
		return "https://" + host + "/" + strings.Join(segs, "/")
	})
}

func strPtr[T ~string](s T) *T { return &s }

// genExecErrState returns a valid exec error state value
func genExecErrState() *rapid.Generator[string] {
	return rapid.SampledFrom([]string{"OK", "Alerting", "Error"})
}

// genNoDataState returns a valid no data state value
func genNoDataState() *rapid.Generator[string] {
	return rapid.SampledFrom([]string{"Alerting", "NoData", "OK"})
}

func mustParseDuration(s string) strfmt.Duration {
	if s == "" {
		return strfmt.Duration(0)
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return strfmt.Duration(0)
	}
	return strfmt.Duration(d)
}
