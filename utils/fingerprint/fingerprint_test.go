package fingerprint

import (
	"encoding/json"
	"maps"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMap(t *testing.T) {
	m := map[string]string{
		"a": "b",
		"c": "d",
		"e": "f",
		"g": "h",
		"i": "j",
		"k": "l",
		"m": "n",
		"o": "p",
		"q": "r",
		"s": "t",
		"u": "v",
		"w": "x",
		"y": "z",
	}
	h := NewHash()
	expected := Map(m, h.String, h.String)

	require.NotEmpty(t, expected)
	b, err := json.Marshal(m)
	require.NoError(t, err)

	for i := 0; i < 100000; i++ {
		var m2 map[string]string
		require.NoError(t, json.Unmarshal(b, &m2))
		curFP := Map(m2, h.String, h.String)
		require.Equal(t, expected, curFP)
	}

	keys := slices.Collect(maps.Keys(m))
	expected = SliceUnordered(keys, h.String)
	for i := 0; i < 100; i++ {
		rand.Shuffle(len(keys), func(i, j int) {
			keys[i], keys[j] = keys[j], keys[i]
		})
		actual := SliceUnordered(keys, h.String)
		require.Equal(t, expected, actual)
	}

	pp1 := Map(map[string]string{
		"a": "b",
		"c": "d",
	}, h.String, h.String)

	pp2 := Map(map[string]string{
		"a": "d",
		"c": "b",
	}, h.String, h.String)

	require.NotEqual(t, pp1, pp2)
}
