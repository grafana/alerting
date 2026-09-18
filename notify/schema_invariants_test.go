package notify

import (
	"encoding/json"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	alertingHTTP "github.com/grafana/alerting/http"
	"github.com/grafana/alerting/notify/notifytest"
	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/receivers/schema"
)

// This file mechanically enforces the schema<->config struct invariants documented in CLAUDE.md's
// "Schema field rules":
//  1. schema.Field.PropertyName must equal the JSON tag of the corresponding config struct field.
//  2. A config struct field of type receivers.Secret/receivers.SecretURL (or a pointer to one) must
//     be marked Secure in the schema, and a schema field marked Secure must map to a struct field
//     that is capable of holding a secret.
//  3. Struct fields whose name contains "File" or "Ref" are deliberately excluded from schemas.
//  4. Inline-embedded structs have their fields expanded into the parent schema, not nested.
//  5. Subform / subform-array fields are compared against their corresponding nested struct type.
//
// Configs whose wire structs are local to NewConfig are checked against their full valid JSON
// fixtures instead. This checks schema/fixture field names and secure value shapes, but cannot
// discover fields missing from both the schema and fixture or infer Secret types from JSON.

// secretType and secretURLType are the Go types that CLAUDE.md requires to be marked Secure in the
// schema. receivers.NotifierConfig is a special case: its only field (send_resolved) is common to
// every integration and is never exposed via the per-integration schema, so it's skipped outright
// rather than expanded like other inline structs.
var (
	secretType         = reflect.TypeOf(receivers.Secret(""))
	secretURLType      = reflect.TypeOf(receivers.SecretURL{})
	notifierConfigType = reflect.TypeOf(receivers.NotifierConfig{})
)

// receiversPkgPrefix bounds how deep subform recursion goes: only structs declared under
// github.com/grafana/alerting/receivers/... are recursed into. Shared HTTP-client schema blocks
// (github.com/grafana/alerting/http/v0mimir, used by nearly every legacy receiver via
// httpcfg.V0HttpConfigOption()) are deliberately not recursed into here - they're the same block
// duplicated on every legacy config, so any mismatch in them would otherwise be reported once per
// receiver instead of once, and they aren't a "registered integration schema" in their own right.
const receiversPkgPrefix = "github.com/grafana/alerting/receivers"

func TestIntegrationSchemasMatchConfigStructs(t *testing.T) {
	for _, integration := range GetSchemaForAllIntegrations() {
		for _, version := range integration.Versions {
			t.Run(string(integration.Type)+"/"+string(version.Version), func(t *testing.T) {
				factory, ok := GetFactoryForIntegrationVersion(integration.Type, version.Version)
				require.True(t, ok, "schema version has no factory")
				configType := factory.ConfigType()
				require.NotNil(t, configType)
				require.Equal(t, reflect.Struct, derefType(configType).Kind())
				// These parsers use function-local wire structs; their output configs have no JSON tags.
				if version.Version == schema.V1 && slices.Contains([]schema.IntegrationType{
					schema.AlertManagerType, schema.OpsGenieType, schema.PushoverType,
				}, integration.Type) {
					fixture, ok := notifytest.AllKnownConfigsForTesting[notifytest.IntegrationVersionKey{Type: integration.Type, Version: version.Version}]
					require.True(t, ok, "integration has no full valid JSON fixture")
					require.NoError(t, factory.ValidateConfig(json.RawMessage(fixture.Config), func(_ string, fallback string) (string, bool) {
						return fallback, false
					}))
					var config map[string]any
					require.NoError(t, json.Unmarshal([]byte(fixture.Config), &config))
					compareFieldsToJSON(t, "", version.Options, []map[string]any{config})
					return
				}
				options := version.Options
				if integration.Type == schema.WebhookType && version.Version == schema.V1 {
					// notify parses http_config separately from the notifier's settings.
					options = make([]schema.Field, 0, len(version.Options))
					httpFields := 0
					for _, field := range version.Options {
						if field.PropertyName == "http_config" {
							httpFields++
							compareFieldsToStruct(t, "http_config", field.SubformOptions, reflect.TypeFor[alertingHTTP.HTTPClientConfig]())
							// HTTP types are outside the struct walk's receivers package
							// boundary. Retain recursive fixture coverage for their subforms.
							var httpConfig map[string]any
							require.NoError(t, json.Unmarshal([]byte(notifytest.FullValidHTTPConfigForTesting), &httpConfig))
							compareFieldsToJSON(t, "", []schema.Field{field}, []map[string]any{httpConfig})
							continue
						}
						options = append(options, field)
					}
					require.Equal(t, 1, httpFields)
				}
				compareFieldsToStruct(t, "", options, configType)
			})
		}
	}
}

