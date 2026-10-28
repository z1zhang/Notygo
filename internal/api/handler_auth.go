package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"notygo/internal/errcode"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/jackc/pgx/v5/pgxpool"
	"notygo/internal/audit"
	"notygo/internal/storage"
)

// dummyHash evens out response timing: when the username does not exist we
// still run a bcrypt comparison so attackers cannot enumerate valid
// usernames by measuring response latency.
var dummyHash = func() string {
	b, err := bcrypt.GenerateFromPassword([]byte("notygo-dummy-password"), bcrypt.DefaultCost)
	if err != nil {
		// bcrypt with a valid cost never fails; keep a constant anyway
		return "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"
	}
	return string(b)
}()

// LoginHandler handles admin login with username/password.
// Every attempt (success or failure) is written to the audit log with the
// client IP; repeated failures trigger a temporary lockout per IP and per
// username.
func LoginHandler(db *pgxpool.Pool, auditRepo *audit.Repository, guard *loginGuard) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, http.StatusBadRequest, errcode.IngestAuthFailed, "invalid request body")
			return
		}

		if req.Username == "" || req.Password == "" {
			WriteError(w, http.StatusBadRequest, errcode.IngestAuthFailed, "username and password are required")
			return
		}

		ip := clientIP(r)
		ipKey := "ip:" + ip
		userKey := "user:" + req.Username

		// Locked-out attempts are rejected before any credential work;
		// they are intentionally not audited individually so a flood of
		// blocked probes cannot bloat the audit table (the failures that
		// caused the lockout are already recorded with the same IP).
		if guard.isLocked(ipKey) || guard.isLocked(userKey) {
			WriteError(w, http.StatusTooManyRequests, errcode.AuthLocked,
				"too many failed login attempts, try again later")
			return
		}

		// Query admin user
		var id, passwordHash string
		err := db.QueryRow(r.Context(),
			`SELECT id, password FROM notygo_user WHERE username = $1`, req.Username,
		).Scan(&id, &passwordHash)
		if err != nil {
			// Equalize timing with the existing-user path
			_ = bcrypt.CompareHashAndPassword([]byte(dummyHash), []byte(req.Password))
			guard.recordFailure(ipKey)
			guard.recordFailure(userKey)
			slog.Warn("login failed: unknown user", "username", req.Username, "remote_ip", ip)
			writeLoginAudit(r.Context(), auditRepo, "auth.login_failed", req.Username, "-",
				"ip="+ip+" username="+req.Username+" reason=invalid_credentials")
			WriteError(w, http.StatusUnauthorized, errcode.IngestAuthFailed, "invalid credentials")
			return
		}

		// Compare bcrypt hash
		if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
			guard.recordFailure(ipKey)
			guard.recordFailure(userKey)
			slog.Warn("login failed: wrong password", "username", req.Username, "remote_ip", ip)
			writeLoginAudit(r.Context(), auditRepo, "auth.login_failed", req.Username, id,
				"ip="+ip+" username="+req.Username+" reason=invalid_credentials")
			WriteError(w, http.StatusUnauthorized, errcode.IngestAuthFailed, "invalid credentials")
			return
		}

		// Generate session token (32 random bytes hex)
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			slog.Error("failed to generate session token", "username", req.Username, "error", err)
			WriteError(w, http.StatusInternalServerError, errcode.SystemInternal, "failed to generate token")
			return
		}
		token := hex.EncodeToString(tokenBytes)

		// Insert session with 24h expiry. Only the SHA-256 of the token is
		// stored: the raw token exists solely in the caller's browser, so a
		// database leak cannot be replayed as valid sessions.
		expiresAt := time.Now().UTC().Add(24 * time.Hour)
		sessionID := generateSessionID()
		nowExpr := storage.NowExpr()
		_, err = db.Exec(r.Context(),
			"INSERT INTO notygo_session (id, user_id, token_hash, created_at, expires_at) VALUES ($1, $2, $3, "+nowExpr+", $4)",
			sessionID, id, hashToken(token), expiresAt,
		)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, errcode.SystemInternal, "failed to create session")
			return
		}

		guard.reset(ipKey)
		guard.reset(userKey)
		writeLoginAudit(r.Context(), auditRepo, "auth.login_success", req.Username, id,
			"ip="+ip+" username="+req.Username)

		WriteJSON(w, http.StatusOK, map[string]any{
			"token":      token,
			"expires_at": expiresAt.Format(time.RFC3339),
		})
	}
}

// writeLoginAudit records a login attempt in the audit trail; a write
// failure is logged but never blocks the login response.
func writeLoginAudit(ctx context.Context, auditRepo *audit.Repository, action, operator, targetID, summary string) {
	summaryCopy := summary
	err := auditRepo.Insert(ctx, &audit.AuditLog{
		Operator:   operator,
		Action:     action,
		TargetType: "admin_user",
		TargetID:   targetID,
		Summary:    &summaryCopy,
	})
	if err != nil {
		slog.Error("write login audit log", "action", action, "error", err)
	}
}

func generateSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// hashToken returns the hex SHA-256 of a bearer token. Sessions store only
// this hash; lookups hash the incoming token the same way.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
