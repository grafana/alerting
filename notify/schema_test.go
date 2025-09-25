package notify

import (
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/notify/notifytest"
	"github.com/grafana/alerting/receivers/alertmanager"
	"github.com/grafana/alerting/receivers/dingding"
	"github.com/grafana/alerting/receivers/discord"
	"github.com/grafana/alerting/receivers/email"
	"github.com/grafana/alerting/receivers/googlechat"
	"github.com/grafana/alerting/receivers/jira"
	"github.com/grafana/alerting/receivers/kafka"
	"github.com/grafana/alerting/receivers/line"
	"github.com/grafana/alerting/receivers/mqtt"
	"github.com/grafana/alerting/receivers/oncall"
	"github.com/grafana/alerting/receivers/opsgenie"
	"github.com/grafana/alerting/receivers/pagerduty"
	"github.com/grafana/alerting/receivers/pushover"
	"github.com/grafana/alerting/receivers/schema"
	"github.com/grafana/alerting/receivers/sensugo"
	"github.com/grafana/alerting/receivers/slack"
	"github.com/grafana/alerting/receivers/sns"
	"github.com/grafana/alerting/receivers/teams"
	"github.com/grafana/alerting/receivers/telegram"
	"github.com/grafana/alerting/receivers/threema"
	"github.com/grafana/alerting/receivers/victorops"
	"github.com/grafana/alerting/receivers/webex"
	"github.com/grafana/alerting/receivers/webhook"
	"github.com/grafana/alerting/receivers/wechat"
	"github.com/grafana/alerting/receivers/wecom"
)

