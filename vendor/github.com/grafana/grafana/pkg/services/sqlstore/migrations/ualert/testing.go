package ualert

import (
	"testing"

	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/services/sqlstore/migrator"
)

// newTestMigration generates an empty migration to use in tests.
func newTestMigration(t *testing.T) *migration {
	t.Helper()

	return &migration{
		mg: &migrator.Migrator{

			Logger: log.New("test"),
		},
		seenChannelUIDs: make(map[string]struct{}),
	}
}
