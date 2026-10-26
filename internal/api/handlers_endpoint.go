package api

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"notygo/internal/audit"
	"notygo/internal/endpoint"
	"notygo/internal/secret"
)

// generateSlug generates a random 32-character UUID slug (no hyphens).
func generateSlug() string {
	var uuid [16]byte
	_, _ = rand.Read(uuid[:])
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x%04x%04x%04x%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}

// ListEndpoints returns a paginated list of endpoints.
func ListEndpoints(db *pgxpool.Pool, secretSvc *secret.Service) http.HandlerFunc {
	repo := endpoint.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		page, size := ParsePagination(r)
		endpoints, total, err := repo.List(r.Context(), page, size)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to list endpoints")
			return
		}
		responses := make([]endpoint.EndpointResponse, len(endpoints))
		for i := range endpoints {
			responses[i] = toEndpointResponse(&endpoints[i], secretSvc)
		}
		WriteJSON(w, http.StatusOK, PaginatedResponse[endpoint.EndpointResponse]{
			Data:  responses,
			Total: total,
			Page:  page,
			Size:  size,
		})
	}
}

// CreateEndpoint creates a new endpoint.
func CreateEndpoint(db *pgxpool.Pool, secretSvc *secret.Service, auditRepo *audit.Repository) http.HandlerFunc {
	repo := endpoint.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		var req endpoint.CreateEndpointRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, http.StatusBadRequest, ErrConfigValidation, "invalid request body")
			return
		}

		if req.Name == "" {
			WriteError(w, http.StatusBadRequest, ErrConfigValidation, "name is required")
			return
		}

		if req.AuthType != nil && !endpoint.IsValidAuthType(*req.AuthType) {
			WriteError(w, http.StatusBadRequest, ErrConfigValidation, fmt.Sprintf("invalid auth_type %q", *req.AuthType))
			return
		}

		// Auto-generate 32-char UUID slug if not provided
		if req.Slug == "" {
			req.Slug = generateSlug()
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
			WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to create endpoint")
			return
		}

		// Audit log
		if auditErr := auditRepo.Insert(r.Context(), &audit.AuditLog{
			Operator:   GetUsernameFromContext(r.Context()),
			Action:     "create",
			TargetType: "endpoint",
			TargetID:   created.ID,
			Summary:    stringPtr(fmt.Sprintf("created endpoint %q", created.Name)),
		}); auditErr != nil {
			slog.Error("audit log insert failed", "action", "create", "target_type", "endpoint", "error", auditErr)
		}

		resp := toEndpointResponse(created, secretSvc)
		WriteJSON(w, http.StatusCreated, resp)

	}
}

// GetEndpoint returns a single endpoint by ID.
func GetEndpoint(db *pgxpool.Pool, secretSvc *secret.Service) http.HandlerFunc {
	repo := endpoint.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		s, err := repo.GetByID(r.Context(), id)
		if err != nil {
			if errors.Is(err, endpoint.ErrNotFound) {
				WriteError(w, http.StatusNotFound, ErrConfigValidation, "endpoint not found")
			} else {
				WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to get endpoint")
			}
			return
		}

		resp := toEndpointResponse(s, secretSvc)
		WriteJSON(w, http.StatusOK, resp)
	}
}

// UpdateEndpoint updates an existing endpoint.
func UpdateEndpoint(db *pgxpool.Pool, secretSvc *secret.Service, auditRepo *audit.Repository) http.HandlerFunc {
	repo := endpoint.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var req endpoint.UpdateEndpointRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, http.StatusBadRequest, ErrConfigValidation, "invalid request body")
			return
		}

		if req.AuthType != nil && !endpoint.IsValidAuthType(*req.AuthType) {
			WriteError(w, http.StatusBadRequest, ErrConfigValidation, fmt.Sprintf("invalid auth_type %q", *req.AuthType))
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
		oldEndpoint, _ := repo.GetByID(r.Context(), id)

		updated, err := repo.Update(r.Context(), id, req)
		if err != nil {
			if errors.Is(err, endpoint.ErrNotFound) {
				WriteError(w, http.StatusNotFound, ErrConfigValidation, "endpoint not found")
			} else {
				WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to update endpoint")
			}
			return
		}

		// Audit log
		if auditErr := auditRepo.Insert(r.Context(), &audit.AuditLog{
			Operator:       GetUsernameFromContext(r.Context()),
			Action:         "update",
			TargetType:     "endpoint",
			TargetID:       id,
			SnapshotBefore: marshalSnapshot(oldEndpoint),
			Summary:        stringPtr(fmt.Sprintf("updated endpoint %q", updated.Name)),
		}); auditErr != nil {
			slog.Error("audit log insert failed", "action", "update", "target_type", "endpoint", "error", auditErr)
		}

		resp := toEndpointResponse(updated, secretSvc)
		WriteJSON(w, http.StatusOK, resp)
	}
}

// DeleteEndpoint deletes a endpoint by ID.
func DeleteEndpoint(db *pgxpool.Pool, auditRepo *audit.Repository) http.HandlerFunc {
	repo := endpoint.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		// Fetch old record for audit snapshot
		oldEndpoint, _ := repo.GetByID(r.Context(), id)

		if err := repo.Delete(r.Context(), id); err != nil {
			WriteError(w, http.StatusNotFound, ErrConfigValidation, "endpoint not found")
			return
		}

		// Audit log
		if auditErr := auditRepo.Insert(r.Context(), &audit.AuditLog{
			Operator:       GetUsernameFromContext(r.Context()),
			Action:         "delete",
			TargetType:     "endpoint",
			TargetID:       id,
			SnapshotBefore: marshalSnapshot(oldEndpoint),
			Summary:        stringPtr(fmt.Sprintf("deleted endpoint %s", id)),
		}); auditErr != nil {
			slog.Error("audit log insert failed", "action", "delete", "target_type", "endpoint", "error", auditErr)
		}

		WriteJSON(w, http.StatusNoContent, nil)
	}
}

// toEndpointResponse converts an Endpoint to an EndpointResponse, decrypting
// the secret for display (same semantics as channels: the frontend masks it
// with type="password" and only re-submits on change).
func toEndpointResponse(s *endpoint.Endpoint, secretSvc *secret.Service) endpoint.EndpointResponse {
	return endpoint.EndpointResponse{
		ID:                 s.ID,
		Name:               s.Name,
		Slug:               s.Slug,
		Enabled:            s.Enabled,
		AuthType:           s.AuthType,
		Secret:             secretForDisplay(s.EncryptedSecret, secretSvc),
		PayloadContentType: s.PayloadContentType,
		MaxBodyBytes:       s.MaxBodyBytes,
		RateLimit:          s.RateLimit,
		MappingID:          s.MappingID,
		CreatedAt:          formatTimeOrNil(s.CreatedAt),
		UpdatedAt:          formatTimeOrNil(s.UpdatedAt),
	}
}
