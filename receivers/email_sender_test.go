package receivers

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmbedTemplate(t *testing.T) {
	// Test the email template is embedded and parsed correctly.
	require.NotEmpty(t, defaultEmailTemplate)

	_, err := NewEmailSenderFactory(EmailSenderConfig{})(Metadata{})
	require.NoError(t, err)
}
