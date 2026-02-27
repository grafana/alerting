package v0mimir1test

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
