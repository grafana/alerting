package notify

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/prometheus/alertmanager/notify"

	alertingHttp "github.com/grafana/alerting/http"
	"github.com/grafana/alerting/images"
	"github.com/grafana/alerting/receivers"
	line "github.com/grafana/alerting/receivers/line/v1"
	pushover "github.com/grafana/alerting/receivers/pushover/v1"
	telegram "github.com/grafana/alerting/receivers/telegram/v1"
	receiversTesting "github.com/grafana/alerting/receivers/testing"
	threema "github.com/grafana/alerting/receivers/threema/v1"
	"github.com/grafana/alerting/templates"
)

func TestReceiverTimeoutError_Error(t *testing.T) {
	e := IntegrationTimeoutError{
		Integration: &GrafanaIntegrationConfig{
			Name: "test",
			UID:  "uid",
		},
		Err: errors.New("context deadline exceeded"),
	}
	require.Equal(t, "the receiver timed out: context deadline exceeded", e.Error())
}

type timeoutError struct{}

func (e timeoutError) Error() string {
	return "the request timed out"
}

func (e timeoutError) Timeout() bool {
	return true
}

func TestProcessNotifierError(t *testing.T) {
	t.Run("assert IntegrationTimeoutError is returned for context deadline exceeded", func(t *testing.T) {
		r := &GrafanaIntegrationConfig{
			Name: "test",
			UID:  "uid",
		}
		require.Equal(t, IntegrationTimeoutError{
			Integration: r,
			Err:         context.DeadlineExceeded,
		}, ProcessIntegrationError(r, context.DeadlineExceeded))
	})

	t.Run("assert IntegrationTimeoutError is returned for *url.Error timeout", func(t *testing.T) {
		r := &GrafanaIntegrationConfig{
			Name: "test",
			UID:  "uid",
		}
		urlError := &url.Error{
			Op:  "Get",
			URL: "https://grafana.net",
			Err: timeoutError{},
		}
		require.Equal(t, IntegrationTimeoutError{
			Integration: r,
			Err:         urlError,
		}, ProcessIntegrationError(r, urlError))
	})

	t.Run("assert unknown error is returned unmodified", func(t *testing.T) {
		r := &GrafanaIntegrationConfig{
			Name: "test",
			UID:  "uid",
		}
		err := errors.New("this is an error")
		require.Equal(t, err, ProcessIntegrationError(r, err))
	})
}

