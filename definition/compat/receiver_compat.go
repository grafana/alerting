package compat

import (
	"reflect"

	"github.com/prometheus/alertmanager/config"
	"github.com/prometheus/common/model"

	"github.com/grafana/alerting/definition"
	httpcfg "github.com/grafana/alerting/http/v0mimir1"
	discord_v0mimir1 "github.com/grafana/alerting/receivers/discord/v0mimir1"
	email_v0mimir1 "github.com/grafana/alerting/receivers/email/v0mimir1"
	jira_v0mimir1 "github.com/grafana/alerting/receivers/jira/v0mimir1"
	opsgenie_v0mimir1 "github.com/grafana/alerting/receivers/opsgenie/v0mimir1"
	pagerduty_v0mimir1 "github.com/grafana/alerting/receivers/pagerduty/v0mimir1"
	pushover_v0mimir1 "github.com/grafana/alerting/receivers/pushover/v0mimir1"
	slack_v0mimir1 "github.com/grafana/alerting/receivers/slack/v0mimir1"
	sns_v0mimir1 "github.com/grafana/alerting/receivers/sns/v0mimir1"
	teams_v0mimir1 "github.com/grafana/alerting/receivers/teams/v0mimir1"
	teams_v0mimir2 "github.com/grafana/alerting/receivers/teams/v0mimir2"
	telegram_v0mimir1 "github.com/grafana/alerting/receivers/telegram/v0mimir1"
	victorops_v0mimir1 "github.com/grafana/alerting/receivers/victorops/v0mimir1"
	webex_v0mimir1 "github.com/grafana/alerting/receivers/webex/v0mimir1"
	webhook_v0mimir1 "github.com/grafana/alerting/receivers/webhook/v0mimir1"
	wechat_v0mimir1 "github.com/grafana/alerting/receivers/wechat/v0mimir1"
)