func TestGetSecretKeysForContactPointType(t *testing.T) {
	httpConfigSecrets := []string{"http_config.authorization.credentials", "http_config.basic_auth.password", "http_config.oauth2.client_secret"}
	testCases := []struct {
		receiverType         schema.IntegrationType
		version              schema.Version
		expectedSecretFields []string
	}{
		{receiverType: dingding.Type, version: schema.V1, expectedSecretFields: []string{"url"}},
		{receiverType: kafka.Type, version: schema.V1, expectedSecretFields: []string{"password"}},
		{receiverType: email.Type, version: schema.V1, expectedSecretFields: []string{}},
		{receiverType: pagerduty.Type, version: schema.V1, expectedSecretFields: []string{"integrationKey"}},
		{receiverType: victorops.Type, version: schema.V1, expectedSecretFields: []string{"url"}},
		{receiverType: oncall.Type, version: schema.V1, expectedSecretFields: []string{"password", "authorization_credentials"}},
		{receiverType: pushover.Type, version: schema.V1, expectedSecretFields: []string{"apiToken", "userKey"}},
		{receiverType: slack.Type, version: schema.V1, expectedSecretFields: []string{"token", "url"}},
		{receiverType: sensugo.Type, version: schema.V1, expectedSecretFields: []string{"apikey"}},
		{receiverType: teams.Type, version: schema.V1, expectedSecretFields: []string{}},
		{receiverType: telegram.Type, version: schema.V1, expectedSecretFields: []string{"bottoken"}},
		{receiverType: webhook.Type, version: schema.V1, expectedSecretFields: []string{
			"password",
			"authorization_credentials",
			"tlsConfig.caCertificate",
			"tlsConfig.clientCertificate",
			"tlsConfig.clientKey",
			"hmacConfig.secret",
			"http_config.oauth2.client_secret",
			"http_config.oauth2.tls_config.caCertificate",
			"http_config.oauth2.tls_config.clientCertificate",
			"http_config.oauth2.tls_config.clientKey",
		}},
		{receiverType: wecom.Type, version: schema.V1, expectedSecretFields: []string{"url", "secret"}},
		{receiverType: alertmanager.Type, version: schema.V1, expectedSecretFields: []string{"basicAuthPassword"}},
		{receiverType: discord.Type, version: schema.V1, expectedSecretFields: []string{"url"}},
		{receiverType: googlechat.Type, version: schema.V1, expectedSecretFields: []string{"url"}},
		{receiverType: line.Type, version: schema.V1, expectedSecretFields: []string{"token"}},
		{receiverType: threema.Type, version: schema.V1, expectedSecretFields: []string{"api_secret"}},
		{receiverType: opsgenie.Type, version: schema.V1, expectedSecretFields: []string{"apiKey"}},
		{receiverType: webex.Type, version: schema.V1, expectedSecretFields: []string{"bot_token"}},
		{receiverType: sns.Type, version: schema.V1, expectedSecretFields: []string{"sigv4.access_key", "sigv4.secret_key"}},
		{receiverType: mqtt.Type, version: schema.V1, expectedSecretFields: []string{"password", "tlsConfig.caCertificate", "tlsConfig.clientCertificate", "tlsConfig.clientKey"}},
		{receiverType: jira.Type, version: schema.V1, expectedSecretFields: []string{"user", "password", "api_token"}},
		{receiverType: victorops.Type, version: schema.V0mimir1, expectedSecretFields: append([]string{"api_key"}, httpConfigSecrets...)},
		{receiverType: sns.Type, version: schema.V0mimir1, expectedSecretFields: append([]string{"sigv4.SecretKey"}, httpConfigSecrets...)},
		{receiverType: telegram.Type, version: schema.V0mimir1, expectedSecretFields: append([]string{"token"}, httpConfigSecrets...)},
		{receiverType: discord.Type, version: schema.V0mimir1, expectedSecretFields: append([]string{"webhook_url"}, httpConfigSecrets...)},
		{receiverType: pagerduty.Type, version: schema.V0mimir1, expectedSecretFields: append([]string{"routing_key", "service_key"}, httpConfigSecrets...)},
		{receiverType: pushover.Type, version: schema.V0mimir1, expectedSecretFields: append([]string{"user_key", "token"}, httpConfigSecrets...)},
		{receiverType: jira.Type, version: schema.V0mimir1, expectedSecretFields: httpConfigSecrets},
		{receiverType: opsgenie.Type, version: schema.V0mimir1, expectedSecretFields: append([]string{"api_key"}, httpConfigSecrets...)},
		{receiverType: teams.Type, version: schema.V0mimir1, expectedSecretFields: append([]string{"webhook_url"}, httpConfigSecrets...)},
		{receiverType: teams.Type, version: schema.V0mimir2, expectedSecretFields: append([]string{"webhook_url"}, httpConfigSecrets...)},
		{receiverType: email.Type, version: schema.V0mimir1, expectedSecretFields: []string{"auth_password", "auth_secret"}},
		{receiverType: slack.Type, version: schema.V0mimir1, expectedSecretFields: append([]string{"api_url"}, httpConfigSecrets...)},
		{receiverType: webex.Type, version: schema.V0mimir1, expectedSecretFields: httpConfigSecrets},
		{receiverType: wechat.Type, version: schema.V0mimir1, expectedSecretFields: append([]string{"api_secret"}, httpConfigSecrets...)},
		{receiverType: webhook.Type, version: schema.V0mimir1, expectedSecretFields: append([]string{"url"}, httpConfigSecrets...)},
	}
	n := GetSchemaForAllIntegrations()
	type typeWithVersion struct {
		Type    schema.IntegrationType
		Version schema.Version
	}
	allTypes := make(map[typeWithVersion]struct{}, len(n))
	getKey := func(pluginType schema.IntegrationType, version schema.Version) typeWithVersion {
		return typeWithVersion{pluginType, version}
	}
	for _, p := range n {
		for _, v := range p.Versions {
			allTypes[getKey(p.Type, v.Version)] = struct{}{}
		}
	}

	for _, testCase := range testCases {
		delete(allTypes, getKey(testCase.receiverType, testCase.version))
		t.Run(fmt.Sprintf("%s-%s", testCase.receiverType, testCase.version), func(t *testing.T) {
			s, _ := GetSchemaForIntegration(testCase.receiverType)
			v, _ := s.GetVersion(testCase.version)
			got := v.GetSecretFieldsPaths()
			require.ElementsMatch(t, testCase.expectedSecretFields, got)
		})
	}

	for it := range allTypes {
		t.Run(fmt.Sprintf("%s-%s", it.Type, it.Version), func(t *testing.T) {
			s, _ := GetSchemaForIntegration(it.Type)
			v, _ := s.GetVersion(it.Version)
			got := v.GetSecretFieldsPaths()
			require.Emptyf(t, got, "secret keys for version %s of %s should be empty", it.Version, it.Type)
		})
	}

	require.Emptyf(t, allTypes, "not all types are covered: %s", allTypes)
}

