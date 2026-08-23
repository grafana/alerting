package v1

import (
	"encoding/json"
	"mime/multipart"
	"testing"

	"github.com/stretchr/testify/require"

	receiversTesting "github.com/grafana/alerting/receivers/testing"
)

func TestNewConfigAPIURL(t *testing.T) {
	t.Run("accepts and normalizes an absolute API URL", func(t *testing.T) {
		cfg, err := NewConfig(
			json.RawMessage(`{"bottoken":"test-token","chatid":"12345678","api_url":"http://localhost:8081/telegram/"}`),
			receiversTesting.DecryptForTesting(nil),
		)

		require.NoError(t, err)
		require.Equal(t, "http://localhost:8081/telegram", cfg.APIURL)
	})

	t.Run("rejects a relative API URL", func(t *testing.T) {
		_, err := NewConfig(
			json.RawMessage(`{"bottoken":"test-token","chatid":"12345678","api_url":"telegram.local"}`),
			receiversTesting.DecryptForTesting(nil),
		)

		require.ErrorContains(t, err, "invalid Telegram API URL")
	})

	t.Run("rejects a non-HTTP API URL", func(t *testing.T) {
		_, err := NewConfig(
			json.RawMessage(`{"bottoken":"test-token","chatid":"12345678","api_url":"ftp://telegram.local"}`),
			receiversTesting.DecryptForTesting(nil),
		)

		require.ErrorContains(t, err, "invalid Telegram API URL")
	})
}

func TestNewWebhookSyncCmdAPIURL(t *testing.T) {
	noop := func(*multipart.Writer) error { return nil }

	t.Run("uses the configured API URL", func(t *testing.T) {
		n := &Notifier{settings: Config{
			APIURL:   "http://localhost:8081/telegram",
			BotToken: "test-token",
			ChatID:   "12345678",
		}}

		cmd, err := n.newWebhookSyncCmd("sendMessage", noop)

		require.NoError(t, err)
		require.Equal(t, "http://localhost:8081/telegram/bottest-token/sendMessage", cmd.URL)
	})

	t.Run("keeps the default integration-test override when API URL is unset", func(t *testing.T) {
		previousAPIURL := APIURL
		APIURL = "http://telegram.test/bot%s/%s"
		t.Cleanup(func() { APIURL = previousAPIURL })

		n := &Notifier{settings: Config{
			BotToken: "test-token",
			ChatID:   "12345678",
		}}

		cmd, err := n.newWebhookSyncCmd("sendPhoto", noop)

		require.NoError(t, err)
		require.Equal(t, "http://telegram.test/bottest-token/sendPhoto", cmd.URL)
	})
}
