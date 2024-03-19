package notify

import (
	"bytes"
	"testing"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"

	amv2 "github.com/prometheus/alertmanager/api/v2/models"
	"github.com/prometheus/alertmanager/silence/silencepb"
)

func TestSilenceState(t *testing.T) {
	now := time.Now()
	createSilence := func(name string, val string) PostableSilence {
		return PostableSilence{
			Silence: amv2.Silence{
				Comment:   ptr("This is a comment"),
				CreatedBy: ptr("test"),
				EndsAt:    ptr(strfmt.DateTime(now.Add(time.Minute))),
				Matchers: amv2.Matchers{{
					IsEqual: ptr(true),
					IsRegex: ptr(false),
					Name:    ptr(name),
					Value:   ptr(val),
				}},
				StartsAt: ptr(strfmt.DateTime(now)),
			},
		}
	}

	createExpectedSilenceMesh := func(name string, val string) *silencepb.MeshSilence {
		return &silencepb.MeshSilence{
			Silence: &silencepb.Silence{
				Comment:   "This is a comment",
				CreatedBy: "test",
				EndsAt:    now.Add(time.Minute).UTC(),
				Matchers:  []*silencepb.Matcher{{Type: silencepb.Matcher_EQUAL, Name: name, Pattern: val}},
				StartsAt:  now.UTC(),
				UpdatedAt: now.UTC(),
			},
			ExpiresAt: now.Add(time.Minute),
		}
	}

	cmpOpts := cmp.Options{
		cmpopts.IgnoreFields(silencepb.Silence{}, "Id", "UpdatedAt"),
		cmpopts.EquateApproxTime(time.Second),
	}

	cases := []struct {
		name     string
		silences []PostableSilence
		expired  bool
	}{
		{
			name: "Single silence",
			silences: []PostableSilence{
				createSilence("foo", "bar"),
			},
		},
		{
			name: "Multiple silences",
			silences: []PostableSilence{
				createSilence("foo", "bar"),
				createSilence("_foo1", "bar"),
			},
		},
		{
			name: "Unicode and edge case silences",
			silences: []PostableSilence{
				createSilence("0foo", "bar"),
				createSilence("foo", "🙂bar"),
				createSilence("foo🙂", "bar"),
			},
		},
		{
			name: "Expired silence",
			silences: []PostableSilence{
				createSilence("foo", "bar"),
			},
			expired: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			am, _ := setupAMTest(t)
			expectedState := SilenceState{}
			for _, silence := range c.silences {
				sid, err := am.CreateSilence(&silence)
				require.NoError(t, err)

				expected := createExpectedSilenceMesh(*silence.Matchers[0].Name, *silence.Matchers[0].Value)
				if c.expired {
					require.NoError(t, am.DeleteSilence(sid))
					sil, err := am.GetSilence(sid)
					require.NoError(t, err)
					expected.Silence.EndsAt = time.Time(*sil.EndsAt).UTC()
					expected.ExpiresAt = time.Time(*sil.EndsAt).Add(30 * time.Millisecond).UTC()
				}
				expectedState[sid] = expected
			}
			state, err := am.SilenceState()
			require.NoError(t, err)
			if !cmp.Equal(state, expectedState, cmpOpts...) {
				t.Errorf("Unexpected Diff: %v", cmp.Diff(state, expectedState, cmpOpts...))
			}

			b, err := state.MarshalBinary()
			require.NoError(t, err)

			decoded, err := DecodeState(bytes.NewReader(b))
			require.NoError(t, err)

			if !cmp.Equal(decoded, expectedState, cmpOpts...) {
				t.Errorf("Unexpected Diff: %v", cmp.Diff(state, expectedState, cmpOpts...))
			}
		})
	}
}
