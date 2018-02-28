package format

import (
	"time"
)

func Milliseconds(d time.Duration) float64 {
	return d.Seconds() * 1e3
}