func TestBuildReceiverConfiguration(t *testing.T) {
	decrypt := GetDecryptedValueFnForTesting
	t.Run("AllKnownConfigsForTesting contains all notifier types", func(t *testing.T) {
		// Sanity check to ensure this fails when not all notifier types are present in the configuration.
		// If this doesn't pass, other tests that rely on this function will not be reliable.
		_, missing := allReceivers(&GrafanaReceiverConfig{})
		require.Greaterf(t, missing, 0, "all notifier types should be missing, allReceivers may no longer be reliable, missing: %d", missing)

		recCfg := &APIReceiver{ConfigReceiver: ConfigReceiver{Name: "test-receiver"}}
		for notifierType, cfg := range AllKnownConfigsForTesting {
			recCfg.Integrations = append(recCfg.Integrations, cfg.GetRawNotifierConfig(notifierType))
		}
		parsed, err := BuildReceiverConfiguration(context.Background(), recCfg, DecodeSecretsFromBase64, decrypt)
		require.NoError(t, err)
		_, missing = allReceivers(&parsed)
		require.Equalf(t, 0, missing, "all notifier types should be present, missing: %d", missing)
	})
	t.Run("should decode secrets from base64", func(t *testing.T) {
		recCfg := &APIReceiver{ConfigReceiver: ConfigReceiver{Name: "test-receiver"}}
		for notifierType, cfg := range AllKnownConfigsForTesting {
			recCfg.Integrations = append(recCfg.Integrations, cfg.GetRawNotifierConfig(notifierType))
		}
		counter := 0
		decryptCount := func(ctx context.Context, sjd map[string][]byte, key string, fallback string) string {
			counter++
			return decrypt(ctx, sjd, key, fallback)
		}
		_, _ = BuildReceiverConfiguration(context.Background(), recCfg, DecodeSecretsFromBase64, decryptCount)
		require.Greater(t, counter, 0)
	})
	t.Run("should fail if at least one config is invalid", func(t *testing.T) {
		recCfg := &APIReceiver{ConfigReceiver: ConfigReceiver{Name: "test-receiver"}}
		for notifierType, cfg := range AllKnownConfigsForTesting {
			recCfg.Integrations = append(recCfg.Integrations, cfg.GetRawNotifierConfig(notifierType))
		}
		bad := &GrafanaIntegrationConfig{
			UID:      "invalid-test",
			Name:     "invalid-test",
			Type:     "slack",
			Settings: json.RawMessage(`{ "test" : "test" }`),
		}
		recCfg.Integrations = append(recCfg.Integrations, bad)

		parsed, err := BuildReceiverConfiguration(context.Background(), recCfg, DecodeSecretsFromBase64, decrypt)
		require.NotNil(t, err)
		require.Equal(t, GrafanaReceiverConfig{}, parsed)
		require.ErrorAs(t, err, &IntegrationValidationError{})
		typedError := err.(IntegrationValidationError)
		require.NotNil(t, typedError.Integration)
		require.Equal(t, bad, typedError.Integration)
		require.ErrorContains(t, err, fmt.Sprintf(`failed to validate integration "%s" (UID %s) of type "%s"`, bad.Name, bad.UID, bad.Type))
	})
	t.Run("should accept empty config", func(t *testing.T) {
		recCfg := &APIReceiver{ConfigReceiver: ConfigReceiver{Name: "test-receiver"}}
		parsed, err := BuildReceiverConfiguration(context.Background(), recCfg, DecodeSecretsFromBase64, decrypt)
		require.NoError(t, err)
		require.Equal(t, recCfg.Name, parsed.Name)
	})
	t.Run("should support non-base64-encoded secrets", func(t *testing.T) {
		recCfg := &APIReceiver{ConfigReceiver: ConfigReceiver{Name: "test-receiver"}}
		// We decode all the secureSettings from base64 and then build then receivers. The test is to ensure that
		// BuildReceiverConfiguration can handle the already decoded secrets correctly.
		for notifierType, cfg := range AllKnownConfigsForTesting {
			notifierRaw := cfg.GetRawNotifierConfig(notifierType)
			if len(notifierRaw.SecureSettings) == 0 {
				continue
			}
			for key := range notifierRaw.SecureSettings {
				decoded, err := base64.StdEncoding.DecodeString(notifierRaw.SecureSettings[key])
				require.NoError(t, err)
				notifierRaw.SecureSettings[key] = string(decoded)
			}
			recCfg.Integrations = append(recCfg.Integrations, notifierRaw)
		}

		parsed, err := BuildReceiverConfiguration(context.Background(), recCfg, NoopDecode, NoopDecrypt)
		require.NoError(t, err)
		require.Equal(t, recCfg.Name, parsed.Name)
		for _, notifier := range recCfg.GrafanaIntegrations.Integrations {
			if notifier.Type == "prometheus-alertmanager" {
				require.Equal(t, notifier.SecureSettings["basicAuthPassword"], parsed.AlertmanagerConfigs[0].Settings.Password)
			}
		}

	})
	t.Run("should fail if notifier type is unknown", func(t *testing.T) {
		recCfg := &APIReceiver{ConfigReceiver: ConfigReceiver{Name: "test-receiver"}}
		for notifierType, cfg := range AllKnownConfigsForTesting {
			recCfg.Integrations = append(recCfg.Integrations, cfg.GetRawNotifierConfig(notifierType))
		}
		bad := &GrafanaIntegrationConfig{
			UID:      "test",
			Name:     "test",
			Type:     fmt.Sprintf("invalid-%d", rand.Uint32()),
			Settings: json.RawMessage(`{ "test" : "test" }`),
		}
		recCfg.Integrations = append(recCfg.Integrations, bad)

		parsed, err := BuildReceiverConfiguration(context.Background(), recCfg, DecodeSecretsFromBase64, decrypt)
		require.NotNil(t, err)
		require.Equal(t, GrafanaReceiverConfig{}, parsed)
		require.ErrorAs(t, err, &IntegrationValidationError{})
		typedError := err.(IntegrationValidationError)
		require.NotNil(t, typedError.Integration)
		require.Equal(t, bad, typedError.Integration)
		require.ErrorContains(t, err, fmt.Sprintf("notifier %s is not supported", bad.Type))
	})
	t.Run("should recognize all known types", func(t *testing.T) {
		recCfg := &APIReceiver{ConfigReceiver: ConfigReceiver{Name: "test-receiver"}}
		for notifierType, cfg := range AllKnownConfigsForTesting {
			recCfg.Integrations = append(recCfg.Integrations, cfg.GetRawNotifierConfig(notifierType))
		}
		parsed, err := BuildReceiverConfiguration(context.Background(), recCfg, DecodeSecretsFromBase64, decrypt)
		require.NoError(t, err)
		require.Equal(t, recCfg.Name, parsed.Name)

		expectedNotifiers := make(map[string]struct{})
		for _, notifier := range recCfg.GrafanaIntegrations.Integrations {
			expectedNotifiers[notifier.Type] = struct{}{}
		}

		// Ensure that one of every notifier is present in the parsed configuration.
		all, _ := allReceivers(&parsed)
		require.Len(t, all, len(AllKnownConfigsForTesting), "mismatch in number of notifiers, expected %d, got %d", len(AllKnownConfigsForTesting), len(all))
		for _, recv := range all {
			if _, ok := expectedNotifiers[recv.Metadata.Type]; ok {
				delete(expectedNotifiers, recv.Metadata.Type)
			} else {
				t.Errorf("unexpected notifier type: %s", recv.Metadata.Type)
			}
		}
		require.Empty(t, expectedNotifiers, "not all expected notifiers were found in the parsed configuration")

		t.Run("should populate metadata", func(t *testing.T) {
			for idx, cfg := range all {
				meta := cfg.Metadata
				require.NotEmptyf(t, meta.Type, "%s notifier (idx: %d) '%s' uid: '%s'.", meta.Type, idx, meta.Name, meta.UID)
				require.NotEmptyf(t, meta.UID, "%s notifier (idx: %d) '%s' uid: '%s'.", meta.Type, idx, meta.Name, meta.UID)
				require.NotEmptyf(t, meta.Name, "%s notifier (idx: %d) '%s' uid: '%s'.", meta.Type, idx, meta.Name, meta.UID)
				var notifierRaw *GrafanaIntegrationConfig
				for _, receiver := range recCfg.Integrations {
					if receiver.Type == meta.Type && receiver.UID == meta.UID && receiver.Name == meta.Name {
						notifierRaw = receiver
						break
					}
				}
				require.NotNilf(t, notifierRaw, "cannot find raw settings for %s notifier '%s' uid: '%s'.", meta.Type, meta.Name, meta.UID)
				require.Equalf(t, notifierRaw.DisableResolveMessage, meta.DisableResolveMessage, "%s notifier '%s' uid: '%s'.", meta.Type, meta.Name, meta.UID)
			}
		})
	})
	t.Run("should recognize type in any case", func(t *testing.T) {
		recCfg := &APIReceiver{ConfigReceiver: ConfigReceiver{Name: "test-receiver"}}
		for notifierType, cfg := range AllKnownConfigsForTesting {
			notifierRaw := cfg.GetRawNotifierConfig(notifierType)
			notifierRaw.Type = strings.ToUpper(notifierRaw.Type)
			recCfg.Integrations = append(recCfg.Integrations, cfg.GetRawNotifierConfig(notifierType))
		}
		parsed, err := BuildReceiverConfiguration(context.Background(), recCfg, DecodeSecretsFromBase64, decrypt)
		require.NoError(t, err)

		expectedNotifiers := make(map[string]struct{})
		for _, notifier := range recCfg.GrafanaIntegrations.Integrations {
			expectedNotifiers[notifier.Type] = struct{}{}
		}

		// Ensure that one of every notifier is present in the parsed configuration.
		all, _ := allReceivers(&parsed)
		require.Len(t, all, len(AllKnownConfigsForTesting), "mismatch in number of notifiers, expected %d, got %d", len(AllKnownConfigsForTesting), len(all))
		for _, recv := range all {
			if _, ok := expectedNotifiers[recv.Metadata.Type]; ok {
				delete(expectedNotifiers, recv.Metadata.Type)
			} else {
				t.Errorf("unexpected notifier type: %s", recv.Metadata.Type)
			}
		}
		require.Empty(t, expectedNotifiers, "not all expected notifiers were found in the parsed configuration")
	})
}

