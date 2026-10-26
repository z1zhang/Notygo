package ingest

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"notygo/internal/endpoint"
	"notygo/internal/secret"
)

// signHex returns the hex HMAC-SHA256 of body keyed by key.
func signHex(key string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// newTestIngester builds an Ingester whose only usable dependency is the
// secret service — enough for validateHMACAuth unit tests.
func newTestIngester(t *testing.T) (*Ingester, string) {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("random key: %v", err)
	}
	secretSvc, err := secret.NewService("v1:" + base64.StdEncoding.EncodeToString(key))
	if err != nil {
		t.Fatalf("secret service: %v", err)
	}
	return NewIngester(nil, nil, secretSvc, nil, 0, nil), "unit-test-secret"
}

// hmacEndpoint builds an endpoint whose encrypted_secret is the encryption
// of secretValue.
func hmacEndpoint(t *testing.T, h *Ingester, secretValue string) *endpoint.Endpoint {
	t.Helper()
	encrypted, err := h.secretSvc.Encrypt(secretValue)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	return &endpoint.Endpoint{AuthType: "hmac", EncryptedSecret: &encrypted}
}

func TestValidateHMACAuth(t *testing.T) {
	body := []byte(`{"event":"deploy"}`)
	good := signHex("unit-test-secret", body)

	tests := []struct {
		name    string
		header  string
		body    []byte
		wantErr bool
	}{
		{"valid signature", good, body, false},
		{"sha256= prefix (GitHub style)", "sha256=" + good, body, false},
		{"tampered body", good, []byte(`{"event":"deploy!"}`), true},
		{"wrong key signature", signHex("other-secret", body), body, true},
		{"missing header", "", body, true},
		{"non-hex signature", "zzzz-not-hex", body, true},
		{"empty body with matching signature", signHex("unit-test-secret", nil), nil, false},
	}

	h, _ := newTestIngester(t)
	ep := hmacEndpoint(t, h, "unit-test-secret")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/endpoint/slug", nil)
			if tt.header != "" {
				r.Header.Set("X-Signature", tt.header)
			}
			err := h.validateHMACAuth(t.Context(), ep, r, tt.body)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateHMACAuth err = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateHMACAuthNoSecretConfigured(t *testing.T) {
	h, _ := newTestIngester(t)
	ep := &endpoint.Endpoint{AuthType: "hmac", EncryptedSecret: nil}
	r := httptest.NewRequest(http.MethodPost, "/endpoint/slug", nil)
	r.Header.Set("X-Signature", "00")
	err := h.validateHMACAuth(t.Context(), ep, r, []byte("{}"))
	if err == nil {
		t.Fatal("hmac endpoint without configured secret must fail auth")
	}
}

func TestValidateHMACAuthWrongStoredSecret(t *testing.T) {
	// The stored encrypted secret decrypts to a different key than the signer used.
	h, _ := newTestIngester(t)
	ep := hmacEndpoint(t, h, "actual-stored-secret")
	body := []byte(`{"a":1}`)
	r := httptest.NewRequest(http.MethodPost, "/endpoint/slug", nil)
	r.Header.Set("X-Signature", signHex("not-the-stored-secret", body))
	err := h.validateHMACAuth(t.Context(), ep, r, body)
	if err == nil {
		t.Fatal("signature computed with a different key must fail")
	}
}
