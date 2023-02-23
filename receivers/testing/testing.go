package testing

import (
	"net/url"

	"github.com/grafana/alerting/receivers"
)

func ParseURLUnsafe(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

func DecryptForTesting(sjd map[string][]byte) receivers.DecryptFunc {
	return func(key string, fallback string) string {
		v, ok := sjd[key]
		if !ok {
			return fallback
		}
		return string(v)
	}
}
