package api

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"zpush/internal/mapping"
)

// ListMappings returns a paginated list of mappings.
func ListMappings(db *sql.DB) http.HandlerFunc {
	repo := mapping.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		page, size := ParsePagination(r)
		mappings, total, err := repo.List(r.Context(), page, size)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to list mappings")
			return
		}
		WriteJSON(w, http.StatusOK, PaginatedResponse[mapping.Mapping]{
			Data:  mappings,
			Total: total,
			Page:  page,
			Size:  size,
		})
	}
}

// CreateMapping creates a new mapping.
func CreateMapping(db *sql.DB) http.HandlerFunc {
	repo := mapping.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name   string              `json:"name"`
			Fields []mapping.MappingEntry `json:"fields"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, http.StatusBadRequest, ErrConfigValidation, "invalid request body")
			return
		}

		if req.Name == "" {
			WriteError(w, http.StatusBadRequest, ErrConfigValidation, "name is required")
			return
		}

		if req.Fields == nil {
			req.Fields = []mapping.MappingEntry{}
		}

		created, err := repo.Create(r.Context(), req.Name, req.Fields)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to create mapping")
			return
		}

		WriteJSON(w, http.StatusCreated, created)
	}
}

// GetMapping returns a single mapping by ID.
func GetMapping(db *sql.DB) http.HandlerFunc {
	repo := mapping.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		m, err := repo.GetByID(r.Context(), id)
		if err != nil {
			WriteError(w, http.StatusNotFound, ErrConfigValidation, "mapping not found")
			return
		}
		WriteJSON(w, http.StatusOK, m)
	}
}

// UpdateMapping updates an existing mapping, incrementing revision.
func UpdateMapping(db *sql.DB) http.HandlerFunc {
	repo := mapping.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var req struct {
			Name   *string              `json:"name,omitempty"`
			Fields *[]mapping.MappingEntry `json:"fields,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, http.StatusBadRequest, ErrConfigValidation, "invalid request body")
			return
		}

		updated, err := repo.Update(r.Context(), id, req.Name, req.Fields)
		if err != nil {
			WriteError(w, http.StatusNotFound, ErrConfigValidation, "mapping not found")
			return
		}

		WriteJSON(w, http.StatusOK, updated)
	}
}

// DeleteMapping deletes a mapping by ID.
func DeleteMapping(db *sql.DB) http.HandlerFunc {
	repo := mapping.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if err := repo.Delete(r.Context(), id); err != nil {
			WriteError(w, http.StatusNotFound, ErrConfigValidation, "mapping not found")
			return
		}
		WriteJSON(w, http.StatusNoContent, nil)
	}
}
