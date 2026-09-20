package httpx

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ParseRetryAfter parses an HTTP Retry-After value. The boolean distinguishes
// an absent or invalid value from a valid zero delay.
func ParseRetryAfter(value string, now time.Time) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	if seconds, err := strconv.ParseUint(value, 10, 63); err == nil {
		const maxSeconds = uint64((1<<63 - 1) / int64(time.Second))
		if seconds > maxSeconds {
			return 0, false
		}
		return time.Duration(seconds) * time.Second, true
	}
	when, err := http.ParseTime(value)
	if err != nil {
		return 0, false
	}
	delay := when.Sub(now)
	if delay < 0 {
		delay = 0
	}
	return delay, true
}
