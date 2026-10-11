package storage

import (
	"fmt"
	"time"
)

// mysqlDatetimeLayout is the time format accepted by MySQL DATETIME columns.
// MySQL supports up to 6 fractional second digits (microseconds).
const mysqlDatetimeLayout = "2006-01-02 15:04:05.999999"

// NowExpr returns the SQL expression for the current UTC time ("NOW()").
func NowExpr() string {
	return "NOW()"
}

// FormatTime formats a time.Time for MySQL DATETIME columns.
// Example: "2024-01-15 10:30:00.123456"
func FormatTime(t time.Time) string {
	return t.UTC().Format(mysqlDatetimeLayout)
}

// FormatNow returns the current UTC time formatted for MySQL.
func FormatNow() string {
	return FormatTime(time.Now().UTC())
}

// ParseTime parses a time string from the database.
// It tries RFC3339Nano first (the format MySQL driver returns by default),
// then MySQL DATETIME with fractional seconds, then without.
func ParseTime(s string) (time.Time, error) {
	// Try RFC3339Nano (go-sql-driver/mysql default output format)
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}
	// Try MySQL DATETIME with fractional seconds
	if t, err := time.Parse(mysqlDatetimeLayout, s); err == nil {
		return t, nil
	}
	// Try MySQL DATETIME without fractional seconds
	if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("cannot parse time: %s", s)
}
