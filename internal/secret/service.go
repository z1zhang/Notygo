package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

// Service provides AES-256-GCM encryption/decryption for sensitive data.
// It supports multiple master keys for key rotation: the last key is used for
// new encryptions, while all keys can decrypt existing data.
type Service struct {
	keys          map[string][]byte // version -> 32-byte key
	activeVersion string            // version used for new encryptions
}

// NewService parses the ENCRYPTION_MASTER_KEY environment variable and creates a Service.
// Format: "v1:<base64_32bytes>" or comma-separated "v1:xxx,v2:yyy".
// The last key in the list is the active encryption key.
func NewService(masterKeyEnv string) (*Service, error) {
	keys := make(map[string][]byte)
	parts := strings.Split(masterKeyEnv, ",")
	var lastVersion string

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid key format %q, expected version:base64", part)
		}
		version := strings.TrimSpace(kv[0])
		keyB64 := strings.TrimSpace(kv[1])

		keyBytes, err := base64.StdEncoding.DecodeString(keyB64)
		if err != nil {
			return nil, fmt.Errorf("invalid base64 for key %s: %w", version, err)
		}
		if len(keyBytes) != 32 {
			return nil, fmt.Errorf("key %s must be 32 bytes (AES-256), got %d", version, len(keyBytes))
		}
		keys[version] = keyBytes
		lastVersion = version
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("no encryption keys provided")
	}

	return &Service{keys: keys, activeVersion: lastVersion}, nil
}

// ActiveVersion returns the key version currently used for new encryptions.
func (s *Service) ActiveVersion() string {
	return s.activeVersion
}

// Encrypt encrypts plaintext using the active master key.
// Output format: "v1:<base64_nonce>:<base64_ciphertext>:<base64_tag>"
func (s *Service) Encrypt(plaintext string) (string, error) {
	key := s.keys[s.activeVersion]
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	// gcm.Seal appends the authentication tag to the ciphertext
	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)

	tagSize := gcm.Overhead()
	ct := ciphertext[:len(ciphertext)-tagSize]
	tag := ciphertext[len(ciphertext)-tagSize:]

	return fmt.Sprintf("%s:%s:%s:%s",
		s.activeVersion,
		base64.StdEncoding.EncodeToString(nonce),
		base64.StdEncoding.EncodeToString(ct),
		base64.StdEncoding.EncodeToString(tag),
	), nil
}

// Decrypt decrypts a stored secret string.
// It automatically selects the correct key based on the version prefix.
func (s *Service) Decrypt(stored string) (string, error) {
	parts := strings.SplitN(stored, ":", 4)
	if len(parts) != 4 {
		return "", fmt.Errorf("invalid secret format: expected 4 colon-separated parts")
	}

	version := parts[0]
	key, ok := s.keys[version]
	if !ok {
		return "", fmt.Errorf("unknown key version: %s", version)
	}

	nonce, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("decode nonce: %w", err)
	}
	ct, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}
	tag, err := base64.StdEncoding.DecodeString(parts[3])
	if err != nil {
		return "", fmt.Errorf("decode tag: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	// GCM expects ciphertext || tag
	combined := append(ct, tag...)
	plaintext, err := gcm.Open(nil, nonce, combined, nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed (wrong key or tampered data)")
	}

	return string(plaintext), nil
}
