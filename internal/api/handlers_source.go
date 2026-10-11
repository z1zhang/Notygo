package api

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"zpush/internal/secret"
	"zpush/internal/source"
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

// ListSources returns a paginated list of sources.
func ListSources(db *sql.DB) http.HandlerFunc {
	repo := source.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		page, size := ParsePagination(r)
		sources, total, err := repo.List(r.Context(), page, size)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to list sources")
			return
		}
		WriteJSON(w, http.StatusOK, PaginatedResponse[source.Source]{
			Data:  sources,
			Total: total,
			Page:  page,
			Size:  size,
		})
	}
}

// CreateSource creates a new source.
func CreateSource(db *sql.DB, secretSvc *secret.Service) http.HandlerFunc {
	repo := source.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		var req source.CreateSourceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, http.StatusBadRequest, ErrConfigValidation, "invalid request body")
			return
		}

		if req.Name == "" {
			WriteError(w, http.StatusBadRequest, ErrConfigValidation, "name is required")
			return
		}

		// Auto-generate 6-char random slug if not provided
		if req.Slug == "" {
			req.Slug = generateSlug()
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
			WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to create source")
			return
		}

		// Mask secret in response
		resp := toSourceResponse(created)
		WriteJSON(w, http.StatusCreated, resp)
	}
}

// GetSource returns a single source by ID.
func GetSource(db *sql.DB) http.HandlerFunc {
	repo := source.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		s, err := repo.GetByID(r.Context(), id)
		if err != nil {
			WriteError(w, http.StatusNotFound, ErrConfigValidation, "source not found")
			return
		}

		resp := toSourceResponse(s)
		WriteJSON(w, http.StatusOK, resp)
	}
}

// UpdateSource updates an existing source.
func UpdateSource(db *sql.DB, secretSvc *secret.Service) http.HandlerFunc {
	repo := source.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var req source.UpdateSourceRequest
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

		updated, err := repo.Update(r.Context(), id, req)
		if err != nil {
			WriteError(w, http.StatusNotFound, ErrConfigValidation, "source not found")
			return
		}

		resp := toSourceResponse(updated)
		WriteJSON(w, http.StatusOK, resp)
	}
}

// DeleteSource deletes a source by ID.
func DeleteSource(db *sql.DB) http.HandlerFunc {
	repo := source.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if err := repo.Delete(r.Context(), id); err != nil {
			WriteError(w, http.StatusNotFound, ErrConfigValidation, "source not found")
			return
		}
		WriteJSON(w, http.StatusNoContent, nil)
	}
}

// toSourceResponse converts a Source to a SourceResponse, masking the secret.
func toSourceResponse(s *source.Source) source.SourceResponse {
	maskedSecretRef := maskSecretRef(s.SecretRef)
	return source.SourceResponse{
		ID:                 s.ID,
		Name:               s.Name,
		Slug:               s.Slug,
		Enabled:            s.Enabled,
		AuthType:           s.AuthType,
		SecretRef:          maskedSecretRef,
		PayloadContentType: s.PayloadContentType,
		MaxBodyBytes:       s.MaxBodyBytes,
		RateLimit:          s.RateLimit,
		MappingID:          s.MappingID,
		ExpectedResponse:   s.ExpectedResponse,
		CreatedAt:          formatTimeOrNil(s.CreatedAt),
		UpdatedAt:          formatTimeOrNil(s.UpdatedAt),
	}
}

// maskSecretRef masks the secret reference for display.
func maskSecretRef(ref *string) *string {
	if ref == nil || *ref == "" {
		s := "not_configured"
		return &s
	}
	s := "configured"
	return &s
}
