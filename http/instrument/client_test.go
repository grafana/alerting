package instrument

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/http/instrument/instrumenttest"
)

func TestTimedClient_operationName(t *testing.T) {
	r, err := http.NewRequest("GET", "https://weave.test", nil)
	assert.NoError(t, err)

	r = r.WithContext(context.WithValue(context.Background(), OperationNameContextKey, "opp"))
	c := NewTimedClient(http.DefaultClient, nil)

	assert.Equal(t, "opp", c.operationName(r))
}

func TestTimedClient_operationName_Default(t *testing.T) {
	r, err := http.NewRequest("GET", "https://weave.test/you/know/me", nil)
	assert.NoError(t, err)

	r = r.WithContext(context.Background())
	c := NewTimedClient(http.DefaultClient, nil)

	assert.Equal(t, "/you/know/me", c.operationName(r))
}

type statusCollector struct {
	statusCodes []string
}

func (c *statusCollector) Register()                                 {}
func (c *statusCollector) Before(context.Context, string, time.Time) {}
func (c *statusCollector) After(_ context.Context, _, code string, _ time.Time) {
	c.statusCodes = append(c.statusCodes, code)
}

type errRequester struct{ err error }

func (e errRequester) Do(*http.Request) (*http.Response, error) { return nil, e.err }

func TestTimeRequest_StatusCode(t *testing.T) {
	t.Run("success records the numeric status code", func(t *testing.T) {
		coll := &statusCollector{}
		req, err := http.NewRequest(http.MethodGet, "https://weave.test", nil)
		require.NoError(t, err)

		resp, err := TimeRequest(context.Background(), "op", coll, instrumenttest.NewFakeRequester(), req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.NoError(t, resp.Body.Close())
		assert.Equal(t, []string{"200"}, coll.statusCodes)
	})

	t.Run("canceled context records canceled", func(t *testing.T) {
		coll := &statusCollector{}
		req, err := http.NewRequest(http.MethodGet, "https://weave.test", nil)
		require.NoError(t, err)

		_, err = TimeRequest(context.Background(), "op", coll, errRequester{err: context.Canceled}, req) // nolint:bodyclose
		require.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, []string{StatusCodeCanceled}, coll.statusCodes)
	})

	t.Run("wrapped canceled error records canceled", func(t *testing.T) {
		coll := &statusCollector{}
		req, err := http.NewRequest(http.MethodGet, "https://weave.test", nil)
		require.NoError(t, err)

		wrapped := errors.Join(errors.New("Post failed"), context.Canceled)
		_, err = TimeRequest(context.Background(), "op", coll, errRequester{err: wrapped}, req) // nolint:bodyclose
		require.Error(t, err)
		assert.Equal(t, []string{StatusCodeCanceled}, coll.statusCodes)
	})

	t.Run("real canceled request via http client records canceled", func(t *testing.T) {
		coll := &statusCollector{}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://weave.test", nil)
		require.NoError(t, err)

		_, err = TimeRequest(ctx, "op", coll, http.DefaultClient, req) // nolint:bodyclose
		require.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, []string{StatusCodeCanceled}, coll.statusCodes)
	})

	t.Run("other error records error", func(t *testing.T) {
		coll := &statusCollector{}
		req, err := http.NewRequest(http.MethodGet, "https://weave.test", nil)
		require.NoError(t, err)

		_, err = TimeRequest(context.Background(), "op", coll, errRequester{err: errors.New("connection refused")}, req) // nolint:bodyclose
		require.Error(t, err)
		assert.Equal(t, []string{StatusCodeError}, coll.statusCodes)
	})

	t.Run("deadline exceeded records error", func(t *testing.T) {
		coll := &statusCollector{}
		req, err := http.NewRequest(http.MethodGet, "https://weave.test", nil)
		require.NoError(t, err)

		_, err = TimeRequest(context.Background(), "op", coll, errRequester{err: context.DeadlineExceeded}, req) // nolint:bodyclose
		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Equal(t, []string{StatusCodeError}, coll.statusCodes)
	})
}
