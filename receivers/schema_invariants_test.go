package receivers_test

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/receivers/schema"

	"github.com/grafana/alerting/receivers/dingding"
	dingdingv1 "github.com/grafana/alerting/receivers/dingding/v1"

	"github.com/grafana/alerting/receivers/discord"
	discordv0mimir1 "github.com/grafana/alerting/receivers/discord/v0mimir1"
	discordv1 "github.com/grafana/alerting/receivers/discord/v1"

	"github.com/grafana/alerting/receivers/email"
	emailv0mimir1 "github.com/grafana/alerting/receivers/email/v0mimir1"

	"github.com/grafana/alerting/receivers/googlechat"
	googlechatv1 "github.com/grafana/alerting/receivers/googlechat/v1"

	"github.com/grafana/alerting/receivers/jira"
	jirav0mimir1 "github.com/grafana/alerting/receivers/jira/v0mimir1"

	"github.com/grafana/alerting/receivers/kafka"
	kafkav1 "github.com/grafana/alerting/receivers/kafka/v1"

	"github.com/grafana/alerting/receivers/line"
	linev1 "github.com/grafana/alerting/receivers/line/v1"

	"github.com/grafana/alerting/receivers/mqtt"
	mqttv1 "github.com/grafana/alerting/receivers/mqtt/v1"

	"github.com/grafana/alerting/receivers/opsgenie"
	opsgeniev0mimir1 "github.com/grafana/alerting/receivers/opsgenie/v0mimir1"

	"github.com/grafana/alerting/receivers/pagerduty"
	pagerdutyv0mimir1 "github.com/grafana/alerting/receivers/pagerduty/v0mimir1"
	pagerdutyv1 "github.com/grafana/alerting/receivers/pagerduty/v1"

	"github.com/grafana/alerting/receivers/pushover"
	pushoverv0mimir1 "github.com/grafana/alerting/receivers/pushover/v0mimir1"

	"github.com/grafana/alerting/receivers/sensugo"
	sensugov1 "github.com/grafana/alerting/receivers/sensugo/v1"

	"github.com/grafana/alerting/receivers/slack"
	slackv0mimir1 "github.com/grafana/alerting/receivers/slack/v0mimir1"
	slackv1 "github.com/grafana/alerting/receivers/slack/v1"

	"github.com/grafana/alerting/receivers/sns"
	snsv0mimir1 "github.com/grafana/alerting/receivers/sns/v0mimir1"
	snsv1 "github.com/grafana/alerting/receivers/sns/v1"

	"github.com/grafana/alerting/receivers/teams"
	teamsv0mimir1 "github.com/grafana/alerting/receivers/teams/v0mimir1"
	teamsv0mimir2 "github.com/grafana/alerting/receivers/teams/v0mimir2"
	teamsv1 "github.com/grafana/alerting/receivers/teams/v1"

	"github.com/grafana/alerting/receivers/telegram"
	telegramv0mimir1 "github.com/grafana/alerting/receivers/telegram/v0mimir1"
	telegramv1 "github.com/grafana/alerting/receivers/telegram/v1"

	"github.com/grafana/alerting/receivers/threema"
	threemav1 "github.com/grafana/alerting/receivers/threema/v1"

	"github.com/grafana/alerting/receivers/victorops"
	victoropsv0mimir1 "github.com/grafana/alerting/receivers/victorops/v0mimir1"
	victoropsv1 "github.com/grafana/alerting/receivers/victorops/v1"

	"github.com/grafana/alerting/receivers/webex"
	webexv0mimir1 "github.com/grafana/alerting/receivers/webex/v0mimir1"
	webexv1 "github.com/grafana/alerting/receivers/webex/v1"

	"github.com/grafana/alerting/receivers/webhook"
	webhookv0mimir1 "github.com/grafana/alerting/receivers/webhook/v0mimir1"

	"github.com/grafana/alerting/receivers/wechat"
	wechatv0mimir1 "github.com/grafana/alerting/receivers/wechat/v0mimir1"

	"github.com/grafana/alerting/receivers/wecom"
	wecomv1 "github.com/grafana/alerting/receivers/wecom/v1"
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
// Not covered here: alertmanager/v1, email/v1, jira/v1, oncall/v1, opsgenie/v1, pushover/v1 and
// webhook/v1. Their exported Config struct carries no JSON tags at all - NewConfig unmarshals into
// an unexported, function-local "raw"/"rawSettings" struct that carries the real wire tags and then
// copies the values across by hand. That local type isn't reachable via reflection from outside the
// function, so these can't be mechanically checked without changing production code (out of scope
// for this test). See the individual NewConfig functions for the actual wire format.

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