func TestGetAvailableNotifiersV2(t *testing.T) {
	n := GetSchemaForAllIntegrations()
	require.NotEmpty(t, n)
	for _, notifier := range n {
		t.Run(fmt.Sprintf("integration %s [%s]", notifier.Type, notifier.Name), func(t *testing.T) {
			currentVersion := schema.V1
			if notifier.Type == "wechat" {
				currentVersion = schema.V0mimir1
			}
			t.Run(fmt.Sprintf("current version is %s", currentVersion), func(t *testing.T) {
				require.Equal(t, currentVersion, notifier.GetCurrentVersion().Version)
			})
			t.Run("should be able to create only v1", func(t *testing.T) {
				for _, version := range notifier.Versions {
					if version.Version == schema.V1 {
						require.True(t, version.CanCreate, "v1 should be able to create")
						continue
					}
					require.False(t, version.CanCreate, "v0 should not be able to create")
				}
			})
		})
	}
}

func TestGetSchemaForIntegration(t *testing.T) {
	t.Run("should return integration schema by type", func(t *testing.T) {
		for _, expected := range GetSchemaForAllIntegrations() {
			actual, ok := GetSchemaForIntegration(expected.Type)
			require.Truef(t, ok, "expected config but got error for plugin type %s", actual.Type)
			assert.Equal(t, expected, actual)
		}
	})

	t.Run("should return integration schema by type alias", func(t *testing.T) {
		for _, plugin := range GetSchemaForAllIntegrations() {
			for _, version := range plugin.Versions {
				if version.TypeAlias == "" {
					continue
				}
				t.Run(string(version.TypeAlias), func(t *testing.T) {
					plugin2, ok := GetSchemaForIntegration(version.TypeAlias)
					require.Truef(t, ok, "expected config but got error for plugin type %s", plugin.Type)
					assert.Equal(t, plugin, plugin2)
				})
			}
		}
	})

	t.Run("should not return if unknown type", func(t *testing.T) {
		v, ok := GetSchemaForIntegration("unknown")
		require.Falsef(t, ok, "returned an unexpected unknown type %v", v)
	})
}

func TestGetSchemaVersionForIntegration(t *testing.T) {
	t.Run("should return specific version of integration schema", func(t *testing.T) {
		for _, typeSchema := range GetSchemaForAllIntegrations() {
			for _, expected := range typeSchema.Versions {
				actual, ok := GetSchemaVersionForIntegration(typeSchema.Type, expected.Version)
				require.Truef(t, ok, "version %s of type %s not found", expected.Version, typeSchema.Type)
				assert.Equal(t, expected, actual)
				if expected.TypeAlias == "" {
					continue
				}
				actual, ok = GetSchemaVersionForIntegration(expected.TypeAlias, expected.Version)
				require.Truef(t, ok, "version %s of type alias %s not found", expected.Version, typeSchema.Type)
				assert.Equal(t, expected, actual)
			}
		}
	})

	t.Run("should not return if unknown type", func(t *testing.T) {
		v, ok := GetSchemaVersionForIntegration("unknown", schema.V0mimir1)
		require.Falsef(t, ok, "returned an unexpected unknown version %v", v)
	})

	t.Run("should not return if version does not exist for type", func(t *testing.T) {
		v, ok := GetSchemaVersionForIntegration(alertmanager.Type, schema.V0mimir1)
		require.Falsef(t, ok, "returned an unexpected unknown version %v", v)
	})

	t.Run("should return correct version for type alias", func(t *testing.T) {
		for _, plugin := range GetSchemaForAllIntegrations() {
			for _, version := range plugin.Versions {
				if version.TypeAlias == "" {
					continue
				}
				for _, other := range plugin.Versions {
					if version.Version == other.Version {
						continue
					}
					actual, ok := GetSchemaVersionForIntegration(version.TypeAlias, other.Version)
					require.Truef(t, ok, "version %s of type alias %s not found", version.Version, version.TypeAlias)
					assert.Equal(t, other, actual)
				}
			}
		}
	})
}

