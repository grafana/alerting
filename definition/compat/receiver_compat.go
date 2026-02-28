package compat

import (
	"reflect"

	"github.com/prometheus/alertmanager/config"

	"github.com/grafana/alerting/definition"
	httpcfg "github.com/grafana/alerting/http/v0mimir1"
	"github.com/grafana/alerting/receivers"
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
			NotifierConfig: receivers.NotifierConfig(c.NotifierConfig),
			HTTPConfig:     httpcfg.FromCommonHTTPClientConfig(c.HTTPConfig),
			WebhookURL:     (*receivers.SecretURL)(c.WebhookURL),
			WebhookURLFile: c.WebhookURLFile,
			Title:          c.Title,
			Message:        c.Message,
		})
	}

	for _, c := range r.EmailConfigs {
		def.EmailConfigs = append(def.EmailConfigs, &email_v0mimir1.Config{
			NotifierConfig:   receivers.NotifierConfig(c.NotifierConfig),
			To:               c.To,
			From:             c.From,
			Hello:            c.Hello,
			Smarthost:        receivers.HostPort(c.Smarthost),
			AuthUsername:     c.AuthUsername,
			AuthPassword:     receivers.Secret(c.AuthPassword),
			AuthPasswordFile: c.AuthPasswordFile,
			AuthSecret:       receivers.Secret(c.AuthSecret),
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
			NotifierConfig: receivers.NotifierConfig(c.NotifierConfig),
			HTTPConfig:     httpcfg.FromCommonHTTPClientConfig(c.HTTPConfig),
			ServiceKey:     receivers.Secret(c.ServiceKey),
			ServiceKeyFile: c.ServiceKeyFile,
			RoutingKey:     receivers.Secret(c.RoutingKey),
			RoutingKeyFile: c.RoutingKeyFile,
			URL:            (*receivers.URL)(c.URL),
			Client:         c.Client,
			ClientURL:      c.ClientURL,
			Description:    c.Description,
			Details:        c.Details,
			Images:         pagerdutyImagesToLocal(c.Images),
			Links:          pagerdutyLinksToLocal(c.Links),
			Source:         c.Source,
			Severity:       c.Severity,
			Class:          c.Class,
			Component:      c.Component,
			Group:          c.Group,
		})
	}

	for _, c := range r.SlackConfigs {
		def.SlackConfigs = append(def.SlackConfigs, &slack_v0mimir1.Config{
			NotifierConfig: receivers.NotifierConfig(c.NotifierConfig),
			HTTPConfig:     httpcfg.FromCommonHTTPClientConfig(c.HTTPConfig),
			APIURL:         (*receivers.SecretURL)(c.APIURL),
			APIURLFile:     c.APIURLFile,
			Channel:        c.Channel,
			Username:       c.Username,
			Color:          c.Color,
			Title:          c.Title,
			TitleLink:      c.TitleLink,
			Pretext:        c.Pretext,
			Text:           c.Text,
			Fields:         slackFieldsToLocal(c.Fields),
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
			Actions:        slackActionsToLocal(c.Actions),
		})
	}

	for _, c := range r.WebhookConfigs {
		def.WebhookConfigs = append(def.WebhookConfigs, &webhook_v0mimir1.Config{
			NotifierConfig: receivers.NotifierConfig(c.NotifierConfig),
			HTTPConfig:     httpcfg.FromCommonHTTPClientConfig(c.HTTPConfig),
			URL:            (*receivers.SecretURL)(c.URL),
			URLFile:        c.URLFile,
			MaxAlerts:      c.MaxAlerts,
			Timeout:        c.Timeout,
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
			NotifierConfig: receivers.NotifierConfig(c.NotifierConfig),
			HTTPConfig:     httpcfg.FromCommonHTTPClientConfig(c.HTTPConfig),
			APIKey:         receivers.Secret(c.APIKey),
			APIKeyFile:     c.APIKeyFile,
			APIURL:         (*receivers.URL)(c.APIURL),
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
			NotifierConfig: receivers.NotifierConfig(c.NotifierConfig),
			HTTPConfig:     httpcfg.FromCommonHTTPClientConfig(c.HTTPConfig),
			APISecret:      receivers.Secret(c.APISecret),
			CorpID:         c.CorpID,
			Message:        c.Message,
			APIURL:         (*receivers.URL)(c.APIURL),
			ToUser:         c.ToUser,
			ToParty:        c.ToParty,
			ToTag:          c.ToTag,
			AgentID:        c.AgentID,
			MessageType:    c.MessageType,
		})
	}

	for _, c := range r.PushoverConfigs {
		def.PushoverConfigs = append(def.PushoverConfigs, &pushover_v0mimir1.Config{
			NotifierConfig: receivers.NotifierConfig(c.NotifierConfig),
			HTTPConfig:     httpcfg.FromCommonHTTPClientConfig(c.HTTPConfig),
			UserKey:        receivers.Secret(c.UserKey),
			UserKeyFile:    c.UserKeyFile,
			Token:          receivers.Secret(c.Token),
			TokenFile:      c.TokenFile,
			Title:          c.Title,
			Message:        c.Message,
			URL:            c.URL,
			URLTitle:       c.URLTitle,
			Device:         c.Device,
			Sound:          c.Sound,
			Priority:       c.Priority,
			Retry:          pushover_v0mimir1.FractionalDuration(c.Retry),
			Expire:         pushover_v0mimir1.FractionalDuration(c.Expire),
			TTL:            pushover_v0mimir1.FractionalDuration(c.TTL),
			HTML:           c.HTML,
		})
	}

	for _, c := range r.VictorOpsConfigs {
		def.VictorOpsConfigs = append(def.VictorOpsConfigs, &victorops_v0mimir1.Config{
			NotifierConfig:    receivers.NotifierConfig(c.NotifierConfig),
			HTTPConfig:        httpcfg.FromCommonHTTPClientConfig(c.HTTPConfig),
			APIKey:            receivers.Secret(c.APIKey),
			APIKeyFile:        c.APIKeyFile,
			APIURL:            (*receivers.URL)(c.APIURL),
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
			NotifierConfig: receivers.NotifierConfig(c.NotifierConfig),
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
			NotifierConfig:       receivers.NotifierConfig(c.NotifierConfig),
			HTTPConfig:           httpcfg.FromCommonHTTPClientConfig(c.HTTPConfig),
			APIUrl:               (*receivers.URL)(c.APIUrl),
			BotToken:             receivers.Secret(c.BotToken),
			BotTokenFile:         c.BotTokenFile,
			ChatID:               c.ChatID,
			Message:              c.Message,
			DisableNotifications: c.DisableNotifications,
			ParseMode:            c.ParseMode,
		})
	}

	for _, c := range r.WebexConfigs {
		def.WebexConfigs = append(def.WebexConfigs, &webex_v0mimir1.Config{
			NotifierConfig: receivers.NotifierConfig(c.NotifierConfig),
			HTTPConfig:     httpcfg.FromCommonHTTPClientConfig(c.HTTPConfig),
			APIURL:         (*receivers.URL)(c.APIURL),
			Message:        c.Message,
			RoomID:         c.RoomID,
		})
	}

	for _, c := range r.MSTeamsConfigs {
		def.MSTeamsConfigs = append(def.MSTeamsConfigs, &teams_v0mimir1.Config{
			NotifierConfig: receivers.NotifierConfig(c.NotifierConfig),
			HTTPConfig:     httpcfg.FromCommonHTTPClientConfig(c.HTTPConfig),
			WebhookURL:     (*receivers.SecretURL)(c.WebhookURL),
			WebhookURLFile: c.WebhookURLFile,
			Title:          c.Title,
			Summary:        c.Summary,
			Text:           c.Text,
		})
	}

	for _, c := range r.MSTeamsV2Configs {
		def.MSTeamsV2Configs = append(def.MSTeamsV2Configs, &teams_v0mimir2.Config{
			NotifierConfig: receivers.NotifierConfig(c.NotifierConfig),
			HTTPConfig:     httpcfg.FromCommonHTTPClientConfig(c.HTTPConfig),
			WebhookURL:     (*receivers.SecretURL)(c.WebhookURL),
			WebhookURLFile: c.WebhookURLFile,
			Title:          c.Title,
			Text:           c.Text,
		})
	}

	for _, c := range r.JiraConfigs {
		def.JiraConfigs = append(def.JiraConfigs, &jira_v0mimir1.Config{
			NotifierConfig:    receivers.NotifierConfig(c.NotifierConfig),
			HTTPConfig:        httpcfg.FromCommonHTTPClientConfig(c.HTTPConfig),
			APIURL:            (*receivers.URL)(c.APIURL),
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
			NotifierConfig: config.NotifierConfig(c.NotifierConfig),
			HTTPConfig:     c.HTTPConfig.ToCommonHTTPClientConfig(),
			WebhookURL:     (*config.SecretURL)(c.WebhookURL),
			WebhookURLFile: c.WebhookURLFile,
			Title:          c.Title,
			Message:        c.Message,
		})
	}

	for _, c := range r.EmailConfigs {
		upstream.EmailConfigs = append(upstream.EmailConfigs, &config.EmailConfig{
			NotifierConfig:   config.NotifierConfig(c.NotifierConfig),
			To:               c.To,
			From:             c.From,
			Hello:            c.Hello,
			Smarthost:        config.HostPort(c.Smarthost),
			AuthUsername:     c.AuthUsername,
			AuthPassword:     config.Secret(c.AuthPassword),
			AuthPasswordFile: c.AuthPasswordFile,
			AuthSecret:       config.Secret(c.AuthSecret),
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
			NotifierConfig: config.NotifierConfig(c.NotifierConfig),
			HTTPConfig:     c.HTTPConfig.ToCommonHTTPClientConfig(),
			ServiceKey:     config.Secret(c.ServiceKey),
			ServiceKeyFile: c.ServiceKeyFile,
			RoutingKey:     config.Secret(c.RoutingKey),
			RoutingKeyFile: c.RoutingKeyFile,
			URL:            (*config.URL)(c.URL),
			Client:         c.Client,
			ClientURL:      c.ClientURL,
			Description:    c.Description,
			Details:        c.Details,
			Images:         pagerdutyImagesToUpstream(c.Images),
			Links:          pagerdutyLinksToUpstream(c.Links),
			Source:         c.Source,
			Severity:       c.Severity,
			Class:          c.Class,
			Component:      c.Component,
			Group:          c.Group,
		})
	}

	for _, c := range r.SlackConfigs {
		upstream.SlackConfigs = append(upstream.SlackConfigs, &config.SlackConfig{
			NotifierConfig: config.NotifierConfig(c.NotifierConfig),
			HTTPConfig:     c.HTTPConfig.ToCommonHTTPClientConfig(),
			APIURL:         (*config.SecretURL)(c.APIURL),
			APIURLFile:     c.APIURLFile,
			Channel:        c.Channel,
			Username:       c.Username,
			Color:          c.Color,
			Title:          c.Title,
			TitleLink:      c.TitleLink,
			Pretext:        c.Pretext,
			Text:           c.Text,
			Fields:         slackFieldsToUpstream(c.Fields),
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
			Actions:        slackActionsToUpstream(c.Actions),
		})
	}

	for _, c := range r.WebhookConfigs {
		cfg := &config.WebhookConfig{
			NotifierConfig: config.NotifierConfig(c.NotifierConfig),
			HTTPConfig:     c.HTTPConfig.ToCommonHTTPClientConfig(),
			URL:            (*config.SecretURL)(c.URL),
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
			NotifierConfig: config.NotifierConfig(c.NotifierConfig),
			HTTPConfig:     c.HTTPConfig.ToCommonHTTPClientConfig(),
			APIKey:         config.Secret(c.APIKey),
			APIKeyFile:     c.APIKeyFile,
			APIURL:         (*config.URL)(c.APIURL),
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
			NotifierConfig: config.NotifierConfig(c.NotifierConfig),
			HTTPConfig:     c.HTTPConfig.ToCommonHTTPClientConfig(),
			APISecret:      config.Secret(c.APISecret),
			CorpID:         c.CorpID,
			Message:        c.Message,
			APIURL:         (*config.URL)(c.APIURL),
			ToUser:         c.ToUser,
			ToParty:        c.ToParty,
			ToTag:          c.ToTag,
			AgentID:        c.AgentID,
			MessageType:    c.MessageType,
		})
	}

	for _, c := range r.PushoverConfigs {
		cfg := &config.PushoverConfig{
			NotifierConfig: config.NotifierConfig(c.NotifierConfig),
			HTTPConfig:     c.HTTPConfig.ToCommonHTTPClientConfig(),
			UserKey:        config.Secret(c.UserKey),
			UserKeyFile:    c.UserKeyFile,
			Token:          config.Secret(c.Token),
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
			NotifierConfig:    config.NotifierConfig(c.NotifierConfig),
			HTTPConfig:        c.HTTPConfig.ToCommonHTTPClientConfig(),
			APIKey:            config.Secret(c.APIKey),
			APIKeyFile:        c.APIKeyFile,
			APIURL:            (*config.URL)(c.APIURL),
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
			NotifierConfig: config.NotifierConfig(c.NotifierConfig),
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
			NotifierConfig:       config.NotifierConfig(c.NotifierConfig),
			HTTPConfig:           c.HTTPConfig.ToCommonHTTPClientConfig(),
			APIUrl:               (*config.URL)(c.APIUrl),
			BotToken:             config.Secret(c.BotToken),
			BotTokenFile:         c.BotTokenFile,
			ChatID:               c.ChatID,
			Message:              c.Message,
			DisableNotifications: c.DisableNotifications,
			ParseMode:            c.ParseMode,
		})
	}

	for _, c := range r.WebexConfigs {
		upstream.WebexConfigs = append(upstream.WebexConfigs, &config.WebexConfig{
			NotifierConfig: config.NotifierConfig(c.NotifierConfig),
			HTTPConfig:     c.HTTPConfig.ToCommonHTTPClientConfig(),
			APIURL:         (*config.URL)(c.APIURL),
			Message:        c.Message,
			RoomID:         c.RoomID,
		})
	}

	for _, c := range r.MSTeamsConfigs {
		upstream.MSTeamsConfigs = append(upstream.MSTeamsConfigs, &config.MSTeamsConfig{
			NotifierConfig: config.NotifierConfig(c.NotifierConfig),
			HTTPConfig:     c.HTTPConfig.ToCommonHTTPClientConfig(),
			WebhookURL:     (*config.SecretURL)(c.WebhookURL),
			WebhookURLFile: c.WebhookURLFile,
			Title:          c.Title,
			Summary:        c.Summary,
			Text:           c.Text,
		})
	}

	for _, c := range r.MSTeamsV2Configs {
		upstream.MSTeamsV2Configs = append(upstream.MSTeamsV2Configs, &config.MSTeamsV2Config{
			NotifierConfig: config.NotifierConfig(c.NotifierConfig),
			HTTPConfig:     c.HTTPConfig.ToCommonHTTPClientConfig(),
			WebhookURL:     (*config.SecretURL)(c.WebhookURL),
			WebhookURLFile: c.WebhookURLFile,
			Title:          c.Title,
			Text:           c.Text,
		})
	}

	for _, c := range r.JiraConfigs {
		upstream.JiraConfigs = append(upstream.JiraConfigs, &config.JiraConfig{
			NotifierConfig:    config.NotifierConfig(c.NotifierConfig),
			HTTPConfig:        c.HTTPConfig.ToCommonHTTPClientConfig(),
			APIURL:            (*config.URL)(c.APIURL),
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
func setConfigDuration(ptr any, d pushover_v0mimir1.FractionalDuration) {
	v := reflect.ValueOf(ptr)
	if !v.IsValid() || v.Kind() != reflect.Pointer || v.IsNil() {
		return
	}
	v.Elem().SetInt(int64(d))
}

func pagerdutyImagesToLocal(images []config.PagerdutyImage) []pagerduty_v0mimir1.PagerdutyImage {
	if images == nil {
		return nil
	}
	out := make([]pagerduty_v0mimir1.PagerdutyImage, len(images))
	for i, img := range images {
		out[i] = pagerduty_v0mimir1.PagerdutyImage(img)
	}
	return out
}

func pagerdutyLinksToLocal(links []config.PagerdutyLink) []pagerduty_v0mimir1.PagerdutyLink {
	if links == nil {
		return nil
	}
	out := make([]pagerduty_v0mimir1.PagerdutyLink, len(links))
	for i, link := range links {
		out[i] = pagerduty_v0mimir1.PagerdutyLink(link)
	}
	return out
}

func pagerdutyImagesToUpstream(images []pagerduty_v0mimir1.PagerdutyImage) []config.PagerdutyImage {
	if images == nil {
		return nil
	}
	out := make([]config.PagerdutyImage, len(images))
	for i, img := range images {
		out[i] = config.PagerdutyImage(img)
	}
	return out
}

func pagerdutyLinksToUpstream(links []pagerduty_v0mimir1.PagerdutyLink) []config.PagerdutyLink {
	if links == nil {
		return nil
	}
	out := make([]config.PagerdutyLink, len(links))
	for i, link := range links {
		out[i] = config.PagerdutyLink(link)
	}
	return out
}

func slackFieldsToLocal(fields []*config.SlackField) []*slack_v0mimir1.SlackField {
	if fields == nil {
		return nil
	}
	out := make([]*slack_v0mimir1.SlackField, len(fields))
	for i, f := range fields {
		out[i] = (*slack_v0mimir1.SlackField)(f)
	}
	return out
}

func slackFieldsToUpstream(fields []*slack_v0mimir1.SlackField) []*config.SlackField {
	if fields == nil {
		return nil
	}
	out := make([]*config.SlackField, len(fields))
	for i, f := range fields {
		out[i] = (*config.SlackField)(f)
	}
	return out
}

func slackActionsToLocal(actions []*config.SlackAction) []*slack_v0mimir1.SlackAction {
	if actions == nil {
		return nil
	}
	out := make([]*slack_v0mimir1.SlackAction, len(actions))
	for i, a := range actions {
		out[i] = &slack_v0mimir1.SlackAction{
			Type:         a.Type,
			Text:         a.Text,
			URL:          a.URL,
			Style:        a.Style,
			Name:         a.Name,
			Value:        a.Value,
			ConfirmField: (*slack_v0mimir1.SlackConfirmationField)(a.ConfirmField),
		}
	}
	return out
}

func slackActionsToUpstream(actions []*slack_v0mimir1.SlackAction) []*config.SlackAction {
	if actions == nil {
		return nil
	}
	out := make([]*config.SlackAction, len(actions))
	for i, a := range actions {
		out[i] = &config.SlackAction{
			Type:         a.Type,
			Text:         a.Text,
			URL:          a.URL,
			Style:        a.Style,
			Name:         a.Name,
			Value:        a.Value,
			ConfirmField: (*config.SlackConfirmationField)(a.ConfirmField),
		}
	}
	return out
}