// schemaCase pairs one version of an integration's schema with the Go Config struct that backs it.
// The schema/factory registry (receivers.Manifest/IntegrationVersionFactory) only carries
// json.RawMessage-based factory functions, not the concrete Config type, so there's no way to
// derive this table from the registry - it has to be listed explicitly.
type schemaCase struct {
	integration string
	version     schema.Version
	fields      []schema.Field
	configType  reflect.Type
}

func newSchemaCase(t *testing.T, integration string, s schema.IntegrationTypeSchema, version schema.Version, cfg any) schemaCase {
	t.Helper()
	v, ok := s.GetVersion(version)
	require.True(t, ok, "%s: version %s not found in its IntegrationTypeSchema", integration, version)
	return schemaCase{
		integration: integration,
		version:     version,
		fields:      v.Options,
		configType:  reflect.TypeOf(cfg),
	}
}

// violation is one mismatch between a schema field and its config struct.
type violation struct {
	integration string
	version     schema.Version
	path        string
	kind        string
	detail      string
}

func (v violation) key() string {
	return fmt.Sprintf("%s/%s/%s:%s", v.integration, v.version, v.path, v.kind)
}

const (
	kindSchemaFieldMissingStruct = "schema_field_missing_struct_field"
	kindStructFieldMissingSchema = "struct_field_missing_schema_field"
	kindSecretTypeNotSecure      = "secret_type_not_marked_secure"
	kindSecureFieldNotSecretLike = "secure_field_not_secret_like"
)

// knownViolations is a documented allow-list of pre-existing mismatches. Fixing them would mean
// changing an integration's config struct or schema, which is out of scope for this test - it only
// enforces the invariant mechanically and reports violations. Any violation NOT in this list still
// fails the suite. If an entry here stops occurring, the test also fails, so the list can't drift.
var knownViolations = map[string]string{
	// jira/v0mimir1: the "Fields" struct field's JSON tag is "custom_fields", but its YAML tag (and
	// the schema PropertyName) is "fields". Per CLAUDE.md, PropertyName must track the JSON tag.
	"jira/v0mimir1/custom_fields:" + kindStructFieldMissingSchema: "jira/v0mimir1 Config.Fields has json tag \"custom_fields\" but yaml tag/PropertyName \"fields\"",
	"jira/v0mimir1/fields:" + kindSchemaFieldMissingStruct:        "jira/v0mimir1 schema PropertyName \"fields\" matches Config.Fields' yaml tag, not its json tag \"custom_fields\"",

	// telegram/v0mimir1: same class of bug - "ChatID"'s JSON tag is "chat", but its YAML tag (and
	// the schema PropertyName) is "chat_id".
	"telegram/v0mimir1/chat:" + kindStructFieldMissingSchema:    "telegram/v0mimir1 Config.ChatID has json tag \"chat\" but yaml tag/PropertyName \"chat_id\"",
	"telegram/v0mimir1/chat_id:" + kindSchemaFieldMissingStruct: "telegram/v0mimir1 schema PropertyName \"chat_id\" matches Config.ChatID's yaml tag, not its json tag \"chat\"",

	// pagerduty/v0mimir1: the "images" subform's "source" field doesn't match PagerdutyImage's "src"
	// json tag - looks like a typo (PagerdutyImage has Src/Alt/Href, not Source).
	"pagerduty/v0mimir1/images.src:" + kindStructFieldMissingSchema:    "pagerduty/v0mimir1 PagerdutyImage.Src (json tag \"src\") has no matching schema field",
	"pagerduty/v0mimir1/images.source:" + kindSchemaFieldMissingStruct: "pagerduty/v0mimir1 images subform PropertyName \"source\" doesn't match PagerdutyImage's \"src\" json tag",

	// wecom/v1: Config.EndpointURL (json tag "endpointUrl") is read from settings by NewConfig (it
	// overrides the default WeCom API endpoint) but isn't exposed as a schema field, so there's no
	// supported way to set it through the UI.
	"wecom/v1/endpointUrl:" + kindStructFieldMissingSchema: "wecom/v1 Config.EndpointURL is read from settings in NewConfig but has no schema field",
}

