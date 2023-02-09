package receivers

import (
	"strconv"
	"strings"
)

type OptionalNumber string

func (o OptionalNumber) String() string {
	return string(o)
}

// Int64 returns the number as an int64.
func (o OptionalNumber) Int64() (int64, error) {
	if string(o) == "" {
		return 0, nil
	}
	return strconv.ParseInt(string(o), 10, 64)
}

func (o *OptionalNumber) UnmarshalJSON(bytes []byte) error {
	str := string(bytes)
	*o = OptionalNumber(strings.Trim(str, "\""))
	return nil
}
