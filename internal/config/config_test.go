package config

import (
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"
)

func mysqlParseDSN(dsn string) (*mysql.Config, error) {
	return mysql.ParseDSN(dsn)
}

func TestBuildMySQLDSN(t *testing.T) {
	dsn := buildMySQLDSN(DatabaseConfig{
		Host: "127.0.0.1", Port: 3306,
		User: "zpush", Password: "plain", Name: "zpush",
	})
	// FormatDSN produces "user:pass@tcp(host:port)/dbname?params"
	for _, want := range []string{"zpush:plain@tcp(127.0.0.1:3306)/zpush", "parseTime=true"} {
		if !strings.Contains(dsn, want) {
			t.Fatalf("dsn %q missing %q", dsn, want)
		}
	}
}

func TestBuildMySQLDSNEscapesSpecialPassword(t *testing.T) {
	dsn := buildMySQLDSN(DatabaseConfig{
		Host: "mysql", Port: 3306,
		User: "zpush", Password: "p@ss:word/1", Name: "zpush",
	})
	// The password must survive a round-trip parse
	parsed, err := mysqlParseDSN(dsn)
	if err != nil {
		t.Fatalf("parse dsn %q: %v", dsn, err)
	}
	if parsed.Passwd != "p@ss:word/1" {
		t.Fatalf("password round-trip = %q, want p@ss:word/1", parsed.Passwd)
	}
}

func TestBuildMySQLDSNIPv6Host(t *testing.T) {
	dsn := buildMySQLDSN(DatabaseConfig{
		Host: "::1", Port: 3306,
		User: "u", Password: "p", Name: "zpush",
	})
	if !strings.Contains(dsn, "[::1]:3306") {
		t.Fatalf("dsn %q missing bracketed IPv6 host", dsn)
	}
}
