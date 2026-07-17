package utils

import (
	"time"
	_ "time/tzdata"
	"github.com/decko/wiki-go/internal/logger"
)

// FormatTimeInTimezone formats a time.Time value using the specified timezone
// If the timezone is invalid, it falls back to UTC
func FormatTimeInTimezone(t time.Time, timezone string, format string) string {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		logger.Warn("Error loading timezone %s: %v, falling back to UTC", timezone, err)
		loc = time.UTC
	}

	return t.In(loc).Format(format)
}
