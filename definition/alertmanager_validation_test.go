package definition

import (
	"testing"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

func TestValidateRoutes(t *testing.T) {
	zero := model.Duration(0)

	type testCase struct {
		desc   string
		route  Route
		expMsg string
	}

	t.Run("valid route", func(t *testing.T) {
		cases := []testCase{
			{
				desc:  "empty",
				route: Route{},
			},
			{
				desc: "simple",
				route: Route{
					Receiver:   "foo",
					GroupByStr: []string{"..."},
				},
			},
			{
				desc: "nested",
				route: Route{
					Receiver:   "foo",
					GroupByStr: []string{"..."},
					Routes: []*Route{
						{
							Receiver: "bar",
						},
					},
				},
			},
		}

		for _, c := range cases {
			t.Run(c.desc, func(t *testing.T) {
				err := c.route.ValidateChild()

				require.NoError(t, err)
			})
		}
	})

	t.Run("invalid route", func(t *testing.T) {
		cases := []testCase{
			{
				desc: "zero group interval",
				route: Route{
					Receiver:      "foo",
					GroupByStr:    []string{"..."},
					GroupInterval: &zero,
				},
				expMsg: "group_interval cannot be zero",
			},
			{
				desc: "zero repeat interval",
				route: Route{
					Receiver:       "foo",
					GroupByStr:     []string{"..."},
					RepeatInterval: &zero,
				},
				expMsg: "repeat_interval cannot be zero",
			},
			{
				desc: "duplicated label",
				route: Route{
					Receiver: "foo",
					GroupByStr: []string{
						"abc",
						"abc",
					},
				},
				expMsg: "duplicated label",
			},
			{
				desc: "wildcard and non-wildcard label simultaneously",
				route: Route{
					Receiver: "foo",
					GroupByStr: []string{
						"...",
						"abc",
					},
				},
				expMsg: "cannot have wildcard",
			},
			{
				desc: "valid with nested invalid",
				route: Route{
					Receiver:   "foo",
					GroupByStr: []string{"..."},
					Routes: []*Route{
						{
							GroupByStr: []string{
								"abc",
								"abc",
							},
						},
					},
				},
				expMsg: "duplicated label",
			},
		}

		for _, c := range cases {
			t.Run(c.desc, func(t *testing.T) {
				err := c.route.ValidateChild()

				require.Error(t, err)
				require.Contains(t, err.Error(), c.expMsg)
			})
		}
	})

	t.Run("route validator normalizes group_by", func(t *testing.T) {
		t.Run("when grouping normally", func(t *testing.T) {
			route := Route{
				Receiver:   "foo",
				GroupByStr: []string{"abc", "def"},
			}

			_ = route.ValidateChild()

			require.False(t, route.GroupByAll)
			require.Equal(t, []model.LabelName{"abc", "def"}, route.GroupBy)
		})

		t.Run("when grouping by wildcard, nil", func(t *testing.T) {
			route := Route{
				Receiver:   "foo",
				GroupByStr: []string{"..."},
			}

			_ = route.ValidateChild()

			require.True(t, route.GroupByAll)
			require.Nil(t, route.GroupBy)
		})

		t.Run("idempotently", func(t *testing.T) {
			route := Route{
				Receiver:   "foo",
				GroupByStr: []string{"abc", "def"},
			}

			err := route.ValidateChild()
			require.NoError(t, err)
			err = route.ValidateChild()
			require.NoError(t, err)

			require.False(t, route.GroupByAll)
			require.Equal(t, []model.LabelName{"abc", "def"}, route.GroupBy)
		})
	})

	t.Run("valid root route", func(t *testing.T) {
		cases := []testCase{
			{
				desc: "simple",
				route: Route{
					Receiver:   "foo",
					GroupByStr: []string{"..."},
				},
			},
		}

		for _, c := range cases {
			t.Run(c.desc, func(t *testing.T) {
				err := c.route.Validate()

				require.NoError(t, err)
			})
		}
	})

	t.Run("invalid root route", func(t *testing.T) {
		cases := []testCase{
			{
				desc: "no receiver",
				route: Route{
					GroupByStr: []string{"..."},
				},
				expMsg: "must specify a default receiver",
			},
			{
				desc: "exact matchers present",
				route: Route{
					Receiver:   "foo",
					GroupByStr: []string{"..."},
					Match: map[string]string{
						"abc": "def",
					},
				},
				expMsg: "must not have any matchers",
			},
			{
				desc: "regex matchers present",
				route: Route{
					Receiver:   "foo",
					GroupByStr: []string{"..."},
					Match: map[string]string{
						"abc": "def",
					},
				},
				expMsg: "must not have any matchers",
			},
			{
				desc: "mute time intervals present",
				route: Route{
					Receiver:          "foo",
					GroupByStr:        []string{"..."},
					MuteTimeIntervals: []string{"10"},
				},
				expMsg: "must not have any mute time intervals",
			},
			{
				desc: "validation error that is not specific to root",
				route: Route{
					Receiver:   "foo",
					GroupByStr: []string{"abc", "abc"},
				},
				expMsg: "duplicated label",
			},
			{
				desc: "nested validation error that is not specific to root",
				route: Route{
					Receiver: "foo",
					Routes: []*Route{
						{
							GroupByStr: []string{"abc", "abc"},
						},
					},
				},
				expMsg: "duplicated label",
			},
		}

		for _, c := range cases {
			t.Run(c.desc, func(t *testing.T) {
				err := c.route.Validate()

				require.Error(t, err)
				require.Contains(t, err.Error(), c.expMsg)
			})
		}
	})
}
