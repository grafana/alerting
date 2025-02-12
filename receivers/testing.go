package receivers

import (
	"context"
)

type NotificationServiceMock struct {
	WebhookCalls []SendWebhookSettings
	Webhook      SendWebhookSettings
	EmailSync    SendEmailSettings
	ShouldError  error
	ResponseBody []byte
	StatusCode   int
}

func (ns *NotificationServiceMock) SendWebhook(_ context.Context, cmd *SendWebhookSettings) error {
	ns.WebhookCalls = append(ns.WebhookCalls, *cmd)
	ns.Webhook = *cmd

	if cmd.Validation != nil && ns.ResponseBody != nil && ns.StatusCode != 0 {
		ns.ShouldError = cmd.Validation(ns.ResponseBody, 200)
	}

	return ns.ShouldError
}

func (ns *NotificationServiceMock) SendEmail(_ context.Context, cmd *SendEmailSettings) error {
	ns.EmailSync = *cmd
	return ns.ShouldError
}

func MockNotificationService() *NotificationServiceMock { return &NotificationServiceMock{} }
