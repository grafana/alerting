package schema

func V0ProxyConfigOptions() []Field {
	return []Field{
		{
			Label:        "Proxy URL",
			Description:  "HTTP proxy server to use to connect to the targets.",
			Element:      ElementTypeInput,
			InputType:    InputTypeText,
			PropertyName: "proxy_url",
		},
		{
			Label:        "No Proxy",
			Description:  "Comma-separated list of domains for which the proxy should not be used.",
			Element:      ElementTypeInput,
			InputType:    InputTypeText,
			PropertyName: "no_proxy",
		},
		{
			Label:        "Proxy From Environment",
			Description:  "Makes use of net/http ProxyFromEnvironment function to determine proxies.",
			Element:      ElementTypeCheckbox,
			PropertyName: "proxy_from_environment",
		},
		{
			Label:        "Proxy Header Environment",
			Description:  "Headers to send to proxies during CONNECT requests.",
			Element:      ElementTypeKeyValueMap,
			PropertyName: "proxy_connect_header",
		},
	}
}

func V0TLSConfigOption(propertyName string) Field {
	return Field{
		Label:        "TLS config",
		Description:  "Configures the TLS settings.",
		PropertyName: propertyName,
		Element:      ElementTypeSubform,
		SubformOptions: []Field{
			{
				Label:        "Server name",
				Description:  "ServerName extension to indicate the name of the server.",
				Element:      ElementTypeInput,
				InputType:    InputTypeText,
				PropertyName: "server_name",
			},
			{
				Label:        "Skip verify",
				Description:  "Disable validation of the server certificate.",
				Element:      ElementTypeCheckbox,
				PropertyName: "insecure_skip_verify",
			},
			{
				Label:        "Min TLS Version",
				Element:      ElementTypeInput,
				InputType:    InputTypeText,
				PropertyName: "min_version",
			},
			{
				Label:        "Max TLS Version",
				Element:      ElementTypeInput,
				InputType:    InputTypeText,
				PropertyName: "max_version",
			},
		},
	}
}

func V0HttpConfigOption() Field {
	oauth2ConfigOption := func() Field {
		return Field{
			Label:        "OAuth2",
			Description:  "Configures the OAuth2 settings.",
			PropertyName: "oauth2",
			Element:      ElementTypeSubform,
			SubformOptions: []Field{
				{
					Label:        "Client ID",
					Description:  "The OAuth2 client ID",
					Element:      ElementTypeInput,
					InputType:    InputTypeText,
					PropertyName: "client_id",
					Required:     true,
				},
				{
					Label:        "Client secret",
					Description:  "The OAuth2 client secret",
					Element:      ElementTypeInput,
					InputType:    InputTypePassword,
					PropertyName: "client_secret",
					Required:     true,
					Secure:       true,
				},
				{
					Label:        "Token URL",
					Description:  "The OAuth2 token exchange URL",
					Element:      ElementTypeInput,
					InputType:    InputTypeText,
					PropertyName: "token_url",
					Required:     true,
				},
				{
					Label:        "Scopes",
					Description:  "Comma-separated list of scopes",
					Element:      ElementStringArray,
					PropertyName: "scopes",
				},
				{
					Label:        "Additional parameters",
					Element:      ElementTypeKeyValueMap,
					PropertyName: "endpoint_params",
				},
				V0TLSConfigOption("TLSConfig"),
			},
		}
	}
	return Field{
		Label:        "HTTP Config",
		Description:  "Note that `basic_auth` and `bearer_token` options are mutually exclusive.",
		PropertyName: "http_config",
		Element:      ElementTypeSubform,
		SubformOptions: append([]Field{
			{
				Label:        "Basic auth",
				Description:  "Sets the `Authorization` header with the configured username and password.",
				PropertyName: "basic_auth",
				Element:      ElementTypeSubform,
				SubformOptions: []Field{
					{
						Label:        "Username",
						Element:      ElementTypeInput,
						InputType:    InputTypeText,
						PropertyName: "username",
					},
					{
						Label:        "Password",
						Element:      ElementTypeInput,
						InputType:    InputTypePassword,
						PropertyName: "password",
						Secure:       true,
					},
				},
			},
			{
				Label:        "Authorization",
				Description:  "The HTTP authorization credentials for the targets.",
				Element:      ElementTypeSubform,
				PropertyName: "authorization",
				SubformOptions: []Field{
					{
						Label:        "Type",
						Element:      ElementTypeInput,
						InputType:    InputTypeText,
						PropertyName: "type",
					},
					{
						Label:        "Credentials",
						Element:      ElementTypeInput,
						InputType:    InputTypePassword,
						PropertyName: "credentials",
						Secure:       true,
					},
				},
			},
			{
				Label:        "Follow redirects",
				Description:  "Whether the client should follow HTTP 3xx redirects.",
				Element:      ElementTypeCheckbox,
				PropertyName: "follow_redirects",
			},
			{
				Label:        "Enable HTTP2",
				Description:  "Whether the client should configure HTTP2.",
				Element:      ElementTypeCheckbox,
				PropertyName: "enable_http2",
			},
			{
				Label:        "HTTP Headers",
				Description:  "Headers to inject in the requests.",
				Element:      ElementTypeKeyValueMap,
				PropertyName: "http_headers",
			},
		}, append(
			V0ProxyConfigOptions(),
			V0TLSConfigOption("tls_config"),
			oauth2ConfigOption())...,
		),
	}
}
