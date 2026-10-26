package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"notygo/internal/audit"
	"notygo/internal/channel"
	"notygo/internal/secret"
	"notygo/internal/sender"
)

// ListChannels returns a paginated list of channels.
func ListChannels(db *pgxpool.Pool, secretSvc *secret.Service) http.HandlerFunc {
	repo := channel.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		page, size := ParsePagination(r)
		result, total, err := repo.List(r.Context(), page, size)
		if err != nil {
			slog.Error("list channels failed", "error", err)
			WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to list channels")
			return
		}

		// Convert to response DTOs with masked secrets
		responses := make([]channel.ChannelResponse, len(result))
		for i := range result {
			responses[i] = toChannelResponse(&result[i], secretSvc)
		}

		WriteJSON(w, http.StatusOK, PaginatedResponse[channel.ChannelResponse]{
			Data:  responses,
			Total: total,
			Page:  page,
			Size:  size,
		})
	}
}

// CreateChannel creates a new channel.
func CreateChannel(db *pgxpool.Pool, secretSvc *secret.Service, auditRepo *audit.Repository) http.HandlerFunc {
	repo := channel.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		var req channel.CreateChannelRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, http.StatusBadRequest, ErrConfigValidation, "invalid request body")
			return
		}

		if req.Name == nil || *req.Name == "" {
			WriteError(w, http.StatusBadRequest, ErrConfigValidation, "name is required")
			return
		}
		if req.Type == nil || *req.Type == "" {
			WriteError(w, http.StatusBadRequest, ErrConfigValidation, "type is required")
			return
		}
		if req.MaxAttempts != nil && (*req.MaxAttempts < 1 || *req.MaxAttempts > 100) {
			WriteError(w, http.StatusBadRequest, ErrConfigValidation, "max_attempts must be between 1 and 100")
			return
		}

		// Encrypt secret if provided
		if req.Secret != nil && *req.Secret != "" && secretSvc != nil {
			encrypted, err := secretSvc.Encrypt(*req.Secret)
			if err != nil {
				WriteError(w, http.StatusInternalServerError, ErrSystemKeyInvalid, "failed to encrypt secret")
				return
			}
			req.Secret = &encrypted
		}

		created, err := repo.Create(r.Context(), req)
		if err != nil {
			slog.Error("create channel failed", "error", err)
			WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to create channel")
			return
		}

		resp := toChannelResponse(created, secretSvc)
		WriteJSON(w, http.StatusCreated, resp)

		// Audit log
		if auditErr := auditRepo.Insert(r.Context(), &audit.AuditLog{
			Operator:   GetUsernameFromContext(r.Context()),
			Action:     "create",
			TargetType: "channel",
			TargetID:   created.ID,
			Summary:    stringPtr(fmt.Sprintf("created channel %q", created.Name)),
		}); auditErr != nil {
			slog.Error("audit log insert failed", "action", "create", "target_type", "channel", "error", auditErr)
		}
	}
}

// GetChannel returns a single channel by ID.
func GetChannel(db *pgxpool.Pool, secretSvc *secret.Service) http.HandlerFunc {
	repo := channel.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		d, err := repo.GetByID(r.Context(), id)
		if err != nil {
			WriteError(w, http.StatusNotFound, ErrConfigValidation, "channel not found")
			return
		}

		resp := toChannelResponse(d, secretSvc)
		WriteJSON(w, http.StatusOK, resp)
	}
}

// UpdateChannel updates an existing channel, incrementing revision.
// Uses UpdateChannelRequest with pointer fields for partial updates.
func UpdateChannel(db *pgxpool.Pool, secretSvc *secret.Service, auditRepo *audit.Repository) http.HandlerFunc {
	repo := channel.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var req channel.UpdateChannelRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, http.StatusBadRequest, ErrConfigValidation, "invalid request body")
			return
		}
		if req.MaxAttempts != nil && (*req.MaxAttempts < 1 || *req.MaxAttempts > 100) {
			WriteError(w, http.StatusBadRequest, ErrConfigValidation, "max_attempts must be between 1 and 100")
			return
		}

		// Encrypt secret if provided
		if req.Secret != nil && *req.Secret != "" && secretSvc != nil {
			encrypted, err := secretSvc.Encrypt(*req.Secret)
			if err != nil {
				WriteError(w, http.StatusInternalServerError, ErrSystemKeyInvalid, "failed to encrypt secret")
				return
			}
			req.Secret = &encrypted
		}

		// Fetch old record for audit snapshot
		oldDest, _ := repo.GetByID(r.Context(), id)

		updated, err := repo.Update(r.Context(), id, req)
		if err != nil {
			WriteError(w, http.StatusNotFound, ErrConfigValidation, "channel not found")
			return
		}

		resp := toChannelResponse(updated, secretSvc)
		WriteJSON(w, http.StatusOK, resp)

		// Audit log
		if auditErr := auditRepo.Insert(r.Context(), &audit.AuditLog{
			Operator:       GetUsernameFromContext(r.Context()),
			Action:         "update",
			TargetType:     "channel",
			TargetID:       id,
			SnapshotBefore: marshalSnapshot(oldDest),
			Summary:        stringPtr(fmt.Sprintf("updated channel %q", updated.Name)),
		}); auditErr != nil {
			slog.Error("audit log insert failed", "action", "update", "target_type", "channel", "error", auditErr)
		}
	}
}

