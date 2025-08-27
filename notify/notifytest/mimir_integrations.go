package notifytest

import (
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"slices"

	promCfg "github.com/prometheus/alertmanager/config"
)

type MimirIntegrationHTTPConfigOption string

const (
	WithBasicAuth             = MimirIntegrationHTTPConfigOption("basic_auth")
	WithLegacyBearerTokenAuth = MimirIntegrationHTTPConfigOption("bearer_token")
	WithAuthorization         = MimirIntegrationHTTPConfigOption("authorization")
	WithOAuth2                = MimirIntegrationHTTPConfigOption("oauth2")
	WithTLS                   = MimirIntegrationHTTPConfigOption("tls_config")
	WithHeaders               = MimirIntegrationHTTPConfigOption("headers")
	WithProxy                 = MimirIntegrationHTTPConfigOption("proxy_config")
	WithDefault               = MimirIntegrationHTTPConfigOption("default")
)

// GetMimirIntegration creates a new instance of the given integration type with selected http config options.
// It panics if the configuration process encounters an issue.
func GetMimirIntegration[T any](opts ...MimirIntegrationHTTPConfigOption) (T, error) {
	var config T
	cfg, err := GetRawConfigForMimirIntegration(reflect.TypeOf(config), opts...)
	if err != nil {
		return config, err
	}
	err = json.Unmarshal([]byte(cfg), &config)
	if err != nil {
		return config, fmt.Errorf("failed to unmarshal config %T: %v", config, err)
	}
	return config, nil
}

// GetMimirIntegrationForType creates a new instance of the given integration type with selected http config options.
// It panics if the configuration process encounters an issue.
func GetMimirIntegrationForType(iType reflect.Type, opts ...MimirIntegrationHTTPConfigOption) (any, error) {
	cfg, err := GetRawConfigForMimirIntegration(iType, opts...)
	if err != nil {
		return nil, err
	}
	elemPtr := reflect.New(iType).Interface()
	err = json.Unmarshal([]byte(cfg), elemPtr)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config %T: %v", iType, err)
	}
	return elemPtr, nil
}

// GetMimirReceiverWithIntegrations creates a Receiver with selected integrations configured from given types and options.
// It returns a Receiver for testing purposes or an error if the configuration process encounters an issue.
func GetMimirReceiverWithIntegrations(iTypes []reflect.Type, opts ...MimirIntegrationHTTPConfigOption) (promCfg.Receiver, error) {
	receiver := promCfg.Receiver{Name: "receiver"}
	receiverVal := reflect.ValueOf(&receiver).Elem()
	receiverType := receiverVal.Type()
	for i := 0; i < receiverType.NumField(); i++ {
		integrationField := receiverType.Field(i)
		if integrationField.Type.Kind() != reflect.Slice {
			continue
		}
		sliceType := integrationField.Type
		elemType := sliceType.Elem()

		sliceVal := reflect.MakeSlice(sliceType, 0, 1)

		// Create a new instance of the element type
		elemPtr := reflect.New(elemType).Interface()
		underlyingType := elemType
		if underlyingType.Kind() == reflect.Ptr {
			underlyingType = underlyingType.Elem()
		}
		if !slices.Contains(iTypes, underlyingType) {
			continue
		}
		rawConfig, err := GetRawConfigForMimirIntegration(underlyingType, opts...)
		if err != nil {
			return promCfg.Receiver{}, fmt.Errorf("failed to get config for type [%s]: %v", underlyingType.String(), err)
		}
		if err := json.Unmarshal([]byte(rawConfig), elemPtr); err != nil {
			return promCfg.Receiver{}, fmt.Errorf("failed to parse config for type %s: %v", elemType.String(), err)
		}
		sliceVal = reflect.Append(sliceVal, reflect.ValueOf(elemPtr).Elem())
		receiverVal.FieldByName(integrationField.Name).Set(sliceVal)
	}
	return receiver, nil
}

// GetMimirReceiverWithAllIntegrations creates a Receiver with all integrations configured from given types and options.
// It returns a Receiver for testing purposes or an error if the configuration process encounters an issue.
func GetMimirReceiverWithAllIntegrations(opts ...MimirIntegrationHTTPConfigOption) (promCfg.Receiver, error) {
	return GetMimirReceiverWithIntegrations(slices.Collect(maps.Keys(ValidMimirConfigs)), opts...)
}

func GetRawConfigForMimirIntegration(iType reflect.Type, opts ...MimirIntegrationHTTPConfigOption) (string, error) {
	cfg, ok := ValidMimirConfigs[iType]
	if !ok {
		return "", fmt.Errorf("invalid config type [%s", iType.String())
	}
	if _, ok := iType.FieldByName("HTTPConfig"); !ok { // ignore integrations without HTTPConfig
		return cfg, nil
	}
	if len(opts) == 0 {
		opts = []MimirIntegrationHTTPConfigOption{WithDefault}
	}
	for _, opt := range opts {
		c, ok := ValidMimirHTTPConfigs[opt]
		if !ok {
			return "", fmt.Errorf("invalid option [%s]", opt)
		}
		bytes, err := mergeSettings([]byte(cfg), []byte(c))
		if err != nil {
			return "", fmt.Errorf("failed to merge config for type [%s] with options [%s]: %v", iType.String(), opt, err)
		}
		cfg = string(bytes)
	}
	return cfg, nil
}