func TestIntegrationSchemasMatchConfigStructs(t *testing.T) {
	cases := []schemaCase{
		newSchemaCase(t, "dingding", dingding.Schema, dingdingv1.Version, dingdingv1.Config{}),

		newSchemaCase(t, "discord", discord.Schema, discordv0mimir1.Version, discordv0mimir1.Config{}),
		newSchemaCase(t, "discord", discord.Schema, discordv1.Version, discordv1.Config{}),

		newSchemaCase(t, "email", email.Schema, emailv0mimir1.Version, emailv0mimir1.Config{}),

		newSchemaCase(t, "googlechat", googlechat.Schema, googlechatv1.Version, googlechatv1.Config{}),

		newSchemaCase(t, "jira", jira.Schema, jirav0mimir1.Version, jirav0mimir1.Config{}),

		newSchemaCase(t, "kafka", kafka.Schema, kafkav1.Version, kafkav1.Config{}),

		newSchemaCase(t, "line", line.Schema, linev1.Version, linev1.Config{}),

		newSchemaCase(t, "mqtt", mqtt.Schema, mqttv1.Version, mqttv1.Config{}),

		newSchemaCase(t, "opsgenie", opsgenie.Schema, opsgeniev0mimir1.Version, opsgeniev0mimir1.Config{}),

		newSchemaCase(t, "pagerduty", pagerduty.Schema, pagerdutyv0mimir1.Version, pagerdutyv0mimir1.Config{}),
		newSchemaCase(t, "pagerduty", pagerduty.Schema, pagerdutyv1.Version, pagerdutyv1.Config{}),

		newSchemaCase(t, "pushover", pushover.Schema, pushoverv0mimir1.Version, pushoverv0mimir1.Config{}),

		newSchemaCase(t, "sensugo", sensugo.Schema, sensugov1.Version, sensugov1.Config{}),

		newSchemaCase(t, "slack", slack.Schema, slackv0mimir1.Version, slackv0mimir1.Config{}),
		newSchemaCase(t, "slack", slack.Schema, slackv1.Version, slackv1.Config{}),

		newSchemaCase(t, "sns", sns.Schema, snsv0mimir1.Version, snsv0mimir1.Config{}),
		newSchemaCase(t, "sns", sns.Schema, snsv1.Version, snsv1.Config{}),

		newSchemaCase(t, "teams", teams.Schema, teamsv0mimir1.Version, teamsv0mimir1.Config{}),
		newSchemaCase(t, "teams", teams.Schema, teamsv0mimir2.Version, teamsv0mimir2.Config{}),
		newSchemaCase(t, "teams", teams.Schema, teamsv1.Version, teamsv1.Config{}),

		newSchemaCase(t, "telegram", telegram.Schema, telegramv0mimir1.Version, telegramv0mimir1.Config{}),
		newSchemaCase(t, "telegram", telegram.Schema, telegramv1.Version, telegramv1.Config{}),

		newSchemaCase(t, "threema", threema.Schema, threemav1.Version, threemav1.Config{}),

		newSchemaCase(t, "victorops", victorops.Schema, victoropsv0mimir1.Version, victoropsv0mimir1.Config{}),
		newSchemaCase(t, "victorops", victorops.Schema, victoropsv1.Version, victoropsv1.Config{}),

		newSchemaCase(t, "webex", webex.Schema, webexv0mimir1.Version, webexv0mimir1.Config{}),
		newSchemaCase(t, "webex", webex.Schema, webexv1.Version, webexv1.Config{}),

		newSchemaCase(t, "webhook", webhook.Schema, webhookv0mimir1.Version, webhookv0mimir1.Config{}),

		newSchemaCase(t, "wechat", wechat.Schema, wechatv0mimir1.Version, wechatv0mimir1.Config{}),

		newSchemaCase(t, "wecom", wecom.Schema, wecomv1.Version, wecomv1.Config{}),
	}

	var violations []violation
	for _, c := range cases {
		compareFieldsToStruct(c.integration, c.version, "", c.fields, c.configType, &violations)
	}

	unexpected := make([]violation, 0, len(violations))
	seenKnown := map[string]bool{}
	for _, v := range violations {
		if _, ok := knownViolations[v.key()]; ok {
			seenKnown[v.key()] = true
			continue
		}
		unexpected = append(unexpected, v)
	}

	// A knownViolations entry that no longer reproduces means the underlying bug was fixed - the
	// allow-list must be trimmed so it can't silently mask a regression later.
	var stale []string
	for key := range knownViolations {
		if !seenKnown[key] {
			stale = append(stale, key)
		}
	}
	sort.Strings(stale)

	if len(unexpected) == 0 && len(stale) == 0 {
		return
	}

	sort.Slice(unexpected, func(i, j int) bool { return unexpected[i].key() < unexpected[j].key() })

	var sb strings.Builder
	if len(unexpected) > 0 {
		fmt.Fprintf(&sb, "%d unexpected schema/config invariant violation(s):\n", len(unexpected))
		for _, v := range unexpected {
			fmt.Fprintf(&sb, "  [%s] %s %s: %s\n", v.kind, v.integration, v.version, v.detail)
		}
	}
	if len(stale) > 0 {
		fmt.Fprintf(&sb, "%d stale knownViolations entry/entries (no longer reproduces - remove from the allow-list):\n", len(stale))
		for _, key := range stale {
			fmt.Fprintf(&sb, "  %s: %s\n", key, knownViolations[key])
		}
	}
	t.Error(sb.String())
}

