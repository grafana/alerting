package v0mimir2

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestValidate(t *testing.T) {
	t.Run("GetFullValidConfig is valid", func(t *testing.T) {
		cfg := GetFullValidConfig()
		require.NoError(t, cfg.Validate())
	})
	t.Run("FullValidConfigForTesting is valid", func(t *testing.T) {
		var cfg Config
		err := yaml.UnmarshalStrict([]byte(FullValidConfigForTesting), &cfg)
		require.NoError(t, err)
		require.NoError(t, cfg.Validate())
	})
}