var ValidMimirHTTPConfigs = map[MimirIntegrationHTTPConfigOption]string{
	WithBasicAuth: `{
		"http_config": {
			"tls_config": {
				"insecure_skip_verify": false
			},
			"follow_redirects": true,
			"enable_http2": true,
			"proxy_url": "",
			"basic_auth": {
				"username": "test-username",
				"password": "test-password"
			}
		}
	}`,
	WithLegacyBearerTokenAuth: `{
		"http_config": {
			"tls_config": {
				"insecure_skip_verify": false
			},
			"follow_redirects": true,
			"enable_http2": true,
			"proxy_url": "",
			"bearer_token": "test-token"
		}
	}`,
	WithAuthorization: `{
		"http_config": {
			"tls_config": {
				"insecure_skip_verify": false
			},
			"follow_redirects": true,
			"enable_http2": true,
			"proxy_url": "",
			"authorization": {
				"type": "bearer",
				"credentials": "test-credentials"
			}
		}
	}`,
	WithOAuth2: `{
		"http_config": {
			"tls_config": {
				"insecure_skip_verify": false
			},
			"follow_redirects": true,
			"enable_http2": true,
			"proxy_url": "",
			"oauth2": {
				"client_id": "test-client-id",
				"client_secret": "test-client-secret",
				"client_secret_file": "",
				"client_secret_ref": "",
				"token_url": "https://localhost/auth/token",
				"scopes": ["scope1", "scope2"],
				"endpoint_params": {
					"param1": "value1",
					"param2": "value2"
				},
				"TLSConfig": {
                    "insecure_skip_verify": false
				},
				"proxy_url": ""
			}
	    }
	}`,
	WithTLS: `{
		"http_config": {
			"follow_redirects": true,
			"enable_http2": true,
			"proxy_url": "",
			"tls_config": {
				"insecure_skip_verify": false,
				"server_name": "test-server-name"
			}
	    }
	}`,
	WithHeaders: `{
		"http_config": {
			"tls_config": {
				"insecure_skip_verify": false
			},
			"follow_redirects": true,
			"enable_http2": true,
			"http_headers": {
				"headers": {
					"X-Header-1": {
						"secrets": ["value1"]
					},
					"X-Header-2": {
						"values": ["value2"]
					}
				}
			}
		}
	}`,
	WithProxy: `{
		"http_config": {
			"tls_config": {
				"insecure_skip_verify": false
			},
			"follow_redirects": true,
			"enable_http2": true,
			"proxy_url": "http://localproxy:8080",
			"no_proxy": "localhost",
			"proxy_connect_header": {
				"X-Proxy-Header": ["proxy-value"]
			}
		}
	}`,
	// This reflects the default
	WithDefault: `{
		"http_config": {
			"tls_config": {
				"insecure_skip_verify": false
			},
			"follow_redirects": true,
			"enable_http2": true,
			"proxy_url": ""
		}
	}`,
}

