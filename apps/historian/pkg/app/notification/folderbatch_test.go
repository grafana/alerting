package notification

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/grafana/grafana-app-sdk/logging"

	"github.com/grafana/alerting/notify/historian"
	"github.com/grafana/alerting/notify/historian/lokiclient"
)

func TestSplitFolderKeys(t *testing.T) {
	spec := notificationFolderSpec

	t.Run("no keys yields a single empty batch", func(t *testing.T) {
		batches, err := splitFolderKeys(nil, spec, 0, 65536)
		require.NoError(t, err)
		require.Equal(t, [][]string{nil}, batches)
	})

	t.Run("max query size <= 0 disables batching", func(t *testing.T) {
		keys := []string{"a", "b", "c"}
		batches, err := splitFolderKeys(keys, spec, 10, 0)
		require.NoError(t, err)
		require.Equal(t, [][]string{keys}, batches)
	})

	t.Run("everything fits in one batch", func(t *testing.T) {
		keys := []string{"folderA", "folderB", "folderC"}
		// A generous limit keeps all keys in one batch.
		batches, err := splitFolderKeys(keys, spec, 100, 65536)
		require.NoError(t, err)
		require.Len(t, batches, 1)
		assert.Equal(t, keys, batches[0])
	})

	t.Run("splits into multiple batches and preserves every key", func(t *testing.T) {
		// Ten 100-byte folders with a tight budget forces several batches.
		var keys []string
		for i := 0; i < 10; i++ {
			keys = append(keys, fmt.Sprintf("%02d-%s", i, strings.Repeat("x", 100)))
		}
		maxQuerySize := folderBatchReservedBytes + spec.fixedLen() + 350

		batches, err := splitFolderKeys(keys, spec, 0, maxQuerySize)
		require.NoError(t, err)
		require.Greater(t, len(batches), 1, "expected the folder set to be split")

		// Every key appears exactly once, order preserved.
		var got []string
		for _, b := range batches {
			got = append(got, b...)
		}
		assert.Equal(t, keys, got)

		// Every batch, once rendered, stays within the budget.
		budget := maxQuerySize - folderBatchReservedBytes
		for _, b := range batches {
			assert.LessOrEqual(t, len(spec.render(escapeFolderKeys(b))), budget)
		}
	})

	t.Run("a single folder that cannot fit is a hard error", func(t *testing.T) {
		keys := []string{strings.Repeat("x", 5000)}
		_, err := splitFolderKeys(keys, spec, 0, 2048)
		require.ErrorIs(t, err, ErrInvalidQuery)
	})
}

func TestFolderFilterSpec_Render(t *testing.T) {
	assert.Equal(t,
		` | folder_uids =~ "(^|.*,)(folderA|folderB)($|,.*)"`,
		notificationFolderSpec.render([]string{"folderA", "folderB"}))
	assert.Equal(t,
		` | folder_uid =~ "^(folderA|folderB)$"`,
		alertFolderSpec.render([]string{"folderA", "folderB"}))
}

func TestBuildNotificationBatches(t *testing.T) {
	t.Run("nil filter produces a single unfiltered query", func(t *testing.T) {
		reader := &LokiReader{logger: &logging.NoOpLogger{}, maxQuerySize: 65536}
		logqls, err := reader.buildNotificationBatches(Query{}, nil, nil)
		require.NoError(t, err)
		require.Len(t, logqls, 1)
		assert.NotContains(t, logqls[0], "folder_uids")
	})

	t.Run("large folder set is split into several bounded queries", func(t *testing.T) {
		// Long folder UIDs so only a couple fit per query under a tight limit.
		var folders []string
		for i := 0; i < 6; i++ {
			folders = append(folders, fmt.Sprintf("f%02d-%s", i, strings.Repeat("y", 200)))
		}
		filter := testFilter([]string{"ruleA"}, folders)

		maxQuerySize := folderBatchReservedBytes + notificationFolderSpec.fixedLen() + 600
		reader := &LokiReader{logger: &logging.NoOpLogger{}, maxQuerySize: maxQuerySize}

		logqls, err := reader.buildNotificationBatches(Query{}, nil, filter)
		require.NoError(t, err)
		require.Greater(t, len(logqls), 1, "expected multiple batched queries")

		for _, logql := range logqls {
			assert.LessOrEqual(t, len(logql), maxQuerySize, "each batched query must stay within the limit")
			assert.Contains(t, logql, "folder_uids =~")
		}
	})
}