// DeleteChannel deletes a channel by ID.
func DeleteChannel(db *pgxpool.Pool, auditRepo *audit.Repository) http.HandlerFunc {
	repo := channel.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		// Fetch old record for audit snapshot
		oldDest, _ := repo.GetByID(r.Context(), id)

		if err := repo.Delete(r.Context(), id); err != nil {
			WriteError(w, http.StatusNotFound, ErrConfigValidation, "channel not found")
			return
		}
		WriteJSON(w, http.StatusNoContent, nil)

		// Audit log
		if auditErr := auditRepo.Insert(r.Context(), &audit.AuditLog{
			Operator:       GetUsernameFromContext(r.Context()),
			Action:         "delete",
			TargetType:     "channel",
			TargetID:       id,
			SnapshotBefore: marshalSnapshot(oldDest),
			Summary:        stringPtr(fmt.Sprintf("deleted channel %s", id)),
		}); auditErr != nil {
			slog.Error("audit log insert failed", "action", "delete", "target_type", "channel", "error", auditErr)
		}
	}
}

// TestChannel sends a test message through the channel, exercising
// the same sender/config code path as real deliveries. The outcome (success
// or failure) is written to the audit log.
func TestChannel(db *pgxpool.Pool, secretSvc *secret.Service, senderRegistry *sender.Registry, auditRepo *audit.Repository) http.HandlerFunc {
	repo := channel.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		dest, err := repo.GetByID(r.Context(), id)
		if err != nil {
			WriteError(w, http.StatusNotFound, ErrConfigValidation, "channel not found")
			return
		}

		s, err := senderRegistry.Get(string(dest.Type))
		if err != nil {
			WriteError(w, http.StatusBadRequest, ErrConfigValidation, err.Error())
			return
		}

		// Build the exact config a real delivery would use
		cfg := channel.BuildSenderConfig(dest, secretSvc)
		if cfg.Timeout <= 0 {
			cfg.Timeout = 10 * time.Second
		}

		msg := sender.RenderedMessage{
			Title:  "Notygo 测试消息",
			Body:   fmt.Sprintf("渠道「%s」测试发送\n时间 %s", dest.Name, time.Now().Format("2006-01-02 15:04:05")),
			Format: "text",
		}

		result, err := s.Send(r.Context(), cfg, msg)

		// Transport-level failure
		if err != nil {
			auditChannelTest(r, auditRepo, dest, "failed", err.Error())
			WriteError(w, http.StatusBadGateway, "DELIVER_001", err.Error())
			return
		}

		// 2xx + no business error code = success
		businessOK := result.BusinessCode == "" || result.BusinessCode == "0"
		if result.StatusCode >= 200 && result.StatusCode < 300 && businessOK {
			auditChannelTest(r, auditRepo, dest, "success", fmt.Sprintf("status %d", result.StatusCode))
			WriteJSON(w, http.StatusOK, map[string]any{
				"success":       true,
				"status_code":   result.StatusCode,
				"response_body": truncateResponse(result.Body),
			})
			return
		}

		// HTTP-level or business-level failure (e.g. DingTalk errcode != 0)
		detail := fmt.Sprintf("status %d", result.StatusCode)
		if result.BusinessCode != "" && result.BusinessCode != "0" {
			detail = fmt.Sprintf("status %d, business code %s", result.StatusCode, result.BusinessCode)
		}
		if snippet := truncateResponse(result.Body); snippet != "" {
			detail += ": " + snippet
		}
		auditChannelTest(r, auditRepo, dest, "failed", detail)
		WriteError(w, http.StatusBadGateway, "DELIVER_004", "test send failed ("+detail+")")
	}
}

// auditChannelTest records a channel test-send outcome; a write
// failure is logged but never blocks the response.
func auditChannelTest(r *http.Request, auditRepo *audit.Repository, dest *channel.Channel, outcome, detail string) {
	if auditRepo == nil {
		return
	}
	summary := fmt.Sprintf("tested channel %q: %s (%s)", dest.Name, outcome, detail)
	if err := auditRepo.Insert(r.Context(), &audit.AuditLog{
		Operator:   GetUsernameFromContext(r.Context()),
		Action:     "test",
		TargetType: "channel",
		TargetID:   dest.ID,
		Summary:    &summary,
	}); err != nil {
		slog.Error("audit log insert failed", "action", "test", "target_type", "channel", "error", err)
	}
}

// truncateResponse clamps a sender response body for display.
func truncateResponse(body string) string {
	const max = 200
	if len(body) > max {
		return body[:max] + "...(truncated)"
	}
	return body
}

// toChannelResponse converts a Channel to a ChannelResponse, masking the secret.
func toChannelResponse(d *channel.Channel, secretSvc *secret.Service) channel.ChannelResponse {
	maskedSecret := secretForDisplay(d.EncryptedSecret, secretSvc)
	return channel.ChannelResponse{
		ID:          d.ID,
		Name:        d.Name,
		Type:        d.Type,
		Enabled:     d.Enabled,
		Config:      d.Config,
		Secret:      maskedSecret,
		TimeoutMs:   d.TimeoutMs,
		MaxAttempts: d.MaxAttempts,
		Revision:    d.Revision,
		CreatedAt:   formatTimeOrNil(d.CreatedAt),
		UpdatedAt:   formatTimeOrNil(d.UpdatedAt),
	}
}

// secretForDisplay returns the decrypted secret for display (channels and
// endpoints share this). The frontend uses type="password" to mask it
// visually; the user can click the eye icon to reveal the full value. Falls
// back to a status label if decryption fails.
func secretForDisplay(ref *string, secretSvc *secret.Service) *string {
	if ref == nil || *ref == "" {
		s := "not_configured"
		return &s
	}

	// Decrypt the secret so the frontend can display it (masked by <input type="password">)
	if secretSvc != nil {
		decrypted, err := secretSvc.Decrypt(*ref)
		if err == nil && decrypted != "" {
			return &decrypted
		}
	}

	s := "configured"
	return &s
}
