package notify

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_AllReceivers(t *testing.T) {
	input := &Route{
		Receiver: "foo",
		Routes: []*Route{
			{
				Receiver: "bar",
				Routes: []*Route{
					{
						Receiver: "bazz",
					},
				},
			},
			{
				Receiver: "buzz",
			},
		},
	}

	require.Equal(t, map[string]struct{}{
		"foo":  {},
		"bar":  {},
		"bazz": {},
		"buzz": {},
	}, AllReceivers(input))

	// test empty
	empty := make(map[string]struct{})
	emptyRoute := &Route{}
	require.Equal(t, empty, AllReceivers(emptyRoute))
}
