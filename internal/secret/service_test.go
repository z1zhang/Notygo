package secret

import (
	"encoding/base64"
	"strings"
	"testing"
)

// Generate a valid 32-byte key for testing
func testKey(t *testing.T) string {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	return base64.StdEncoding.EncodeToString(key)
}

func testKey2(t *testing.T) string {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 100)
	}
	return base64.StdEncoding.EncodeToString(key)
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	svc, err := NewService("v1:" + testKey(t))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	plaintext := "my-super-secret-token"
	encrypted, err := svc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Verify format
	parts := strings.SplitN(encrypted, ":", 4)
	if len(parts) != 4 {
		t.Fatalf("expected 4 parts, got %d", len(parts))
	}
	if parts[0] != "v1" {
		t.Fatalf("expected version v1, got %s", parts[0])
	}

	decrypted, err := svc.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if decrypted != plaintext {
		t.Fatalf("expected %q, got %q", plaintext, decrypted)
	}
}

func TestEncryptProducesDifferentCiphertexts(t *testing.T) {
	svc, err := NewService("v1:" + testKey(t))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	plaintext := "same-input"
	enc1, _ := svc.Encrypt(plaintext)
	enc2, _ := svc.Encrypt(plaintext)

	if enc1 == enc2 {
		t.Fatal("two encryptions of the same plaintext should produce different ciphertexts (different nonces)")
	}
}

func TestMultiKeyDecrypt(t *testing.T) {
	svc1, err := NewService("v1:" + testKey(t))
	if err != nil {
		t.Fatalf("NewService v1: %v", err)
	}

	plaintext := "encrypted-with-v1"
	encrypted, err := svc1.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt with v1: %v", err)
	}

	// Create a service with both v1 and v2, where v2 is active
	svc2, err := NewService("v1:" + testKey(t) + ",v2:" + testKey2(t))
	if err != nil {
		t.Fatalf("NewService v1+v2: %v", err)
	}

	if svc2.ActiveVersion() != "v2" {
		t.Fatalf("expected active version v2, got %s", svc2.ActiveVersion())
	}

	// Should still decrypt v1-encrypted data
	decrypted, err := svc2.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt v1 data with v1+v2 service: %v", err)
	}

	if decrypted != plaintext {
		t.Fatalf("expected %q, got %q", plaintext, decrypted)
	}

	// New encryptions should use v2
	newEnc, _ := svc2.Encrypt("new-data")
	if !strings.HasPrefix(newEnc, "v2:") {
		t.Fatal("new encryption should use v2 key")
	}
}

func TestDecryptWithWrongKey(t *testing.T) {
	svc1, err := NewService("v1:" + testKey(t))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	encrypted, _ := svc1.Encrypt("secret")

	// Try to decrypt with a different key
	svc2, err := NewService("v1:" + testKey2(t))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	// Same version "v1" but different key bytes
	_, err = svc2.Decrypt(encrypted)
	if err == nil {
		t.Fatal("expected decryption to fail with wrong key")
	}
}

func TestDecryptTamperedData(t *testing.T) {
	svc, err := NewService("v1:" + testKey(t))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	encrypted, _ := svc.Encrypt("secret")

	// Tamper with the ciphertext
	parts := strings.SplitN(encrypted, ":", 4)
	parts[2] = base64.StdEncoding.EncodeToString([]byte("tampered"))
	tampered := strings.Join(parts, ":")

	_, err = svc.Decrypt(tampered)
	if err == nil {
		t.Fatal("expected decryption to fail with tampered data")
	}
}

func TestInvalidMasterKey(t *testing.T) {
	tests := []struct {
		name string
		env  string
	}{
		{"empty", ""},
		{"wrong format", "not-a-key"},
		{"too short", "v1:" + base64.StdEncoding.EncodeToString([]byte("short"))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewService(tt.env)
			if err == nil {
				t.Fatal("expected error for invalid master key")
			}
		})
	}
}

func TestDecryptInvalidFormat(t *testing.T) {
	svc, err := NewService("v1:" + testKey(t))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	_, err = svc.Decrypt("invalid-format")
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
}