// UpstreamReceiverToDefinitionReceiver converts an upstream alertmanager config.Receiver to a definition.Receiver.
func UpstreamReceiverToDefinitionReceiver(r config.Receiver) definition.Receiver {
	def := definition.Receiver{Name: r.Name}

	for _, c := range r.DiscordConfigs {
		def.DiscordConfigs = append(def.DiscordConfigs, &discord_v0mimir1.Config{
			NotifierConfig: c.NotifierConfig,
			HTTPConfig:     httpcfg.FromCommonHTTPClientConfig(c.HTTPConfig),
			WebhookURL:     c.WebhookURL,
			WebhookURLFile: c.WebhookURLFile,
			Title:          c.Title,
			Message:        c.Message,
		})
	}

	for _, c := range r.EmailConfigs {
		def.EmailConfigs = append(def.EmailConfigs, &email_v0mimir1.Config{
			NotifierConfig:   c.NotifierConfig,
			To:               c.To,
			From:             c.From,
			Hello:            c.Hello,
			Smarthost:        c.Smarthost,
			AuthUsername:     c.AuthUsername,
			AuthPassword:     c.AuthPassword,
			AuthPasswordFile: c.AuthPasswordFile,
			AuthSecret:       c.AuthSecret,
			AuthIdentity:     c.AuthIdentity,
			Headers:          c.Headers,
			HTML:             c.HTML,
			Text:             c.Text,
			RequireTLS:       c.RequireTLS,
			TLSConfig:        httpcfg.FromCommonTLSConfig(c.TLSConfig),
		})
	}

	for _, c := range r.PagerdutyConfigs {
		def.PagerdutyConfigs = append(def.PagerdutyConfigs, &pagerduty_v0mimir1.Config{
			NotifierConfig: c.NotifierConfig,
			HTTPConfig:     httpcfg.FromCommonHTTPClientConfig(c.HTTPConfig),
			ServiceKey:     c.ServiceKey,
			ServiceKeyFile: c.ServiceKeyFile,
			RoutingKey:     c.RoutingKey,
			RoutingKeyFile: c.RoutingKeyFile,
			URL:            c.URL,
			Client:         c.Client,
			ClientURL:      c.ClientURL,
			Description:    c.Description,
			Details:        c.Details,
			Images:         c.Images,
			Links:          c.Links,
			Source:         c.Source,
			Severity:       c.Severity,
			Class:          c.Class,
			Component:      c.Component,
			Group:          c.Group,
		})
	}

	for _, c := range r.SlackConfigs {
		def.SlackConfigs = append(def.SlackConfigs, &slack_v0mimir1.Config{
			NotifierConfig: c.NotifierConfig,
			HTTPConfig:     httpcfg.FromCommonHTTPClientConfig(c.HTTPConfig),
			APIURL:         c.APIURL,
			APIURLFile:     c.APIURLFile,
			Channel:        c.Channel,
			Username:       c.Username,
			Color:          c.Color,
			Title:          c.Title,
			TitleLink:      c.TitleLink,
			Pretext:        c.Pretext,
			Text:           c.Text,
			Fields:         c.Fields,
			ShortFields:    c.ShortFields,
			Footer:         c.Footer,
			Fallback:       c.Fallback,
			CallbackID:     c.CallbackID,
			IconEmoji:      c.IconEmoji,
			IconURL:        c.IconURL,
			ImageURL:       c.ImageURL,
			ThumbURL:       c.ThumbURL,
			LinkNames:      c.LinkNames,
			MrkdwnIn:       c.MrkdwnIn,
			Actions:        c.Actions,
		})
	}

	for _, c := range r.WebhookConfigs {
		def.WebhookConfigs = append(def.WebhookConfigs, &webhook_v0mimir1.Config{
			NotifierConfig: c.NotifierConfig,
			HTTPConfig:     httpcfg.FromCommonHTTPClientConfig(c.HTTPConfig),
			URL:            c.URL,
			URLFile:        c.URLFile,
			MaxAlerts:      c.MaxAlerts,
			Timeout:        model.Duration(c.Timeout),
		})
	}

	for _, c := range r.OpsGenieConfigs {
		responders := make([]opsgenie_v0mimir1.Responder, len(c.Responders))
		for i, r := range c.Responders {
			responders[i] = opsgenie_v0mimir1.Responder{
				ID:       r.ID,
				Name:     r.Name,
				Username: r.Username,
				Type:     r.Type,
			}
		}
		def.OpsGenieConfigs = append(def.OpsGenieConfigs, &opsgenie_v0mimir1.Config{
			NotifierConfig: c.NotifierConfig,
			HTTPConfig:     httpcfg.FromCommonHTTPClientConfig(c.HTTPConfig),
			APIKey:         c.APIKey,
			APIKeyFile:     c.APIKeyFile,
			APIURL:         c.APIURL,
			Message:        c.Message,
			Description:    c.Description,
			Source:         c.Source,
			Details:        c.Details,
			Entity:         c.Entity,
			Responders:     responders,
			Actions:        c.Actions,
			Tags:           c.Tags,
			Note:           c.Note,
			Priority:       c.Priority,
			UpdateAlerts:   c.UpdateAlerts,
		})
	}

	for _, c := range r.WechatConfigs {
		def.WechatConfigs = append(def.WechatConfigs, &wechat_v0mimir1.Config{
			NotifierConfig: c.NotifierConfig,
			HTTPConfig:     httpcfg.FromCommonHTTPClientConfig(c.HTTPConfig),
			APISecret:      c.APISecret,
			CorpID:         c.CorpID,
			Message:        c.Message,
			APIURL:         c.APIURL,
			ToUser:         c.ToUser,
			ToParty:        c.ToParty,
			ToTag:          c.ToTag,
			AgentID:        c.AgentID,
			MessageType:    c.MessageType,
		})
	}

	for _, c := range r.PushoverConfigs {
		def.PushoverConfigs = append(def.PushoverConfigs, &pushover_v0mimir1.Config{
			NotifierConfig: c.NotifierConfig,
			HTTPConfig:     httpcfg.FromCommonHTTPClientConfig(c.HTTPConfig),
			UserKey:        c.UserKey,
			UserKeyFile:    c.UserKeyFile,
			Token:          c.Token,
			TokenFile:      c.TokenFile,
			Title:          c.Title,
			Message:        c.Message,
			URL:            c.URL,
			URLTitle:       c.URLTitle,
			Device:         c.Device,
			Sound:          c.Sound,
			Priority:       c.Priority,
			Retry:          model.Duration(c.Retry),
			Expire:         model.Duration(c.Expire),
			TTL:            model.Duration(c.TTL),
			HTML:           c.HTML,
		})
	}

	for _, c := range r.VictorOpsConfigs {
		def.VictorOpsConfigs = append(def.VictorOpsConfigs, &victorops_v0mimir1.Config{
			NotifierConfig:    c.NotifierConfig,
			HTTPConfig:        httpcfg.FromCommonHTTPClientConfig(c.HTTPConfig),
			APIKey:            c.APIKey,
			APIKeyFile:        c.APIKeyFile,
			APIURL:            c.APIURL,
			RoutingKey:        c.RoutingKey,
			MessageType:       c.MessageType,
			StateMessage:      c.StateMessage,
			EntityDisplayName: c.EntityDisplayName,
			MonitoringTool:    c.MonitoringTool,
			CustomFields:      c.CustomFields,
		})
	}

	for _, c := range r.SNSConfigs {
		def.SNSConfigs = append(def.SNSConfigs, &sns_v0mimir1.Config{
			NotifierConfig: c.NotifierConfig,
			HTTPConfig:     httpcfg.FromCommonHTTPClientConfig(c.HTTPConfig),
			APIUrl:         c.APIUrl,
			Sigv4:          c.Sigv4,
			TopicARN:       c.TopicARN,
			PhoneNumber:    c.PhoneNumber,
			TargetARN:      c.TargetARN,
			Subject:        c.Subject,
			Message:        c.Message,
			Attributes:     c.Attributes,
		})
	}

	for _, c := range r.TelegramConfigs {
		def.TelegramConfigs = append(def.TelegramConfigs, &telegram_v0mimir1.Config{
			NotifierConfig:       c.NotifierConfig,
			HTTPConfig:           httpcfg.FromCommonHTTPClientConfig(c.HTTPConfig),
			APIUrl:               c.APIUrl,
			BotToken:             c.BotToken,
			BotTokenFile:         c.BotTokenFile,
			ChatID:               c.ChatID,
			Message:              c.Message,
			DisableNotifications: c.DisableNotifications,
			ParseMode:            c.ParseMode,
		})
	}

	for _, c := range r.WebexConfigs {
		def.WebexConfigs = append(def.WebexConfigs, &webex_v0mimir1.Config{
			NotifierConfig: c.NotifierConfig,
			HTTPConfig:     httpcfg.FromCommonHTTPClientConfig(c.HTTPConfig),
			APIURL:         c.APIURL,
			Message:        c.Message,
			RoomID:         c.RoomID,
		})
	}

	for _, c := range r.MSTeamsConfigs {
		def.MSTeamsConfigs = append(def.MSTeamsConfigs, &teams_v0mimir1.Config{
			NotifierConfig: c.NotifierConfig,
			HTTPConfig:     httpcfg.FromCommonHTTPClientConfig(c.HTTPConfig),
			WebhookURL:     c.WebhookURL,
			WebhookURLFile: c.WebhookURLFile,
			Title:          c.Title,
			Summary:        c.Summary,
			Text:           c.Text,
		})
	}

	for _, c := range r.MSTeamsV2Configs {
		def.MSTeamsV2Configs = append(def.MSTeamsV2Configs, &teams_v0mimir2.Config{
			NotifierConfig: c.NotifierConfig,
			HTTPConfig:     httpcfg.FromCommonHTTPClientConfig(c.HTTPConfig),
			WebhookURL:     c.WebhookURL,
			WebhookURLFile: c.WebhookURLFile,
			Title:          c.Title,
			Text:           c.Text,
		})
	}

	for _, c := range r.JiraConfigs {
		def.JiraConfigs = append(def.JiraConfigs, &jira_v0mimir1.Config{
			NotifierConfig:    c.NotifierConfig,
			HTTPConfig:        httpcfg.FromCommonHTTPClientConfig(c.HTTPConfig),
			APIURL:            c.APIURL,
			Project:           c.Project,
			Summary:           c.Summary,
			Description:       c.Description,
			Labels:            c.Labels,
			Priority:          c.Priority,
			IssueType:         c.IssueType,
			ReopenTransition:  c.ReopenTransition,
			ResolveTransition: c.ResolveTransition,
			WontFixResolution: c.WontFixResolution,
			ReopenDuration:    c.ReopenDuration,
			Fields:            c.Fields,
		})
	}

	return def
}