// compareFieldsToStruct compares one schema version's Options against the flattened fields of its
// backing config struct t, recording every mismatch into violations. path is the dotted location of
// fields/t within the integration (empty for the top-level struct, "http_config" etc. for subforms).
func compareFieldsToStruct(integration string, version schema.Version, path string, fields []schema.Field, t reflect.Type, violations *[]violation) {
	flat := flattenConfigFields(t)

	seen := make(map[string]bool, len(fields))
	for _, f := range fields {
		seen[f.PropertyName] = true

		sf, ok := flat[f.PropertyName]
		if !ok {
			*violations = append(*violations, violation{
				integration: integration,
				version:     version,
				path:        joinPath(path, f.PropertyName),
				kind:        kindSchemaFieldMissingStruct,
				detail:      fmt.Sprintf("schema field %q has no struct field with a matching json tag in %s", f.PropertyName, t),
			})
			continue
		}

		checkSecretInvariant(integration, version, joinPath(path, f.PropertyName), f, sf, violations)

		if !isRecursableElement(f.Element) {
			continue
		}
		nested := subformStructType(sf.Type)
		if nested == nil || !isDeepCheckable(nested) {
			continue
		}
		compareFieldsToStruct(integration, version, joinPath(path, f.PropertyName), f.SubformOptions, nested, violations)
	}

	// Sort for deterministic output.
	tags := make([]string, 0, len(flat))
	for tag := range flat {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	for _, tag := range tags {
		if seen[tag] {
			continue
		}
		sf := flat[tag]
		*violations = append(*violations, violation{
			integration: integration,
			version:     version,
			path:        joinPath(path, tag),
			kind:        kindStructFieldMissingSchema,
			detail:      fmt.Sprintf("struct field %s (json tag %q, type %s) has no matching schema field", sf.Name, tag, sf.Type),
		})
	}
}

// checkSecretInvariant enforces rule 2: a Secret/SecretURL-typed struct field must be Secure in the
// schema, and a Secure schema field must map to a struct field that's actually capable of holding a
// secret. Plain strings are accepted on the Secure side because several receiver generations (v1
// receivers, and shared types like receivers.TLSConfig) decrypt secrets into a plain string field
// rather than a receivers.Secret/SecretURL - only the legacy v0mimir1/v0mimir2 receivers use the
// latter. A struct/slice/bool/etc. field marked Secure would be a real bug; a string one is not.
func checkSecretInvariant(integration string, version schema.Version, path string, f schema.Field, sf reflect.StructField, violations *[]violation) {
	if isSecretGoType(sf.Type) && !f.Secure {
		*violations = append(*violations, violation{
			integration: integration,
			version:     version,
			path:        path,
			kind:        kindSecretTypeNotSecure,
			detail:      fmt.Sprintf("struct field %s has type %s but schema field %q is not marked Secure", sf.Name, sf.Type, f.PropertyName),
		})
	}
	if f.Secure && !isPlausibleSecretHolder(sf.Type) {
		*violations = append(*violations, violation{
			integration: integration,
			version:     version,
			path:        path,
			kind:        kindSecureFieldNotSecretLike,
			detail:      fmt.Sprintf("schema field %q is marked Secure but struct field %s has type %s, which can't hold a secret", f.PropertyName, sf.Name, sf.Type),
		})
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
