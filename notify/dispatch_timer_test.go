package notify

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_DispatchTimer_String(t *testing.T) {
	tt := []struct {
		name string
		dt   DispatchTimer
		exp  string
	}{
		{
			name: "when default",
			dt:   DispatchTimerDefault,
			exp:  "default",
		},
		{
			name: "when sync",
			dt:   DispatchTimerSync,
			exp:  "sync",
		},
		{
			name: "when unknown",
			dt:   DispatchTimer(999),
			exp:  "default",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.exp, tc.dt.String())
		})
	}
}

func Test_DispatchTimer_FromString(t *testing.T) {
	tt := []struct {
		name string
		s    string
		exp  DispatchTimer
	}{
		{
			name: "when default",
			s:    "default",
			exp:  DispatchTimerDefault,
		},
		{
			name: "when sync",
			s:    "sync",
			exp:  DispatchTimerSync,
		},
		{
			name: "when unknown",
			s:    "default",
			exp:  DispatchTimerDefault,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			dt := new(DispatchTimer)
			dt.FromString(tc.s)

			assert.Equal(t, tc.exp, *dt)
		})
	}
}