// DefinitionReceiverToUpstreamReceiver converts a definition.Receiver to an upstream alertmanager config.Receiver.
func DefinitionReceiverToUpstreamReceiver(r definition.Receiver) config.Receiver {
	upstream := config.Receiver{Name: r.Name}

	for _, c := range r.DiscordConfigs {
		upstream.DiscordConfigs = append(upstream.DiscordConfigs, &config.DiscordConfig{
			NotifierConfig: c.NotifierConfig,
			HTTPConfig:     c.HTTPConfig.ToCommonHTTPClientConfig(),
			WebhookURL:     c.WebhookURL,
			WebhookURLFile: c.WebhookURLFile,
			Title:          c.Title,
			Message:        c.Message,
		})
	}

	for _, c := range r.EmailConfigs {
		upstream.EmailConfigs = append(upstream.EmailConfigs, &config.EmailConfig{
			NotifierConfig:   c.NotifierConfig,
			To:               c.To,
			From:             c.From,
			Hello:            c.Hello,
			Smarthost:        c.Smarthost,
			AuthUsername:     c.AuthUsername,
			AuthPassword:     c.AuthPassword,
			AuthPasswordFile: c.AuthPasswordFile,
			AuthSecret:       c.AuthSecret,
			AuthIdentity:     c.AuthIdentity,
			Headers:          c.Headers,
			HTML:             c.HTML,
			Text:             c.Text,
			RequireTLS:       c.RequireTLS,
			TLSConfig:        httpcfg.ToCommonTLSConfig(c.TLSConfig),
		})
	}

	for _, c := range r.PagerdutyConfigs {
		upstream.PagerdutyConfigs = append(upstream.PagerdutyConfigs, &config.PagerdutyConfig{
			NotifierConfig: c.NotifierConfig,
			HTTPConfig:     c.HTTPConfig.ToCommonHTTPClientConfig(),
			ServiceKey:     c.ServiceKey,
			ServiceKeyFile: c.ServiceKeyFile,
			RoutingKey:     c.RoutingKey,
			RoutingKeyFile: c.RoutingKeyFile,
			URL:            c.URL,
			Client:         c.Client,
			ClientURL:      c.ClientURL,
			Description:    c.Description,
			Details:        c.Details,
			Images:         c.Images,
			Links:          c.Links,
			Source:         c.Source,
			Severity:       c.Severity,
			Class:          c.Class,
			Component:      c.Component,
			Group:          c.Group,
		})
	}

	for _, c := range r.SlackConfigs {
		upstream.SlackConfigs = append(upstream.SlackConfigs, &config.SlackConfig{
			NotifierConfig: c.NotifierConfig,
			HTTPConfig:     c.HTTPConfig.ToCommonHTTPClientConfig(),
			APIURL:         c.APIURL,
			APIURLFile:     c.APIURLFile,
			Channel:        c.Channel,
			Username:       c.Username,
			Color:          c.Color,
			Title:          c.Title,
			TitleLink:      c.TitleLink,
			Pretext:        c.Pretext,
			Text:           c.Text,
			Fields:         c.Fields,
			ShortFields:    c.ShortFields,
			Footer:         c.Footer,
			Fallback:       c.Fallback,
			CallbackID:     c.CallbackID,
			IconEmoji:      c.IconEmoji,
			IconURL:        c.IconURL,
			ImageURL:       c.ImageURL,
			ThumbURL:       c.ThumbURL,
			LinkNames:      c.LinkNames,
			MrkdwnIn:       c.MrkdwnIn,
			Actions:        c.Actions,
		})
	}

	for _, c := range r.WebhookConfigs {
		cfg := &config.WebhookConfig{
			NotifierConfig: c.NotifierConfig,
			HTTPConfig:     c.HTTPConfig.ToCommonHTTPClientConfig(),
			URL:            c.URL,
			URLFile:        c.URLFile,
			MaxAlerts:      c.MaxAlerts,
			Timeout:        c.Timeout,
		}
		upstream.WebhookConfigs = append(upstream.WebhookConfigs, cfg)
	}

	for _, c := range r.OpsGenieConfigs {
		responders := make([]config.OpsGenieConfigResponder, len(c.Responders))
		for i, r := range c.Responders {
			responders[i] = config.OpsGenieConfigResponder{
				ID:       r.ID,
				Name:     r.Name,
				Username: r.Username,
				Type:     r.Type,
			}
		}
		upstream.OpsGenieConfigs = append(upstream.OpsGenieConfigs, &config.OpsGenieConfig{
			NotifierConfig: c.NotifierConfig,
			HTTPConfig:     c.HTTPConfig.ToCommonHTTPClientConfig(),
			APIKey:         c.APIKey,
			APIKeyFile:     c.APIKeyFile,
			APIURL:         c.APIURL,
			Message:        c.Message,
			Description:    c.Description,
			Source:         c.Source,
			Details:        c.Details,
			Entity:         c.Entity,
			Responders:     responders,
			Actions:        c.Actions,
			Tags:           c.Tags,
			Note:           c.Note,
			Priority:       c.Priority,
			UpdateAlerts:   c.UpdateAlerts,
		})
	}

	for _, c := range r.WechatConfigs {
		upstream.WechatConfigs = append(upstream.WechatConfigs, &config.WechatConfig{
			NotifierConfig: c.NotifierConfig,
			HTTPConfig:     c.HTTPConfig.ToCommonHTTPClientConfig(),
			APISecret:      c.APISecret,
			CorpID:         c.CorpID,
			Message:        c.Message,
			APIURL:         c.APIURL,
			ToUser:         c.ToUser,
			ToParty:        c.ToParty,
			ToTag:          c.ToTag,
			AgentID:        c.AgentID,
			MessageType:    c.MessageType,
		})
	}

	for _, c := range r.PushoverConfigs {
		cfg := &config.PushoverConfig{
			NotifierConfig: c.NotifierConfig,
			HTTPConfig:     c.HTTPConfig.ToCommonHTTPClientConfig(),
			UserKey:        c.UserKey,
			UserKeyFile:    c.UserKeyFile,
			Token:          c.Token,
			TokenFile:      c.TokenFile,
			Title:          c.Title,
			Message:        c.Message,
			URL:            c.URL,
			URLTitle:       c.URLTitle,
			Device:         c.Device,
			Sound:          c.Sound,
			Priority:       c.Priority,
			HTML:           c.HTML,
		}
		setConfigDuration(&cfg.Retry, c.Retry)
		setConfigDuration(&cfg.Expire, c.Expire)
		setConfigDuration(&cfg.TTL, c.TTL)
		upstream.PushoverConfigs = append(upstream.PushoverConfigs, cfg)
	}

	for _, c := range r.VictorOpsConfigs {
		upstream.VictorOpsConfigs = append(upstream.VictorOpsConfigs, &config.VictorOpsConfig{
			NotifierConfig:    c.NotifierConfig,
			HTTPConfig:        c.HTTPConfig.ToCommonHTTPClientConfig(),
			APIKey:            c.APIKey,
			APIKeyFile:        c.APIKeyFile,
			APIURL:            c.APIURL,
			RoutingKey:        c.RoutingKey,
			MessageType:       c.MessageType,
			StateMessage:      c.StateMessage,
			EntityDisplayName: c.EntityDisplayName,
			MonitoringTool:    c.MonitoringTool,
			CustomFields:      c.CustomFields,
		})
	}

	for _, c := range r.SNSConfigs {
		upstream.SNSConfigs = append(upstream.SNSConfigs, &config.SNSConfig{
			NotifierConfig: c.NotifierConfig,
			HTTPConfig:     c.HTTPConfig.ToCommonHTTPClientConfig(),
			APIUrl:         c.APIUrl,
			Sigv4:          c.Sigv4,
			TopicARN:       c.TopicARN,
			PhoneNumber:    c.PhoneNumber,
			TargetARN:      c.TargetARN,
			Subject:        c.Subject,
			Message:        c.Message,
			Attributes:     c.Attributes,
		})
	}

	for _, c := range r.TelegramConfigs {
		upstream.TelegramConfigs = append(upstream.TelegramConfigs, &config.TelegramConfig{
			NotifierConfig:       c.NotifierConfig,
			HTTPConfig:           c.HTTPConfig.ToCommonHTTPClientConfig(),
			APIUrl:               c.APIUrl,
			BotToken:             c.BotToken,
			BotTokenFile:         c.BotTokenFile,
			ChatID:               c.ChatID,
			Message:              c.Message,
			DisableNotifications: c.DisableNotifications,
			ParseMode:            c.ParseMode,
		})
	}

	for _, c := range r.WebexConfigs {
		upstream.WebexConfigs = append(upstream.WebexConfigs, &config.WebexConfig{
			NotifierConfig: c.NotifierConfig,
			HTTPConfig:     c.HTTPConfig.ToCommonHTTPClientConfig(),
			APIURL:         c.APIURL,
			Message:        c.Message,
			RoomID:         c.RoomID,
		})
	}

	for _, c := range r.MSTeamsConfigs {
		upstream.MSTeamsConfigs = append(upstream.MSTeamsConfigs, &config.MSTeamsConfig{
			NotifierConfig: c.NotifierConfig,
			HTTPConfig:     c.HTTPConfig.ToCommonHTTPClientConfig(),
			WebhookURL:     c.WebhookURL,
			WebhookURLFile: c.WebhookURLFile,
			Title:          c.Title,
			Summary:        c.Summary,
			Text:           c.Text,
		})
	}

	for _, c := range r.MSTeamsV2Configs {
		upstream.MSTeamsV2Configs = append(upstream.MSTeamsV2Configs, &config.MSTeamsV2Config{
			NotifierConfig: c.NotifierConfig,
			HTTPConfig:     c.HTTPConfig.ToCommonHTTPClientConfig(),
			WebhookURL:     c.WebhookURL,
			WebhookURLFile: c.WebhookURLFile,
			Title:          c.Title,
			Text:           c.Text,
		})
	}

	for _, c := range r.JiraConfigs {
		upstream.JiraConfigs = append(upstream.JiraConfigs, &config.JiraConfig{
			NotifierConfig:    c.NotifierConfig,
			HTTPConfig:        c.HTTPConfig.ToCommonHTTPClientConfig(),
			APIURL:            c.APIURL,
			Project:           c.Project,
			Summary:           c.Summary,
			Description:       c.Description,
			Labels:            c.Labels,
			Priority:          c.Priority,
			IssueType:         c.IssueType,
			ReopenTransition:  c.ReopenTransition,
			ResolveTransition: c.ResolveTransition,
			WontFixResolution: c.WontFixResolution,
			ReopenDuration:    c.ReopenDuration,
			Fields:            c.Fields,
		})
	}

	return upstream
}

// setConfigDuration sets a duration field in an upstream alertmanager config struct via reflection.
// The upstream alertmanager defines an unexported 'duration' type (underlying type int64) that
// cannot be assigned to directly from outside the package.
func setConfigDuration(ptr any, d model.Duration) {
	v := reflect.ValueOf(ptr)
	if !v.IsValid() || v.Kind() != reflect.Pointer || v.IsNil() {
		return
	}
	v.Elem().SetInt(int64(d))
}
