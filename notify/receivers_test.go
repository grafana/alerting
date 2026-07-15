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
	"syscall"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yamlv2 "gopkg.in/yaml.v2"
	yamlv3 "gopkg.in/yaml.v3"

	"github.com/prometheus/alertmanager/notify"

	alertingHttp "github.com/grafana/alerting/http"
	"github.com/grafana/alerting/images"
	"github.com/grafana/alerting/models"
	"github.com/grafana/alerting/notify/notifytest"
	"github.com/grafana/alerting/receivers"
	line "github.com/grafana/alerting/receivers/line/v1"
	pushover "github.com/grafana/alerting/receivers/pushover/v1"
	"github.com/grafana/alerting/receivers/schema"
	telegram "github.com/grafana/alerting/receivers/telegram/v1"
	receiversTesting "github.com/grafana/alerting/receivers/testing"
	threema "github.com/grafana/alerting/receivers/threema/v1"
	"github.com/grafana/alerting/templates"
)

func TestReceiverTimeoutError_Error(t *testing.T) {
	e := IntegrationTimeoutError{
		Integration: &models.IntegrationConfig{
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
		r := &models.IntegrationConfig{
			Name: "test",
			UID:  "uid",
		}
		require.Equal(t, IntegrationTimeoutError{
			Integration: r,
			Err:         context.DeadlineExceeded,
		}, ProcessIntegrationError(r, context.DeadlineExceeded))
	})

	t.Run("assert IntegrationTimeoutError is returned for *url.Error timeout", func(t *testing.T) {
		r := &models.IntegrationConfig{
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
		r := &models.IntegrationConfig{
			Name: "test",
			UID:  "uid",
		}
		err := errors.New("this is an error")
		require.Equal(t, err, ProcessIntegrationError(r, err))
	})
}

func TestHTTPConfig(t *testing.T) {
	for notifierType, cfg := range notifytest.AllKnownV1ConfigsForTesting {
		t.Run(string(notifierType), func(t *testing.T) {
			if notifierType == schema.EmailType {
				t.Skip("does not support http_config")
			}

			if notifierType == schema.SlackType ||
				notifierType == schema.SNSType ||
				notifierType == schema.MQTTType ||
				notifierType == schema.AlertManagerType {
				t.Skip("does not yet support http client")
			}

			t.Run("should support building with http_config", func(t *testing.T) {
				config := cfg.GetRawNotifierConfig("")

				// Config should include http_config if the notifier supports it, but let's sanity check.
				require.Containsf(t, string(config.Settings), "http_config", "notifier %s does not contain http_config", notifierType)

				// buildDecrypt mirrors how the manifest factory resolves secure settings
				// (decode base64, then look up via the decrypt function) so parseHTTPConfig
				// receives the same input it does in production.
				buildDecrypt := func(secure map[string]string) receivers.DecryptFunc {
					secureSettings, err := DecodeSecretsFromBase64(secure)
					require.NoError(t, err)
					return func(key string, fallback string) (string, bool) {
						if _, ok := secureSettings[key]; !ok {
							return fallback, false
						}
						return GetDecryptedValueFnForTesting(context.Background(), secureSettings, key, fallback), true
					}
				}

				t.Run("with secureSettings", func(t *testing.T) {
					for key, value := range receiversTesting.ReadSecretsJSONForTesting(notifytest.FullValidHTTPConfigSecretsForTesting) {
						config.SecureSettings[key] = base64.StdEncoding.EncodeToString(value)
					}

					httpClientConfig, err := parseHTTPConfig(config, buildDecrypt(config.SecureSettings))
					require.NoError(t, err)

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

					require.Equal(t, expectedHTTPConfig, httpClientConfig)
				})

				t.Run("without secureSettings", func(t *testing.T) {
					config.SecureSettings = nil
					httpClientConfig, err := parseHTTPConfig(config, buildDecrypt(config.SecureSettings))
					require.NoError(t, err)

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

					require.Equal(t, expectedHTTPConfig, httpClientConfig)
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

				config := cfg.GetRawNotifierConfig("")

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
				newSettings, err := MergeSettings(config.Settings, urlOverride)
				require.NoError(t, err)
				config.SecureSettings = nil // Remove SecureSettings to prevent interference with url overrrides.

				// Modify the config to include http_config
				newSettings, err = MergeSettings(newSettings, httpConfig)
				require.NoError(t, err)

				config.Settings = newSettings

				recCfg := models.ReceiverConfig{
					Name: "test-receiver",
					Integrations: []*models.IntegrationConfig{
						config,
					},
				}

				invalidAddress := fmt.Errorf("invalid address")
				allowedUrls := map[string]struct{}{
					oauth2Server.URL: {},
					testServer.URL:   {},
				}

				tmplCfg, err := templates.NewConfig("grafana", "http://localhost", "", templates.DefaultLimits)
				require.NoError(t, err)
				tmpl, err := templates.NewFactory(nil, tmplCfg, log.NewNopLogger())
				require.NoError(t, err)

				integrations, err := BuildReceiverIntegrationsWithManifests(
					rand.Int63(),
					recCfg,
					tmpl,
					&images.URLProvider{},
					GetDecryptedValueFnForTesting,
					DecodeSecretsFromBase64,
					receivers.MockNotificationService(),
					[]alertingHttp.ClientOption{
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
					},
					NoWrap,
					fmt.Sprintf("Grafana v%d", rand.Uint32()),
					log.NewNopLogger(),
					nil,
				)
				require.NoError(t, err)

				require.Len(t, integrations, 1)

				integration := integrations[0]

				alert := newTestAlert(nil, time.Now(), time.Now())

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

// TestReceiver_JSONRoundTrip verifies JSON marshal/unmarshal roundtrip for a
// receiver with all integration types, using MarshalJSONWithSecrets to
// preserve secret values in plain text.
func TestReceiver_JSONRoundTrip(t *testing.T) {
	original := notifytest.FullValidMimirReceiver()

	integrations, err := ConfigReceiverToMimirIntegrations(original)
	require.NoError(t, err)
	for _, integration := range integrations {
		t.Run(fmt.Sprintf("%s %s", integration.Schema.Type(), integration.Schema.Version), func(t *testing.T) {
			data, err := integration.ConfigJSON()
			require.NoError(t, err)

			t.Run("JSON unmarshaled by JSON", func(t *testing.T) {
				got := reflect.New(reflect.TypeOf(integration.Config)).Interface()
				require.NoError(t, json.Unmarshal(data, got))

				require.Equal(t, integration.Config, reflect.ValueOf(got).Elem().Interface())
			})

			t.Run("JSON unmarshaled by YAML v2", func(t *testing.T) {
				got := reflect.New(reflect.TypeOf(integration.Config)).Interface()
				require.NoError(t, yamlv2.Unmarshal(data, got))

				require.Equal(t, integration.Config, reflect.ValueOf(got).Elem().Interface())
			})

			t.Run("JSON unmarshaled by YAML v3", func(t *testing.T) {
				got := reflect.New(reflect.TypeOf(integration.Config)).Interface()
				require.NoError(t, yamlv3.Unmarshal(data, got))

				require.Equal(t, integration.Config, reflect.ValueOf(got).Elem().Interface())
			})
		})
	}
}

func TestValidateAPIReceiver(t *testing.T) {
	ctx := context.Background()
	decrypt := GetDecryptedValueFnForTesting

	t.Run("accepts valid configs for all known integrations", func(t *testing.T) {
		for key, cfg := range notifytest.AllKnownConfigsForTesting {
			t.Run(fmt.Sprintf("%s %s", key.Type, key.Version), func(t *testing.T) {
				raw := cfg.GetRawNotifierConfig("")
				api := models.ReceiverConfig{
					Name:         "test-receiver",
					Integrations: []*models.IntegrationConfig{raw},
				}
				require.NoError(t, ValidateAPIReceiver(ctx, api, DecodeSecretsFromBase64, decrypt))
			})
		}
	})

	t.Run("returns error when receiver name is empty", func(t *testing.T) {
		api := models.ReceiverConfig{}
		err := ValidateAPIReceiver(ctx, api, DecodeSecretsFromBase64, decrypt)
		require.Error(t, err)
		require.ErrorContains(t, err, "receiver name is required")
	})

	t.Run("returns error for unknown integration type", func(t *testing.T) {
		raw := &models.IntegrationConfig{
			Name:     "test",
			Type:     "unknown-type",
			Version:  schema.V1,
			Settings: json.RawMessage(`{}`),
		}
		api := models.ReceiverConfig{
			Name:         "test-receiver",
			Integrations: []*models.IntegrationConfig{raw},
		}
		err := ValidateAPIReceiver(ctx, api, DecodeSecretsFromBase64, decrypt)
		require.ErrorContains(t, err, "invalid integration config at index 0")
		require.ErrorContains(t, err, "invalid integration type or version")
	})

	t.Run("returns error for unknown integration version", func(t *testing.T) {
		raw := &models.IntegrationConfig{
			Name:     "test",
			Type:     schema.WebhookType,
			Version:  "v99",
			Settings: json.RawMessage(`{}`),
		}
		api := models.ReceiverConfig{
			Name:         "test-receiver",
			Integrations: []*models.IntegrationConfig{raw},
		}
		err := ValidateAPIReceiver(ctx, api, DecodeSecretsFromBase64, decrypt)
		require.ErrorContains(t, err, "invalid integration config at index 0")
		require.ErrorContains(t, err, "invalid integration type or version")
	})

	t.Run("returns error when integration config is invalid", func(t *testing.T) {
		// webhook v1 requires a non-empty URL
		raw := &models.IntegrationConfig{
			Name:     "test",
			Type:     schema.WebhookType,
			Version:  schema.V1,
			Settings: json.RawMessage(`{"url": ""}`),
		}
		api := models.ReceiverConfig{
			Name:         "test-receiver",
			Integrations: []*models.IntegrationConfig{raw},
		}
		err := ValidateAPIReceiver(ctx, api, DecodeSecretsFromBase64, decrypt)
		require.ErrorContains(t, err, "invalid integration config at index 0")
	})

	t.Run("returns error when secure settings cannot be decoded", func(t *testing.T) {
		raw := &models.IntegrationConfig{
			Name:           "test",
			Type:           schema.WebhookType,
			Version:        schema.V1,
			Settings:       json.RawMessage(`{"url": "http://localhost"}`),
			SecureSettings: map[string]string{"key": "not-valid-base64!!!"},
		}
		api := models.ReceiverConfig{
			Name:         "test-receiver",
			Integrations: []*models.IntegrationConfig{raw},
		}
		err := ValidateAPIReceiver(ctx, api, DecodeSecretsFromBase64, decrypt)
		require.ErrorContains(t, err, "invalid integration config at index 0")
	})

	t.Run("collects errors from multiple invalid integrations", func(t *testing.T) {
		makeInvalid := func(name string) *models.IntegrationConfig {
			return &models.IntegrationConfig{
				Name:     name,
				Type:     "unknown-type",
				Version:  schema.V1,
				Settings: json.RawMessage(`{}`),
			}
		}
		api := models.ReceiverConfig{
			Name:         "test-receiver",
			Integrations: []*models.IntegrationConfig{makeInvalid("a"), makeInvalid("b")},
		}
		err := ValidateAPIReceiver(ctx, api, DecodeSecretsFromBase64, decrypt)
		require.ErrorContains(t, err, "invalid integration config at index 0")
		require.ErrorContains(t, err, "invalid integration config at index 1")
	})
}