func TestHTTPConfig(t *testing.T) {
	for notifierType, cfg := range AllKnownConfigsForTesting {
		t.Run(notifierType, func(t *testing.T) {
			if notifierType == "email" {
				t.Skip("does not support http_config")
			}

			if notifierType == "slack" ||
				notifierType == "sns" ||
				notifierType == "mqtt" ||
				notifierType == "prometheus-alertmanager" {
				t.Skip("does not yet support http client")
			}

			t.Run("should support building with http_config", func(t *testing.T) {
				config := cfg.GetRawNotifierConfig(notifierType)

				// Config should include http_config if the notifier supports it, but let's sanity check.
				require.Containsf(t, string(config.Settings), "http_config", "notifier %s does not contain http_config", notifierType)

				recCfg := &APIReceiver{
					ConfigReceiver: ConfigReceiver{Name: "test-receiver"},
					GrafanaIntegrations: GrafanaIntegrations{
						[]*GrafanaIntegrationConfig{
							config,
						},
					},
				}

				t.Run("with secureSettings", func(t *testing.T) {
					for key, value := range receiversTesting.ReadSecretsJSONForTesting(FullValidHTTPConfigSecretsForTesting) {
						config.SecureSettings[key] = base64.StdEncoding.EncodeToString(value)
					}

					parsed, err := BuildReceiverConfiguration(context.Background(), recCfg, DecodeSecretsFromBase64, GetDecryptedValueFnForTesting)
					require.NoError(t, err)

					recvs, _ := allReceivers(&parsed)
					require.Len(t, recvs, 1)

					recv := recvs[0]

					expectedHTTPConfig := &alertingHttp.HTTPClientConfig{
						OAuth2: &alertingHttp.OAuth2Config{
							ClientID:     "test-client-id",
							ClientSecret: "test-override-oauth2-secret",
							TokenURL:     "https://localhost/auth/token",
							Scopes:       []string{"scope1", "scope2"},
							EndpointParams: map[string]string{
								"param1": "value1",
								"param2": "value2",
							},
							TLSConfig: &receivers.TLSConfig{
								InsecureSkipVerify: false,
								ClientCertificate:  alertingHttp.TestCertPem,
								ClientKey:          alertingHttp.TestKeyPem,
								CACertificate:      alertingHttp.TestCACert,
							},
							ProxyConfig: &alertingHttp.ProxyConfig{
								ProxyURL:             alertingHttp.MustURL("http://localproxy:8080"),
								NoProxy:              "localhost",
								ProxyFromEnvironment: false,
								ProxyConnectHeader: map[string]string{
									"X-Proxy-Header": "proxy-value",
								},
							},
						},
					}

					require.Equal(t, expectedHTTPConfig, recv.HTTPClientConfig)
				})

				t.Run("without secureSettings", func(t *testing.T) {
					config.SecureSettings = nil
					parsed, err := BuildReceiverConfiguration(context.Background(), recCfg, DecodeSecretsFromBase64, GetDecryptedValueFnForTesting)
					require.NoError(t, err)

					recvs, _ := allReceivers(&parsed)
					require.Len(t, recvs, 1)

					recv := recvs[0]

					expectedHTTPConfig := &alertingHttp.HTTPClientConfig{
						OAuth2: &alertingHttp.OAuth2Config{
							ClientID:     "test-client-id",
							ClientSecret: "test-client-secret",
							TokenURL:     "https://localhost/auth/token",
							Scopes:       []string{"scope1", "scope2"},
							EndpointParams: map[string]string{
								"param1": "value1",
								"param2": "value2",
							},
							TLSConfig: &receivers.TLSConfig{
								InsecureSkipVerify: false,
								ClientCertificate:  alertingHttp.TestCertPem,
								ClientKey:          alertingHttp.TestKeyPem,
								CACertificate:      alertingHttp.TestCACert,
							},
							ProxyConfig: &alertingHttp.ProxyConfig{
								ProxyURL:             alertingHttp.MustURL("http://localproxy:8080"),
								NoProxy:              "localhost",
								ProxyFromEnvironment: false,
								ProxyConnectHeader: map[string]string{
									"X-Proxy-Header": "proxy-value",
								},
							},
						},
					}

					require.Equal(t, expectedHTTPConfig, recv.HTTPClientConfig)
				})
			})
			t.Run("should support notifying with oauth2 authorization", func(t *testing.T) {
				// Simpler, but more direct test that ensures it's working for each notifier with a minimal setup.
				// More comprehensive tests of advanced options are in the http package.
				type oauth2Response struct {
					AccessToken string `json:"access_token"`
					TokenType   string `json:"token_type"`
				}

				oathRequestCnt := 0
				expectedAuthResponse := oauth2Response{
					AccessToken: "12345",
					TokenType:   "Bearer",
				}
				oauthHandler := func(w http.ResponseWriter, r *http.Request) {
					oathRequestCnt++

					expectedAuthHeader := alertingHttp.GetBasicAuthHeader("test-client-id", "test-client-secret")
					actualAuthHeader := r.Header.Get("Authorization")
					assert.Equalf(t, expectedAuthHeader, actualAuthHeader, "expected Authorization header to match, got: %s", actualAuthHeader)

					err := r.ParseForm()
					assert.NoError(t, err, "expected no error parsing form")

					expectedOAuth2RequestValues := url.Values{"grant_type": []string{"client_credentials"}}
					assert.Equalf(t, expectedOAuth2RequestValues, r.Form, "expected OAuth2 request values to match, got: %v", r.Form)

					res, _ := json.Marshal(expectedAuthResponse)
					w.Header().Add("Content-Type", "application/json")
					_, _ = w.Write(res)
				}

				oauth2Server := httptest.NewServer(http.HandlerFunc(oauthHandler))
				defer oauth2Server.Close()

				httpConfig, err := json.Marshal(map[string]any{
					"http_config": alertingHttp.HTTPClientConfig{
						OAuth2: &alertingHttp.OAuth2Config{
							ClientID:     "test-client-id",
							ClientSecret: "test-client-secret",
							TokenURL:     oauth2Server.URL + "/oauth2/token",
						},
					},
				})
				require.NoError(t, err)

				testRequestCnt := 0
				testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					testRequestCnt++
					assert.Equalf(t, expectedAuthResponse.TokenType+" "+expectedAuthResponse.AccessToken, r.Header.Get("Authorization"),
						"expected Authorization header from Access Token to match, got: %s", r.Header.Get("Authorization"))
					w.WriteHeader(http.StatusOK)
				}))
				defer testServer.Close()

				// Deal with notifiers that have hardcoded API URLs.
				origLine := line.APIURL
				origPushover := pushover.APIURL
				origTelegram := telegram.APIURL
				origThreema := threema.APIURL
				line.APIURL = testServer.URL
				pushover.APIURL = testServer.URL
				telegram.APIURL = testServer.URL + "/bot%s/%s"
				threema.APIURL = testServer.URL
				defer func() {
					line.APIURL = origLine
					pushover.APIURL = origPushover
					telegram.APIURL = origTelegram
					threema.APIURL = origThreema
				}()

				config := cfg.GetRawNotifierConfig(notifierType)

				// Override common url patterns:
				urlOverride, err := json.Marshal(map[string]any{
					"url":            testServer.URL,
					"api_url":        testServer.URL, // JIRA, SNS, Webex
					"kafkaRestProxy": testServer.URL, // Kafka REST Proxy
					"brokerUrl":      testServer.URL, // MQTT
					"apiUrl":         testServer.URL, // OpsGenie
					"client_url":     testServer.URL, // PagerDuty
					"endpointUrl":    testServer.URL, // Slack, Wecom
				})
				require.NoError(t, err)
				newSettings, err := MergeSettings([]byte(config.Settings), urlOverride)
				require.NoError(t, err)
				config.SecureSettings = nil // Remove SecureSettings to prevent interference with url overrrides.

				// Modify the config to include http_config
				newSettings, err = MergeSettings(newSettings, httpConfig)
				require.NoError(t, err)

				config.Settings = newSettings

				recCfg := &APIReceiver{
					ConfigReceiver: ConfigReceiver{Name: "test-receiver"},
					GrafanaIntegrations: GrafanaIntegrations{
						[]*GrafanaIntegrationConfig{
							config,
						},
					},
				}

				parsed, err := BuildReceiverConfiguration(context.Background(), recCfg, DecodeSecretsFromBase64, GetDecryptedValueFnForTesting)
				require.NoError(t, err)

				invalidAddress := fmt.Errorf("invalid address")
				allowedUrls := map[string]struct{}{
					oauth2Server.URL: {},
					testServer.URL:   {},
				}
				integrations, err := BuildGrafanaReceiverIntegrations(
					parsed,
					templates.ForTests(t),
					&images.URLProvider{},
					log.NewNopLogger(),
					receivers.MockNotificationService(),
					NoWrap,
					rand.Int63(),
					fmt.Sprintf("Grafana v%d", rand.Uint32()),
					nil,
					alertingHttp.WithDialer(net.Dialer{
						// Prevent all network calls not going to oauth2Server or testServer.
						// Since we're actually calling the real Notify method, this is to ensure that
						// we don't start calling real endpoints in the tests.
						// Additionally, it will help ensure the test is correctly validating the OAuth2 flow.
						Control: func(_, address string, _ syscall.RawConn) error {
							if _, ok := allowedUrls["http://"+address]; !ok {
								return fmt.Errorf("%w: %s", invalidAddress, address)
							}
							return nil
						},
					}),
				)
				require.NoError(t, err)

				require.Len(t, integrations, 1)

				integration := integrations[0]

				alert := newTestAlert(TestReceiversConfigBodyParams{}, time.Now(), time.Now())

				ctx := context.Background()
				ctx = notify.WithGroupKey(ctx, fmt.Sprintf("%s-%s-%d", integration.Name(), alert.Labels.Fingerprint(), time.Now().Unix()))
				ctx = notify.WithGroupLabels(ctx, alert.Labels)
				ctx = notify.WithReceiverName(ctx, integration.String())
				_, err = integration.Notify(ctx, &alert)
				if errors.Is(err, invalidAddress) {
					t.Errorf("notifier should not be sending to anything but oauth2Server or testServer, got: %v", err)
				}

				assert.Equal(t, 1, oathRequestCnt, "expected %d OAuth2 request to be sent, got: %d", 1, oathRequestCnt)
				assert.Equal(t, 1, testRequestCnt, "expected %d webhook request to be sent, got: %d", 1, testRequestCnt)
			})
		})
	}
}

func allReceivers(r *GrafanaReceiverConfig) ([]NotifierConfig[any], int) {
	var recvs []NotifierConfig[any]
	data, _ := json.Marshal(r)
	var asMap map[string][]NotifierConfig[any]
	_ = json.Unmarshal(data, &asMap)

	notifierConfigPrefix := reflect.TypeOf((*NotifierConfig[any])(nil)).Elem().Name()
	notifierConfigPrefix = notifierConfigPrefix[:strings.Index(notifierConfigPrefix, "[")+1]
	isNotifierConfigField := func(name string) bool {
		field, ok := reflect.TypeOf(GrafanaReceiverConfig{}).FieldByName(name)
		if !ok || field.Type.Kind() != reflect.Slice {
			return false
		}

		if !strings.HasPrefix(field.Type.Elem().Elem().Name(), notifierConfigPrefix) {
			return false
		}
		return true
	}

	missing := 0
	for k, configs := range asMap {
		if !isNotifierConfigField(k) {
			// Skip fields that are not of type []*NotifierConfig.
			continue
		}
		if len(configs) == 0 {
			missing++
			continue
		}
		recvs = append(recvs, configs...)
	}
	return recvs, missing
}