// TestLokiReader_Query_BatchesAndDeduplicates drives a query whose accessible
// folder set is too large for a single query. The reader must split it into
// several queries and, because a notification referencing folders spread across
// batches is returned by more than one batch query, deduplicate the results.
func TestLokiReader_Query_BatchesAndDeduplicates(t *testing.T) {
	now := time.Now().UTC()

	// Two long folder UIDs so each lands in its own batch under the limit below.
	folderA := strings.Repeat("a", 2000)
	folderB := strings.Repeat("b", 2000)

	// One notification referencing both folders and one accessible rule. Both
	// batch queries match it (each batch pushes down one of its folders).
	entryJSON := createLokiEntryJSON(nil, historian.NotificationHistoryLokiEntry{
		SchemaVersion: 2,
		UUID:          "uuid-mixed",
		Receiver:      "Shared",
		Status:        "firing",
		RuleUIDs:      []string{"ruleA"},
		FolderUIDs:    []string{folderA, folderB},
		AlertCount:    1,
		PipelineTime:  now,
	})
	resp := lokiclient.QueryRes{
		Data: lokiclient.QueryData{
			Result: []lokiclient.Stream{
				{Values: []lokiclient.Sample{{T: now, V: entryJSON}}},
			},
		},
	}

	mockClient := &mockLokiClient{}
	mockClient.On("RangeQuery", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(resp, nil)

	// Budget leaves room for exactly one 2000-byte folder per query.
	reader := &LokiReader{
		client:       mockClient,
		logger:       &logging.NoOpLogger{},
		maxQuerySize: folderBatchReservedBytes + 3000,
	}

	filter := testFilter([]string{"ruleA"}, []string{folderA, folderB})
	result, err := reader.Query(context.Background(), Query{}, filter)
	require.NoError(t, err)

	// Two batches → two Loki queries.
	var rangeCalls int
	for _, c := range mockClient.Calls {
		if c.Method == "RangeQuery" {
			rangeCalls++
		}
	}
	assert.Equal(t, 2, rangeCalls, "expected one Loki query per folder batch")

	// The notification, matched by both batches, must be returned once.
	require.Len(t, result.Entries, 1)
	assert.Equal(t, "uuid-mixed", result.Entries[0].Uuid)
}

func TestMergeCounts(t *testing.T) {
	t.Run("sums identical label sets across batches and applies the limit", func(t *testing.T) {
		counts := []Count{
			{Receiver: stringPtr("A"), Count: 3},
			{Receiver: stringPtr("A"), Count: 4}, // same label set, different batch
			{Receiver: stringPtr("B"), Count: 5},
			{Receiver: stringPtr("C"), Count: 1},
		}

		got := mergeCounts(counts, 2)
		require.Len(t, got, 2, "limit applied after merge")

		// A merges to 3+4=7 (highest), B stays 5; C (1) is dropped by the limit.
		assert.Equal(t, "A", *got[0].Receiver)
		assert.Equal(t, int64(7), got[0].Count)
		assert.Equal(t, "B", *got[1].Receiver)
		assert.Equal(t, int64(5), got[1].Count)
	})
}

func TestMergeRangeCounts(t *testing.T) {
	rv := func(ts, count int64) RangeValue { return RangeValue{Timestamp: ts, Count: count} }

	counts := []Count{
		{Receiver: stringPtr("A"), Values: []RangeValue{rv(1, 2), rv(2, 3)}},
		{Receiver: stringPtr("A"), Values: []RangeValue{rv(2, 1), rv(3, 4)}}, // same label set
		{Receiver: stringPtr("B"), Values: []RangeValue{rv(1, 9)}},
	}

	got := mergeRangeCounts(counts)
	require.Len(t, got, 2)

	byReceiver := map[string][]RangeValue{}
	for _, c := range got {
		byReceiver[*c.Receiver] = c.Values
	}

	assert.Equal(t, []RangeValue{rv(1, 2), rv(2, 4), rv(3, 4)}, byReceiver["A"])
	assert.Equal(t, []RangeValue{rv(1, 9)}, byReceiver["B"])
}