func TestSchemaTypeUniqueness(t *testing.T) {
	knownTypes := make(map[string]struct{})
	for _, plugin := range GetSchemaForAllIntegrations() {
		iType := strings.ToLower(string(plugin.Type))
		if _, ok := knownTypes[iType]; ok {
			assert.Failf(t, "duplicate plugin type", "plugin type %s", plugin.Type)
		}
		knownTypes[iType] = struct{}{}
		for _, version := range plugin.Versions {
			if version.TypeAlias == "" {
				continue
			}
			iType = strings.ToLower(string(version.TypeAlias))
			if _, ok := knownTypes[iType]; ok {
				assert.Failf(t, "mimir type duplicates Grafana plugin type", "plugin type %s", iType)
			}
			knownTypes[iType] = struct{}{}
		}
	}
}

func TestV0IntegrationsSecrets(t *testing.T) {
	// This test ensures that all known integrations' secrets are listed in the schema definition.
	notifytest.ForEachIntegrationType(t, func(configType reflect.Type) {
		t.Run(configType.Name(), func(t *testing.T) {
			integrationType := schema.IntegrationType(strings.ToLower(strings.TrimSuffix(configType.Name(), "Config")))
			iSchema, ok := GetSchemaForIntegration(integrationType)
			require.Truef(t, ok, "schema for %s not found", integrationType)
			var version schema.IntegrationSchemaVersion
			if iSchema.Type != integrationType {
				version, ok = iSchema.GetVersionByTypeAlias(integrationType)
				require.Truef(t, ok, "version for type alias %s not found", integrationType)
			} else {
				version, ok = iSchema.GetVersion(schema.V0mimir1)
				require.Truef(t, ok, "mimir version for %s not found", integrationType)
			}
			expectedSecrets := version.GetSecretFieldsPaths()
			var secrets []string
			for option := range maps.Keys(notifytest.ValidMimirHTTPConfigs) {
				cfg, err := notifytest.GetMimirIntegrationForType(configType, option)
				require.NoError(t, err)
				data, err := json.Marshal(cfg)
				require.NoError(t, err)
				m := map[string]any{}
				err = json.Unmarshal(data, &m)
				require.NoError(t, err)
				secrets = append(secrets, getSecrets(m, "")...)
			}
			secrets = unique(secrets)
			t.Log(secrets)
			require.ElementsMatch(t, expectedSecrets, secrets)
		})
	})
}

func unique(slice []string) []string {
	keys := make(map[string]struct{}, len(slice))
	list := make([]string, 0, len(slice))
	for _, entry := range slice {
		if _, value := keys[entry]; !value {
			keys[entry] = struct{}{}
			list = append(list, entry)
		}
	}
	return list
}

func getSecrets(m map[string]any, parent string) []string {
	var result []string
	for key, val := range m {
		str, ok := val.(string)
		if ok && str == "<secret>" {
			result = append(result, parent+key)
		}
		m, ok := val.(map[string]any)
		if ok {
			subSecrets := getSecrets(m, parent+key+".")
			result = append(result, subSecrets...)
		}
	}
	return result
}
