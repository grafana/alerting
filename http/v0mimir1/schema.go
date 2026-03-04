package v0mimir1

import "github.com/grafana/alerting/receivers/schema"

func V0ProxyConfigOptions() []schema.Field {
	return []schema.Field{
		{
			Label:        "Proxy URL",
			Description:  "HTTP proxy server to use to connect to the targets.",
			Element:      schema.ElementTypeInput,
			InputType:    schema.InputTypeText,
			PropertyName: "proxy_url",
		},
		{
			Label:        "No Proxy",
			Description:  "Comma-separated list of domains for which the proxy should not be used.",
			Element:      schema.ElementTypeInput,
			InputType:    schema.InputTypeText,
			PropertyName: "no_proxy",
		},
		{
			Label:        "Proxy From Environment",
			Description:  "Makes use of net/http ProxyFromEnvironment function to determine proxies.",
			Element:      schema.ElementTypeCheckbox,
			PropertyName: "proxy_from_environment",
		},
		{
			Label:        "Proxy Header Environment",
			Description:  "Headers to send to proxies during CONNECT requests.",
			Element:      schema.ElementTypeKeyValueMap,
			PropertyName: "proxy_connect_header",
		},
	}
}

func V0TLSConfigOption(propertyName string) schema.Field {
	return schema.Field{
		Label:        "TLS config",
		Description:  "Configures the TLS settings.",
		PropertyName: propertyName,
		Element:      schema.ElementTypeSubform,
		SubformOptions: []schema.Field{
			{
				Label:        "Server name",
				Description:  "ServerName extension to indicate the name of the server.",
				Element:      schema.ElementTypeInput,
				InputType:    schema.InputTypeText,
				PropertyName: "server_name",
			},
			{
				Label:        "Skip verify",
				Description:  "Disable validation of the server certificate.",
				Element:      schema.ElementTypeCheckbox,
				PropertyName: "insecure_skip_verify",
			},
			{
				Label:        "Min TLS Version",
				Element:      schema.ElementTypeInput,
				InputType:    schema.InputTypeText,
				PropertyName: "min_version",
			},
			{
				Label:        "Max TLS Version",
				Element:      schema.ElementTypeInput,
				InputType:    schema.InputTypeText,
				PropertyName: "max_version",
			},
		},
	}
}

func V0HttpConfigOption() schema.Field {
	oauth2ConfigOption := func() schema.Field {
		return schema.Field{
			Label:        "OAuth2",
			Description:  "Configures the OAuth2 settings.",
			PropertyName: "oauth2",
			Element:      schema.ElementTypeSubform,
			SubformOptions: []schema.Field{
				{
					Label:        "Client ID",
					Description:  "The OAuth2 client ID",
					Element:      schema.ElementTypeInput,
					InputType:    schema.InputTypeText,
					PropertyName: "client_id",
					Required:     true,
				},
				{
					Label:        "Client secret",
					Description:  "The OAuth2 client secret",
					Element:      schema.ElementTypeInput,
					InputType:    schema.InputTypePassword,
					PropertyName: "client_secret",
					Required:     true,
					Secure:       true,
				},
				{
					Label:        "Token URL",
					Description:  "The OAuth2 token exchange URL",
					Element:      schema.ElementTypeInput,
					InputType:    schema.InputTypeText,
					PropertyName: "token_url",
					Required:     true,
				},
				{
					Label:        "Scopes",
					Description:  "Comma-separated list of scopes",
					Element:      schema.ElementStringArray,
					PropertyName: "scopes",
				},
				{
					Label:        "Additional parameters",
					Element:      schema.ElementTypeKeyValueMap,
					PropertyName: "endpoint_params",
				},
				V0TLSConfigOption("TLSConfig"),
			},
		}
	}
	return schema.Field{
		Label:        "HTTP Config",
		Description:  "Note that `basic_auth` and `bearer_token` options are mutually exclusive.",
		PropertyName: "http_config",
		Element:      schema.ElementTypeSubform,
		SubformOptions: append([]schema.Field{
			{
				Label:        "Basic auth",
				Description:  "Sets the `Authorization` header with the configured username and password.",
				PropertyName: "basic_auth",
				Element:      schema.ElementTypeSubform,
				SubformOptions: []schema.Field{
					{
						Label:        "Username",
						Element:      schema.ElementTypeInput,
						InputType:    schema.InputTypeText,
						PropertyName: "username",
					},
					{
						Label:        "Password",
						Element:      schema.ElementTypeInput,
						InputType:    schema.InputTypePassword,
						PropertyName: "password",
						Secure:       true,
					},
				},
			},
			{
				Label:        "Authorization",
				Description:  "The HTTP authorization credentials for the targets.",
				Element:      schema.ElementTypeSubform,
				PropertyName: "authorization",
				SubformOptions: []schema.Field{
					{
						Label:        "Type",
						Element:      schema.ElementTypeInput,
						InputType:    schema.InputTypeText,
						PropertyName: "type",
					},
					{
						Label:        "Credentials",
						Element:      schema.ElementTypeInput,
						InputType:    schema.InputTypePassword,
						PropertyName: "credentials",
						Secure:       true,
					},
				},
			},
			{
				Label:        "Follow redirects",
				Description:  "Whether the client should follow HTTP 3xx redirects.",
				Element:      schema.ElementTypeCheckbox,
				PropertyName: "follow_redirects",
			},
			{
				Label:        "Enable HTTP2",
				Description:  "Whether the client should configure HTTP2.",
				Element:      schema.ElementTypeCheckbox,
				PropertyName: "enable_http2",
			},
			{
				Label:        "HTTP Headers",
				Description:  "Headers to inject in the requests.",
				Element:      schema.ElementTypeKeyValueMap,
				PropertyName: "http_headers",
			},
		}, append(
			V0ProxyConfigOptions(),
			V0TLSConfigOption("tls_config"),
			oauth2ConfigOption())...,
		),
	}
}