// Array entries may populate different optional fields, so compare their combined keys.
// Only subforms are traversed; keys of arbitrary maps (headers, vars, fields) are user-defined.
func compareFieldsToJSON(t *testing.T, path string, fields []schema.Field, objects []map[string]any) {
	t.Helper()
	values := make(map[string][]any)
	for _, object := range objects {
		for key, value := range object {
			values[key] = append(values[key], value)
		}
	}
	for _, field := range fields {
		fieldPath := joinPath(path, field.PropertyName)
		entries, ok := values[field.PropertyName]
		if !ok {
			t.Errorf("%s: schema field is missing from full valid JSON fixtures", fieldPath)
			continue
		}
		delete(values, field.PropertyName)
		var nested []map[string]any
		for _, value := range entries {
			if field.Secure {
				_, ok := value.(string)
				require.True(t, ok, "%s: secure field must have a string value in the fixture", fieldPath)
			}
			if field.Element == schema.ElementTypeSubform {
				object, ok := value.(map[string]any)
				require.True(t, ok, "%s: subform fixture must be an object", fieldPath)
				nested = append(nested, object)
				continue
			}
			if field.Element == schema.ElementSubformArray {
				array, ok := value.([]any)
				require.True(t, ok, "%s: subform-array fixture must be an array", fieldPath)
				for _, entry := range array {
					object, ok := entry.(map[string]any)
					require.True(t, ok, "%s: subform-array entries must be objects", fieldPath)
					nested = append(nested, object)
				}
			}
		}
		if isRecursableElement(field.Element) {
			compareFieldsToJSON(t, fieldPath, field.SubformOptions, nested)
		}
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		t.Errorf("%s: JSON fixture field has no matching schema field", joinPath(path, key))
	}
}

// compareFieldsToStruct checks schema fields against the flattened config fields.
// path identifies nested fields in failure messages.
func compareFieldsToStruct(t *testing.T, path string, fields []schema.Field, configType reflect.Type) {
	t.Helper()
	flat := flattenConfigFields(configType)
	seen := make(map[string]bool, len(fields))
	for _, f := range fields {
		seen[f.PropertyName] = true
		fieldPath := joinPath(path, f.PropertyName)
		sf, ok := flat[f.PropertyName]
		if !ok {
			t.Errorf("%s: schema field has no struct field with a matching json tag in %s", fieldPath, configType)
			continue
		}

		if isSecretGoType(sf.Type) && !f.Secure {
			t.Errorf("%s: struct field %s has type %s but schema field is not marked Secure", fieldPath, sf.Name, sf.Type)
		}
		// Native receivers may decrypt secrets into plain strings.
		if f.Secure && !isPlausibleSecretHolder(sf.Type) {
			t.Errorf("%s: schema field is marked Secure but struct field %s has type %s, which can't hold a secret", fieldPath, sf.Name, sf.Type)
		}

		if !isRecursableElement(f.Element) {
			continue
		}
		nested := subformStructType(sf.Type)
		if nested != nil && isDeepCheckable(nested) {
			compareFieldsToStruct(t, fieldPath, f.SubformOptions, nested)
		}
	}

	// Sort for deterministic output.
	tags := make([]string, 0, len(flat))
	for tag := range flat {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	for _, tag := range tags {
		if !seen[tag] {
			sf := flat[tag]
			t.Errorf("%s: struct field %s (type %s) has no matching schema field", joinPath(path, tag), sf.Name, sf.Type)
		}
	}
}

// flattenConfigFields walks t's fields and returns a map of json tag -> struct field, expanding
// inline/embedded structs into the same namespace (rule 4). receivers.NotifierConfig is skipped
// outright rather than expanded (see the package doc comment above). Fields without a json tag
// (or with a "-" tag) aren't wire-facing and are skipped; fields whose Go name contains "File" or
// "Ref" are excluded per rule 3.
func flattenConfigFields(t reflect.Type) map[string]reflect.StructField {
	out := map[string]reflect.StructField{}
	flattenInto(t, out)
	return out
}

func flattenInto(t reflect.Type, out map[string]reflect.StructField) {
	t = derefType(t)
	if t.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if strings.Contains(f.Name, "File") || strings.Contains(f.Name, "Ref") {
			continue
		}
		if f.Anonymous {
			if derefType(f.Type) == notifierConfigType {
				continue
			}
			flattenInto(f.Type, out)
			continue
		}
		tag, ok := jsonTag(f)
		if !ok {
			continue
		}
		out[tag] = f
	}
}

func jsonTag(f reflect.StructField) (string, bool) {
	tag, ok := f.Tag.Lookup("json")
	if !ok || tag == "-" {
		return "", false
	}
	name := strings.Split(tag, ",")[0]
	if name == "" {
		name = f.Name
	}
	return name, true
}

func derefType(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

func isSecretGoType(t reflect.Type) bool {
	t = derefType(t)
	return t == secretType || t == secretURLType
}

func isPlausibleSecretHolder(t reflect.Type) bool {
	t = derefType(t)
	return isSecretGoType(t) || t.Kind() == reflect.String
}

func isRecursableElement(e schema.ElementType) bool {
	return e == schema.ElementTypeSubform || e == schema.ElementSubformArray
}

// subformStructType unwraps pointers/slices/arrays down to the struct type a subform (or
// subform-array) field is backed by, or nil if it isn't backed by a struct at all.
func subformStructType(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}
	return t
}

func isDeepCheckable(t reflect.Type) bool {
	return strings.HasPrefix(t.PkgPath(), receiversPkgPrefix)
}

func joinPath(path, segment string) string {
	if path == "" {
		return segment
	}
	return path + "." + segment
}