var ValidMimirConfigs = map[reflect.Type]string{
	reflect.TypeOf(promCfg.DiscordConfig{}): `{
			"send_resolved": true,
			"webhook_url": "http://localhost",
			"http_config": {},
			"title": "test title",
			"message": "test message"
		}`,
	reflect.TypeOf(promCfg.EmailConfig{}): `{
			"to": "team@example.com",
			"from": "alertmanager@example.com",
			"smarthost": "smtp.example.com:587",
			"auth_username": "alertmanager",
			"auth_password": "password123",
			"auth_secret": "secret-auth",
			"auth_identity": "alertmanager",
			"require_tls": true,
			"text": "test email",
			"headers": {
				"Subject": "test subject"
			},
			"tls_config": {
				"insecure_skip_verify": false,     
				"server_name": "test-server-name"
			},
			"send_resolved": true
        }`,
	reflect.TypeOf(promCfg.PagerdutyConfig{}): ` {
			"url": "http://localhost/",
			"http_config": {},
			"routing_key": "test-routing-secret-key",
			"service_key": "test-service-secret-key",
			"client": "Alertmanager",
			"client_url": "https://monitoring.example.com",
			"description": "test description",
			"severity": "test severity",
			"details": {
			  "firing": "test firing"
			},
			"images": [
				{ 
					"alt": "test alt",
					"src": "test src",
					"href": "http://localhost"
				}
			],
			"links": [
				{
					"href": "http://localhost",
					"text": "test text"     
				}
			],
			"source": "test source",
			"class": "test class",
			"component": "test component",
			"group": "test group",
			"send_resolved": true
        }`,
	reflect.TypeOf(promCfg.SlackConfig{}): `{
			"api_url": "http://localhost",
			"http_config": {},
			"channel": "#alerts",
			"username": "Alerting Team",
			"color": "danger",
			"title": "test title",
			"title_link": "http://localhost",
			"pretext": "test pretext",
			"text": "test text",
			"fields": [ 
				{
					"title": "test title",
					"value": "test value",
					"short": true   
				}
			],
			"short_fields": true,
			"footer": "test footer",
			"fallback": "test fallback",
			"callback_id": "test callback id",
			"icon_emoji": ":warning:",
			"icon_url": "https://example.com/icon.png",
			"image_url": "https://example.com/image.png",
			"thumb_url": "https://example.com/thumb.png",
			"link_names": true,
			"mrkdwn_in": ["fallback", "pretext", "text"],
			"actions": [
				{
					"type": "test-type",
					"text": "test-text",
					"url": "http://localhost",
					"style": "test-style",
					"name": "test-name",
					"value": "test-value",
					"confirm": {
						"title": "test-title",
						"text": "test-text",
						"ok_text": "test-ok-text",
						"dismiss_text": "test-dismiss-text"     
					}
				}
			],
			"send_resolved": true
		}`,
	reflect.TypeOf(promCfg.WebhookConfig{}): `{
			"send_resolved": true,
			"url": "http://localhost",
			"url_file": "",
			"http_config": {},
			"max_alerts": 10,
			"timeout": "30s"
		}`,
	reflect.TypeOf(promCfg.OpsGenieConfig{}): `{
			"api_key": "api-secret-key",
			"api_url": "http://localhost",
			"http_config": {},
			"message": "test message",
			"description": "test description",
			"source": "Alertmanager",
			"details": {
				"firing": "test firing"
			},
			"entity": "test entity",
			"responders": [{ "type": "team", "name": "ops-team" }],
			"actions": "test actions",
			"tags": "test-tags",   
			"note": "Triggered by Alertmanager",
			"priority": "P3",
			"update_alerts": true,
			"send_resolved": true
		}`,
	reflect.TypeOf(promCfg.WechatConfig{}): `{
			"send_resolved": true,
			"api_url": "http://localhost",
			"http_config": {},
			"api_secret": "12345-secret",
			"corp_id": "12345",
			"to_user": "user1",
			"to_party": "party1",
			"to_tag": "tag1",
			"agent_id": "1000002",
			"message": "test message",
			"message_type": "text"
        }`,
	reflect.TypeOf(promCfg.PushoverConfig{}): `{
			"user_key": "secret-user-key",
			"token": "secret-token",
			"title": "test title",
			"message": "test message",
			"url": "https://monitoring.example.com",
			"http_config": {},
			"url_title": "test url title",
			"device": "test device",
			"sound": "bike",
			"priority": "urgent",
			"retry": "30s",
			"expire": "1h0m0s",
			"ttl": "1h0m0s",
			"html": true,
			"send_resolved": true
		}`,
	reflect.TypeOf(promCfg.VictorOpsConfig{}): ` {
			"api_url": "http://localhost",
			"api_key": "secret-api-key",
			"http_config": {},
			"routing_key": "team1",
			"message_type": "CRITICAL",
			"entity_display_name": "test entity",
			"state_message": "test state message",
			"monitoring_tool": "Grafana",
			"custom_fields": {
				"test": "test"
			},
			"send_resolved": true
		}`,
	// all sigv4 fields of SNSConfig are different in yaml
	reflect.TypeOf(promCfg.SNSConfig{}): ` {
			"http_config": {},
			"topic_arn": "arn:aws:sns:us-east-1:123456789012:alerts",
			"sigv4": {
				"Region": "us-east-1",
				"AccessKey": "secret-access-key",
				"SecretKey": "secret-secret-key",
				"Profile": "default",
				"RoleARN": "arn:aws:iam::123456789012:role/role-name"
			},
			"subject": "test subject",
			"message": "test message",
			"attributes": { "key1": "value1" },
			"send_resolved": true
		}`,
	// token and chat fields of TelegramConfig are different in yaml
	reflect.TypeOf(promCfg.TelegramConfig{}): `{
			"api_url": "https://localhost",
			"http_config": {},
			"token": "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
			"chat": -1001234567890,
			"message": "TestMessage",
			"parse_mode": "MarkdownV2",
			"send_resolved": true
		}`,
	reflect.TypeOf(promCfg.WebexConfig{}): `{
			"api_url": "https://localhost",
			"http_config": {
			  "authorization": { "type": "Bearer", "credentials": "bot_token" }
			},
			"room_id": "12345",
			"message": "test templated message",
			"send_resolved": true
        }`,
	reflect.TypeOf(promCfg.MSTeamsConfig{}): `{
			"send_resolved": true,
			"webhook_url": "http://localhost",
			"http_config": {},
			"title": "test title",
			"summary": "test summary",
			"text": "test text"
        }`,
	reflect.TypeOf(promCfg.JiraConfig{}): `{
			"api_url": "http://localhost",
			"project": "PROJ",
			"issue_type": "Bug",
			"summary": "test summary",
			"description": "test description",
			"priority": "High",
			"labels": ["alertmanager"],
			"custom_fields": {
				"customfield_10000": "test customfield_10000"
			},
			"send_resolved": true
		}`,
}
