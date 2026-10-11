package api

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"

	"zpush/internal/storage"
)

// LoginHandler handles admin login with username/password.
func LoginHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, http.StatusBadRequest, ErrIngestAuthFailed, "invalid request body")
			return
		}

		if req.Username == "" || req.Password == "" {
			WriteError(w, http.StatusBadRequest, ErrIngestAuthFailed, "username and password are required")
			return
		}

		// Query admin user
		var id, passwordHash string
		err := db.QueryRowContext(r.Context(),
			`SELECT id, password_hash FROM admin_users WHERE username = ?`, req.Username,
		).Scan(&id, &passwordHash)
		if err != nil {
			WriteError(w, http.StatusUnauthorized, ErrIngestAuthFailed, "invalid credentials")
			return
		}

		// Compare bcrypt hash
		if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
			WriteError(w, http.StatusUnauthorized, ErrIngestAuthFailed, "invalid credentials")
			return
		}

		// Generate session token (32 random bytes hex)
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to generate token")
			return
		}
		token := hex.EncodeToString(tokenBytes)

		// Insert session with 24h expiry
		expiresAt := time.Now().UTC().Add(24 * time.Hour)
		sessionID := generateSessionID()
		nowExpr := storage.NowExpr()
		_, err = db.ExecContext(r.Context(),
			"INSERT INTO sessions (id, user_id, token, created_at, expires_at) VALUES (?, ?, ?, "+nowExpr+", ?)",
			sessionID, id, token, storage.FormatTime(expiresAt),
		)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to create session")
			return
		}

		WriteJSON(w, http.StatusOK, map[string]any{
			"token":      token,
			"expires_at": expiresAt.Format(time.RFC3339),
		})
	}
}

func generateSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
