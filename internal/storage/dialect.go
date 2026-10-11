package storage

import (
	"fmt"
	"time"
)

// Dialect represents the SQL dialect in use.
type Dialect string

const (
	DialectSQLite Dialect = "sqlite"
	DialectMySQL  Dialect = "mysql"
)

// CurrentDialect is set by Open() and used by all repositories.
var CurrentDialect Dialect = DialectSQLite

// NowExpr returns the SQL expression for the current UTC time.
// SQLite: strftime('%Y-%m-%dT%H:%M:%SZ','now')
// MySQL:  NOW()
func (d Dialect) NowExpr() string {
	if d == DialectMySQL {
		return "NOW()"
	}
	return "strftime('%Y-%m-%dT%H:%M:%SZ','now')"
}

// NowCompare returns the SQL expression for current UTC time in WHERE comparisons.
// Some SQLite calls use a slightly different format with a space.
func (d Dialect) NowCompare() string {
	if d == DialectMySQL {
		return "NOW()"
	}
	return "strftime('%Y-%m-%dT%H:%M:%SZ', 'now')"
}

// NowExpr returns the global dialect's current time expression.
func NowExpr() string {
	return CurrentDialect.NowExpr()
}

// NowCompare returns the global dialect's current time expression for comparisons.
func NowCompare() string {
	return CurrentDialect.NowCompare()
}

// mysqlDatetimeLayout is the time format accepted by MySQL DATETIME columns.
// MySQL supports up to 6 fractional second digits (microseconds).
const mysqlDatetimeLayout = "2006-01-02 15:04:05.999999"

// FormatTime formats a time.Time for the current dialect.
// SQLite: RFC3339Nano (e.g. "2024-01-15T10:30:00.123456789Z")
// MySQL:  MySQL DATETIME format  (e.g. "2024-01-15 10:30:00.123456")
func FormatTime(t time.Time) string {
	if CurrentDialect == DialectMySQL {
		return t.UTC().Format(mysqlDatetimeLayout)
	}
	return t.UTC().Format(time.RFC3339Nano)
}

// FormatNow returns the current UTC time formatted for the current dialect.
func FormatNow() string {
	return FormatTime(time.Now().UTC())
}

// ParseTime parses a time string from the database.
// It tries RFC3339Nano first (SQLite), then MySQL DATETIME formats.
func ParseTime(s string) (time.Time, error) {
	// Try RFC3339Nano (SQLite format)
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
