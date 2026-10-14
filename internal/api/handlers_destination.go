package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"zpush/internal/audit"
	"zpush/internal/destination"
	"zpush/internal/secret"
)

// ListDestinations returns a paginated list of destinations.
func ListDestinations(db *sql.DB, secretSvc *secret.Service) http.HandlerFunc {
	repo := destination.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		page, size := ParsePagination(r)
		result, total, err := repo.List(r.Context(), page, size)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to list destinations")
			return
		}

		// Convert to response DTOs with masked secrets
		responses := make([]destination.DestinationResponse, len(result))
		for i := range result {
			responses[i] = toDestinationResponse(&result[i], secretSvc)
		}

		WriteJSON(w, http.StatusOK, PaginatedResponse[destination.DestinationResponse]{
			Data:  responses,
			Total: total,
			Page:  page,
			Size:  size,
		})
	}
}

// CreateDestination creates a new destination.
func CreateDestination(db *sql.DB, secretSvc *secret.Service, auditRepo *audit.Repository) http.HandlerFunc {
	repo := destination.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		var req destination.CreateDestinationRequest
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

		// Encrypt secret if provided
		if req.SecretRef != nil && *req.SecretRef != "" && secretSvc != nil {
			encrypted, err := secretSvc.Encrypt(*req.SecretRef)
			if err != nil {
				WriteError(w, http.StatusInternalServerError, ErrSystemKeyInvalid, "failed to encrypt secret")
				return
			}
			req.SecretRef = &encrypted
		}

		created, err := repo.Create(r.Context(), req)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to create destination")
			return
		}

		resp := toDestinationResponse(created, secretSvc)
		WriteJSON(w, http.StatusCreated, resp)

		// Audit log
		if auditErr := auditRepo.Insert(r.Context(), &audit.AuditLog{
			Operator:   GetUsernameFromContext(r.Context()),
			Action:     "create",
			TargetType: "destination",
			TargetID:   created.ID,
			Summary:    stringPtr(fmt.Sprintf("created destination %q", created.Name)),
		}); auditErr != nil {
			slog.Error("audit log insert failed", "action", "create", "target_type", "destination", "error", auditErr)
		}
	}
}

// GetDestination returns a single destination by ID.
func GetDestination(db *sql.DB, secretSvc *secret.Service) http.HandlerFunc {
	repo := destination.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		d, err := repo.GetByID(r.Context(), id)
		if err != nil {
			WriteError(w, http.StatusNotFound, ErrConfigValidation, "destination not found")
			return
		}

		resp := toDestinationResponse(d, secretSvc)
		WriteJSON(w, http.StatusOK, resp)
	}
}

// UpdateDestination updates an existing destination, incrementing revision.
// Uses UpdateDestinationRequest with pointer fields for partial updates.
func UpdateDestination(db *sql.DB, secretSvc *secret.Service, auditRepo *audit.Repository) http.HandlerFunc {
	repo := destination.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var req destination.UpdateDestinationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, http.StatusBadRequest, ErrConfigValidation, "invalid request body")
			return
		}

		// Encrypt secret if provided
		if req.SecretRef != nil && *req.SecretRef != "" && secretSvc != nil {
			encrypted, err := secretSvc.Encrypt(*req.SecretRef)
			if err != nil {
				WriteError(w, http.StatusInternalServerError, ErrSystemKeyInvalid, "failed to encrypt secret")
				return
			}
			req.SecretRef = &encrypted
		}

		// Fetch old record for audit snapshot
		oldDest, _ := repo.GetByID(r.Context(), id)

		updated, err := repo.Update(r.Context(), id, req)
		if err != nil {
			WriteError(w, http.StatusNotFound, ErrConfigValidation, "destination not found")
			return
		}

		resp := toDestinationResponse(updated, secretSvc)
		WriteJSON(w, http.StatusOK, resp)

		// Audit log
		if auditErr := auditRepo.Insert(r.Context(), &audit.AuditLog{
			Operator:       GetUsernameFromContext(r.Context()),
			Action:         "update",
			TargetType:     "destination",
			TargetID:       id,
			SnapshotBefore: marshalSnapshot(oldDest),
			Summary:        stringPtr(fmt.Sprintf("updated destination %q", updated.Name)),
		}); auditErr != nil {
			slog.Error("audit log insert failed", "action", "update", "target_type", "destination", "error", auditErr)
		}
	}
}

// DeleteDestination deletes a destination by ID.
func DeleteDestination(db *sql.DB, auditRepo *audit.Repository) http.HandlerFunc {
	repo := destination.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		// Fetch old record for audit snapshot
		oldDest, _ := repo.GetByID(r.Context(), id)

		if err := repo.Delete(r.Context(), id); err != nil {
			WriteError(w, http.StatusNotFound, ErrConfigValidation, "destination not found")
			return
		}
		WriteJSON(w, http.StatusNoContent, nil)

		// Audit log
		if auditErr := auditRepo.Insert(r.Context(), &audit.AuditLog{
			Operator:       GetUsernameFromContext(r.Context()),
			Action:         "delete",
			TargetType:     "destination",
			TargetID:       id,
			SnapshotBefore: marshalSnapshot(oldDest),
			Summary:        stringPtr(fmt.Sprintf("deleted destination %s", id)),
		}); auditErr != nil {
			slog.Error("audit log insert failed", "action", "delete", "target_type", "destination", "error", auditErr)
		}
	}
}

// toDestinationResponse converts a Destination to a DestinationResponse, masking the secret.
func toDestinationResponse(d *destination.Destination, secretSvc *secret.Service) destination.DestinationResponse {
	maskedSecretRef := maskDestSecretRef(d.SecretRef, secretSvc)
	return destination.DestinationResponse{
		ID:        d.ID,
		Name:      d.Name,
		Type:      d.Type,
		Enabled:   d.Enabled,
		Config:    d.Config,
		SecretRef: maskedSecretRef,
		TimeoutMs: d.TimeoutMs,
		Revision:  d.Revision,
		CreatedAt: formatTimeOrNil(d.CreatedAt),
		UpdatedAt: formatTimeOrNil(d.UpdatedAt),
	}
}

// maskDestSecretRef returns the decrypted secret for display.
// The frontend uses type="password" to mask it visually; the user can click
// the eye icon to reveal the full value. Falls back to a status label if
// decryption fails.
func maskDestSecretRef(ref *string, secretSvc *secret.Service) *string {
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
