package hash

import (
	"hash/fnv"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type test struct {
	Str     string
	private int
}

type complex struct {
	Test    *test
	TestMap *map[test]any
}

func TestDeepHashObject_Map(t *testing.T) {
	u, err := url.Parse("http://localhost:3000")
	require.NoError(t, err)
	u2, err := url.Parse("http://admin:admin@localhost:3000")
	require.NoError(t, err)

	testCases := [][]any{
		{
			map[string]string{"a": "b", "c": "d", "e": "f"},
			map[string]string{"a": "b", "c": "d"},
		},
		{
			map[int]int{1: 2},
			map[int]int{2: 1},
		},
		{
			map[test]string{{"test", 1}: "b", {"test", 2}: "d"},
			map[test]string{{"test", 2}: "b", {"test", 3}: "d"},
		},
		{
			[]string{"a", "b", "c"},
			[]string{"a", "b", "c", "d"},
		},
		{
			&[]bool{true, false},
			&[]bool{false, true},
		},
		{
			[]*complex{
				nil,
				{Test: &test{"test", 1}},
			},
			[]*complex{
				{Test: &test{"test", 1}},
			},
		},
		{
			[]*complex{
				nil,
				{Test: &test{"test", 1}},
			},
			[]*complex{
				{TestMap: &map[test]any{
					{"test", 1}: &test{"test", 1},
					{"test", 2}: float64(1),
				}},
			},
		},
		{
			nil,
			[]any{},
		},
		{
			map[any]any{
				"a": "b",
				&test{"test", 1}: complex{
					Test: &test{"test", 1},
					TestMap: &map[test]any{
						{"test", 1}: &test{"test", 1},
						{"test", 2}: 1,
					},
				},
				1:          1,
				true:       true,
				time.Now(): time.Now(),
			},
			nil,
		},
		{
			1,
			2,
		},
		{
			"test",
			"test ",
		},
		{
			true,
			false,
		},
		{
			u,
			u2,
		},
		{
			time.Now(),
			time.Now().UTC(),
		},
		{
			complex{
				Test: &test{"test", 1},
				TestMap: &map[test]any{
					{"test", 1}: &test{"test", 1},
					{"test", 2}: 1,
				},
			},
			complex{
				Test: &test{"test", 1},
				TestMap: &map[test]any{
					{"test", 1}: &test{"test", 1},
					{"test", 2}: float64(1),
				},
			},
		},
	}

	for idx, tc := range testCases {
		h := fnv.New64a()
		DeepHashObject(h, tc[0])
		hash11 := h.Sum64()
		DeepHashObject(h, tc[0])
		hash12 := h.Sum64()

		if hash12 != hash11 {
			t.Log(configForHash.Sprintf("%#v", tc[0]))
			require.Failf(t, "DeepHashObject returned different result for the same object.", "test case %d", idx)
		}

		DeepHashObject(h, tc[1])
		hash21 := h.Sum64()
		if hash21 == hash11 {
			t.Log("Object 1")
			t.Log(configForHash.Sprintf("%#v", tc[0]))
			t.Log("Object 2")
			t.Log(configForHash.Sprintf("%#v", tc[1]))
			require.Failf(t, "DeepHashObject returned same result for different objects.", "test case %d", idx)
		}
	}
}
