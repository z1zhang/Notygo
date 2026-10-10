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

		// Mask secrets in response
		for i := range result {
			result[i].SecretRef = maskDestSecretRef(result[i].SecretRef)
		}

		WriteJSON(w, http.StatusOK, PaginatedResponse[destination.Destination]{
			Data:  result,
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
		var dest destination.Destination
		if err := json.NewDecoder(r.Body).Decode(&dest); err != nil {
			WriteError(w, http.StatusBadRequest, ErrConfigValidation, "invalid request body")
			return
		}

		if dest.Name == "" {
			WriteError(w, http.StatusBadRequest, ErrConfigValidation, "name is required")
			return
		}
		if dest.Type == "" {
			WriteError(w, http.StatusBadRequest, ErrConfigValidation, "type is required")
			return
		}

		// Encrypt secret if provided
		if dest.SecretRef != nil && *dest.SecretRef != "" && secretSvc != nil {
			encrypted, err := secretSvc.Encrypt(*dest.SecretRef)
			if err != nil {
				WriteError(w, http.StatusInternalServerError, ErrSystemKeyInvalid, "failed to encrypt secret")
				return
			}
			dest.SecretRef = &encrypted
		}

		created, err := repo.Create(r.Context(), &dest)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to create destination")
			return
		}

		created.SecretRef = maskDestSecretRef(created.SecretRef)
		WriteJSON(w, http.StatusCreated, created)
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

		d.SecretRef = maskDestSecretRef(d.SecretRef)
		WriteJSON(w, http.StatusOK, d)
	}
}

// UpdateDestination updates an existing destination, incrementing revision.
func UpdateDestination(db *sql.DB, secretSvc *secret.Service) http.HandlerFunc {
	repo := destination.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var dest destination.Destination
		if err := json.NewDecoder(r.Body).Decode(&dest); err != nil {
			WriteError(w, http.StatusBadRequest, ErrConfigValidation, "invalid request body")
			return
		}

		if dest.Name == "" {
			WriteError(w, http.StatusBadRequest, ErrConfigValidation, "name is required")
			return
		}

		// Encrypt secret if provided
		if dest.SecretRef != nil && *dest.SecretRef != "" && secretSvc != nil {
			encrypted, err := secretSvc.Encrypt(*dest.SecretRef)
			if err != nil {
				WriteError(w, http.StatusInternalServerError, ErrSystemKeyInvalid, "failed to encrypt secret")
				return
			}
			dest.SecretRef = &encrypted
		}

		updated, err := repo.Update(r.Context(), id, &dest)
		if err != nil {
			WriteError(w, http.StatusNotFound, ErrConfigValidation, "destination not found")
			return
		}

		updated.SecretRef = maskDestSecretRef(updated.SecretRef)
		WriteJSON(w, http.StatusOK, updated)
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

// maskDestSecretRef masks the destination secret reference for display.
func maskDestSecretRef(ref *string) *string {
	if ref == nil || *ref == "" {
		s := "not_configured"
		return &s
	}
	s := "configured"
	return &s
}
