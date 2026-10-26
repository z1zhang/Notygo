package endpoint

import "testing"

func TestAuthTypeCodeRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		code int16
	}{
		{"none", 0},
		{"bearer", 1},
		{"hmac", 2},
	}
	for _, c := range cases {
		code, err := authTypeToCode(c.name)
		if err != nil {
			t.Fatalf("authTypeToCode(%q): %v", c.name, err)
		}
		if code != c.code {
			t.Fatalf("authTypeToCode(%q) = %d, want %d", c.name, code, c.code)
		}
		back, err := authTypeFromCode(c.code)
		if err != nil {
			t.Fatalf("authTypeFromCode(%d): %v", c.code, err)
		}
		if back != c.name {
			t.Fatalf("authTypeFromCode(%d) = %q, want %q", c.code, back, c.name)
		}
	}
}

func TestAuthTypeUnknownValues(t *testing.T) {
	if _, err := authTypeToCode("basic"); err == nil {
		t.Fatal("authTypeToCode('basic') should error")
	}
	if _, err := authTypeFromCode(3); err == nil {
		t.Fatal("authTypeFromCode(3) should error")
	}
	if _, err := authTypeFromCode(-1); err == nil {
		t.Fatal("authTypeFromCode(-1) should error")
	}
}

func TestIsValidAuthType(t *testing.T) {
	for _, ok := range []string{"none", "bearer", "hmac"} {
		if !IsValidAuthType(ok) {
			t.Fatalf("IsValidAuthType(%q) = false, want true", ok)
		}
	}
	for _, bad := range []string{"", "basic", "oauth", "HMAC", "bearer "} {
		if IsValidAuthType(bad) {
			t.Fatalf("IsValidAuthType(%q) = true, want false", bad)
		}
	}
}
