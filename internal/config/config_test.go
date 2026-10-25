package config

import (
	"net/url"
	"strings"
	"testing"
)

func postgresParseDSN(dsn string) (*url.URL, error) {
	return url.Parse(dsn)
}

func TestBuildPostgresDSN(t *testing.T) {
	dsn := buildPostgresDSN(DatabaseConfig{
		Host: "127.0.0.1", Port: 5432,
		User: "notygo", Password: "plain", Name: "notygo", SSLMode: "disable",
	})
	// FormatDSN produces "postgres://user:pass@host:port/dbname?params"
	for _, want := range []string{"postgres://notygo:plain@127.0.0.1:5432/notygo", "sslmode=disable"} {
		if !strings.Contains(dsn, want) {
			t.Fatalf("dsn %q missing %q", dsn, want)
		}
	}
}

func TestBuildPostgresDSNEscapesSpecialPassword(t *testing.T) {
	dsn := buildPostgresDSN(DatabaseConfig{
		Host: "postgres", Port: 5432,
		User: "notygo", Password: "p@ss:word/1", Name: "notygo", SSLMode: "disable",
	})
	// The password must survive a round-trip parse
	parsed, err := postgresParseDSN(dsn)
	if err != nil {
		t.Fatalf("parse dsn %q: %v", dsn, err)
	}
	password, _ := parsed.User.Password()
	if password != "p@ss:word/1" {
		t.Fatalf("password round-trip = %q, want p@ss:word/1", password)
	}
}

func TestBuildPostgresDSNIPv6Host(t *testing.T) {
	dsn := buildPostgresDSN(DatabaseConfig{
		Host: "::1", Port: 5432,
		User: "u", Password: "p", Name: "notygo", SSLMode: "disable",
	})
	if !strings.Contains(dsn, "[::1]:5432") {
		t.Fatalf("dsn %q missing bracketed IPv6 host", dsn)
	}
}
