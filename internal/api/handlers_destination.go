package api

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"zPush/internal/destination"
	"zPush/internal/secret"
)

// ListDestinations returns a paginated list of destinations.
func ListDestinations(db *sql.DB) http.HandlerFunc {
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
			responses[i] = toDestinationResponse(&result[i])
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
func CreateDestination(db *sql.DB, secretSvc *secret.Service) http.HandlerFunc {
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

		resp := toDestinationResponse(created)
		WriteJSON(w, http.StatusCreated, resp)
	}
}

// GetDestination returns a single destination by ID.
func GetDestination(db *sql.DB) http.HandlerFunc {
	repo := destination.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		d, err := repo.GetByID(r.Context(), id)
		if err != nil {
			WriteError(w, http.StatusNotFound, ErrConfigValidation, "destination not found")
			return
		}

		resp := toDestinationResponse(d)
		WriteJSON(w, http.StatusOK, resp)
	}
}

// UpdateDestination updates an existing destination, incrementing revision.
// Uses UpdateDestinationRequest with pointer fields for partial updates.
func UpdateDestination(db *sql.DB, secretSvc *secret.Service) http.HandlerFunc {
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

		updated, err := repo.Update(r.Context(), id, req)
		if err != nil {
			WriteError(w, http.StatusNotFound, ErrConfigValidation, "destination not found")
			return
		}

		resp := toDestinationResponse(updated)
		WriteJSON(w, http.StatusOK, resp)
	}
}

// DeleteDestination deletes a destination by ID.
func DeleteDestination(db *sql.DB) http.HandlerFunc {
	repo := destination.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if err := repo.Delete(r.Context(), id); err != nil {
			WriteError(w, http.StatusNotFound, ErrConfigValidation, "destination not found")
			return
		}
		WriteJSON(w, http.StatusNoContent, nil)
	}
}

// toDestinationResponse converts a Destination to a DestinationResponse, masking the secret.
func toDestinationResponse(d *destination.Destination) destination.DestinationResponse {
	maskedSecretRef := maskDestSecretRef(d.SecretRef)
	return destination.DestinationResponse{
		ID:             d.ID,
		Name:           d.Name,
		Type:           d.Type,
		Enabled:        d.Enabled,
		Config:         d.Config,
		SecretRef:      maskedSecretRef,
		TimeoutMs:      d.TimeoutMs,
		RateLimit:      d.RateLimit,
		CircuitBreaker: d.CircuitBreaker,
		Revision:       d.Revision,
		CreatedAt:      d.CreatedAt.Format("2006-01-02T15:04:05.999Z"),
		UpdatedAt:      d.UpdatedAt.Format("2006-01-02T15:04:05.999Z"),
	}
}

// maskDestSecretRef masks the destination secret reference for display.
func maskDestSecretRef(ref *string) *string {
	if ref == nil || *ref == "" {
		s := "not_configured"
		return &s
	}
	s := "configured"
	return &s
}
