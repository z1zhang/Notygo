package storage

// NowExpr returns the SQL expression for the current time.
// PostgreSQL timestamptz values are absolute points in time, so server or
// session timezone settings cannot skew comparisons with Go-side UTC
// timestamps (unlike MySQL DATETIME, where NOW() returning local time would).
func NowExpr() string {
	return "now()"
}
