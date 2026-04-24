package receivers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/receivers/schema"
)

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

func testFactory(version schema.Version) IntegrationVersionFactory {
	return IntegrationVersionFactory{
		Version: version,
		Type:    "test",
	}
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
		assert.Equal(t, schema.V1, f.Version)
	})

	t.Run("not found", func(t *testing.T) {
		_, ok := m.GetFactoryForVersion(schema.V0mimir2)
		assert.False(t, ok)
	})
}
