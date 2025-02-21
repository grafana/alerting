package gomplate

import (
	"time"
)

func CreateTimeFuncs() Namespace {
	return Namespace{"time", &TimeFuncs{}}
}

// Time Functions.
type TimeFuncs struct {
}

func (TimeFuncs) Now() time.Time {
	return time.Now()
}
