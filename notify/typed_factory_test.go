package notify

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	slackV0 "github.com/grafana/alerting/receivers/slack/v0mimir1"
	slackV1 "github.com/grafana/alerting/receivers/slack/v1"
	receiversTesting "github.com/grafana/alerting/receivers/testing"
)

func TestGetFactoryForIntegration(t *testing.T) {
	t.Run("Grafana config", func(t *testing.T) {
		factory, err := GetFactoryForIntegration[slackV1.Config]()
		require.NoError(t, err)
		assert.Equal(t, slackV1.Type, factory.Type())
		assert.Equal(t, slackV1.Version, factory.Version())
		assert.Equal(t, reflect.TypeFor[slackV1.Config](), factory.ConfigType())

		raw := []byte(`{"url":"https://example.com/slack"}`)
		decrypt := receiversTesting.DecryptForTesting(nil)
		want, err := slackV1.NewConfig(raw, decrypt)
		require.NoError(t, err)
		got, err := factory.Parse(raw, decrypt)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("Mimir config", func(t *testing.T) {
		factory, err := GetFactoryForIntegration[slackV0.Config]()
		require.NoError(t, err)
		assert.Equal(t, slackV0.Type, factory.Type())
		assert.Equal(t, slackV0.Version, factory.Version())
	})

	t.Run("unknown config", func(t *testing.T) {
		factory, err := GetFactoryForIntegration[struct{}]()
		require.ErrorContains(t, err, "no factory registered")
		assert.Zero(t, factory)
	})

	t.Run("pointer config", func(t *testing.T) {
		_, err := GetFactoryForIntegration[*slackV1.Config]()
		require.ErrorContains(t, err, "no factory registered")
	})

	t.Run("distinct named config", func(t *testing.T) {
		type config slackV1.Config
		_, err := GetFactoryForIntegration[config]()
		require.ErrorContains(t, err, "no factory registered")
	})

	t.Run("interface config", func(t *testing.T) {
		_, err := GetFactoryForIntegration[any]()
		require.ErrorContains(t, err, "no factory registered")
	})
}
