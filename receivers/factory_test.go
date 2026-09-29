package receivers

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/receivers/schema"
)

func TestNewIntegrationVersionFactory(t *testing.T) {
	type config struct{ Token string }
	parseErr := errors.New("invalid config")
	buildErr := errors.New("cannot build notifier")
	for _, tc := range []struct {
		name     string
		parseErr error
		buildErr error
	}{
		{name: "success"},
		{name: "parser error", parseErr: parseErr},
		{name: "builder error", buildErr: buildErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := json.RawMessage(`{"token":"encrypted"}`)
			meta := Metadata{Name: "test"}
			opts := NotifierOpts{OrgID: 42}
			channel := &struct{ NotificationChannel }{}
			buildCalls := 0
			factory := NewIntegrationVersionFactory("test", schema.V1,
				func(got json.RawMessage, decrypt DecryptFunc) (config, error) {
					require.Equal(t, raw, got)
					token, ok := decrypt("token", "fallback")
					require.True(t, ok)
					return config{Token: token}, tc.parseErr
				},
				func(cfg config, gotMeta Metadata, gotOpts NotifierOpts) (NotificationChannel, error) {
					buildCalls++
					assert.Equal(t, config{Token: "decrypted"}, cfg)
					assert.Equal(t, meta, gotMeta)
					assert.Equal(t, opts, gotOpts)
					if tc.buildErr != nil {
						return nil, tc.buildErr
					}
					return channel, nil
				})
			assert.Equal(t, schema.IntegrationType("test"), factory.Type())
			assert.Equal(t, schema.V1, factory.Version())
			assert.Equal(t, reflect.TypeFor[config](), factory.ConfigType())
			decrypt := func(key, fallback string) (string, bool) {
				assert.Equal(t, "token", key)
				assert.Equal(t, "fallback", fallback)
				return "decrypted", true
			}
			parsed, parseErr := factory.Parse(raw, decrypt)
			assert.ErrorIs(t, parseErr, tc.parseErr)
			assert.Equal(t, "decrypted", parsed.Token)
			assert.Zero(t, buildCalls, "parsing must not build a notifier")
			assert.ErrorIs(t, factory.ValidateConfig(raw, decrypt), tc.parseErr)
			assert.Zero(t, buildCalls, "validation must not build a notifier")
			got, err := factory.NewNotifier(raw, decrypt, meta, opts)
			if tc.parseErr != nil {
				assert.ErrorIs(t, err, tc.parseErr)
				assert.Nil(t, got)
				assert.Zero(t, buildCalls, "parser errors must prevent notifier construction")
			} else {
				assert.ErrorIs(t, err, tc.buildErr)
				assert.Equal(t, 1, buildCalls)
				if tc.buildErr != nil {
					assert.Nil(t, got)
				} else {
					assert.Same(t, channel, got)
				}
			}
		})
	}
}

func testSchema(versions ...schema.Version) schema.IntegrationTypeSchema {
	s := schema.IntegrationTypeSchema{
		Type: "test",
		Name: "Test",
	}
	for _, v := range versions {
		s.Versions = append(s.Versions, schema.IntegrationSchemaVersion{Version: v})
	}
	return s
}

func testFactory(version schema.Version) VersionFactory {
	return NewIntegrationVersionFactory("test", version,
		func(json.RawMessage, DecryptFunc) (struct{}, error) { return struct{}{}, nil },
		func(struct{}, Metadata, NotifierOpts) (NotificationChannel, error) { return nil, nil },
	)
}

func TestNewManifest(t *testing.T) {
	t.Run("matching versions", func(t *testing.T) {
		s := testSchema(schema.V1, schema.V0mimir1)
		require.NotPanics(t, func() {
			m := NewManifest(s, testFactory(schema.V1), testFactory(schema.V0mimir1))
			assert.Equal(t, s.Type, m.Type)
		})
	})

	t.Run("single version", func(t *testing.T) {
		s := testSchema(schema.V1)
		require.NotPanics(t, func() {
			NewManifest(s, testFactory(schema.V1))
		})
	})

	t.Run("factory version not in schema", func(t *testing.T) {
		s := testSchema(schema.V1)
		assert.PanicsWithValue(t, "factory version v0mimir1 not found in schema for test", func() {
			NewManifest(s, testFactory(schema.V1), testFactory(schema.V0mimir1))
		})
	})

	t.Run("duplicate factory version", func(t *testing.T) {
		s := testSchema(schema.V1)
		assert.PanicsWithValue(t, "duplicate factory version v1 for test", func() {
			NewManifest(s, testFactory(schema.V1), testFactory(schema.V1))
		})
	})

	t.Run("schema version missing factory", func(t *testing.T) {
		s := testSchema(schema.V1, schema.V0mimir1)
		assert.PanicsWithValue(t, "schema version v0mimir1 has no factory for test", func() {
			NewManifest(s, testFactory(schema.V1))
		})
	})
}

func TestManifest_GetFactoryForVersion(t *testing.T) {
	s := testSchema(schema.V1, schema.V0mimir1)
	m := NewManifest(s, testFactory(schema.V0mimir1), testFactory(schema.V1))

	t.Run("found", func(t *testing.T) {
		f, ok := m.GetFactoryForVersion(schema.V1)
		require.True(t, ok)
		assert.Equal(t, schema.V1, f.Version())
	})

	t.Run("not found", func(t *testing.T) {
		_, ok := m.GetFactoryForVersion(schema.V0mimir2)
		assert.False(t, ok)
	})
}
