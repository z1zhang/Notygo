package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"zpush/internal/audit"
	"zpush/internal/templates"
)

// ListTemplates returns a paginated list of templates.
func ListTemplates(db *sql.DB) http.HandlerFunc {
	repo := templates.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		page, size := ParsePagination(r)
		result, total, err := repo.List(r.Context(), page, size)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to list templates")
			return
		}
		WriteJSON(w, http.StatusOK, PaginatedResponse[templates.Template]{
			Data:  result,
			Total: total,
			Page:  page,
			Size:  size,
		})
	}
}

// CreateTemplate creates a new template.
func CreateTemplate(db *sql.DB, auditRepo *audit.Repository) http.HandlerFunc {
	repo := templates.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name          string `json:"name"`
			Format        string `json:"format"`
			TitleTemplate string `json:"title_template"`
			BodyTemplate  string `json:"body_template"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, http.StatusBadRequest, ErrConfigValidation, "invalid request body")
			return
		}

		if req.Name == "" {
			WriteError(w, http.StatusBadRequest, ErrConfigValidation, "name is required")
			return
		}

		if req.Format == "" {
			req.Format = "markdown"
		}

		// Validate template syntax
		if err := validateTemplateSyntax(req.TitleTemplate, req.BodyTemplate); err != nil {
			WriteError(w, http.StatusBadRequest, ErrTplSyntax, err.Error())
			return
		}

		created, err := repo.Create(r.Context(), req.Name, req.Format, req.TitleTemplate, req.BodyTemplate)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to create template")
			return
		}

		WriteJSON(w, http.StatusCreated, created)

		// Audit log
		if auditErr := auditRepo.Insert(r.Context(), &audit.AuditLog{
			Operator:   GetUsernameFromContext(r.Context()),
			Action:     "create",
			TargetType: "template",
			TargetID:   created.ID,
			Summary:    stringPtr(fmt.Sprintf("created template %q", created.Name)),
		}); auditErr != nil {
			slog.Error("audit log insert failed", "action", "create", "target_type", "template", "error", auditErr)
		}
	}
}

// GetTemplate returns a single template by ID.
func GetTemplate(db *sql.DB) http.HandlerFunc {
	repo := templates.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		t, err := repo.GetByID(r.Context(), id)
		if err != nil {
			WriteError(w, http.StatusNotFound, ErrConfigValidation, "template not found")
			return
		}
		WriteJSON(w, http.StatusOK, t)
	}
}

// UpdateTemplate updates an existing template, incrementing revision and validating syntax.
func UpdateTemplate(db *sql.DB, auditRepo *audit.Repository) http.HandlerFunc {
	repo := templates.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var req struct {
			Name          *string `json:"name,omitempty"`
			Format        *string `json:"format,omitempty"`
			TitleTemplate *string `json:"title_template,omitempty"`
			BodyTemplate  *string `json:"body_template,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, http.StatusBadRequest, ErrConfigValidation, "invalid request body")
			return
		}

		// Validate template syntax if provided
		titleVal := ""
		bodyVal := ""
		if req.TitleTemplate != nil {
			titleVal = *req.TitleTemplate
		}
		if req.BodyTemplate != nil {
			bodyVal = *req.BodyTemplate
		}
		if req.TitleTemplate != nil || req.BodyTemplate != nil {
			if err := validateTemplateSyntax(titleVal, bodyVal); err != nil {
				WriteError(w, http.StatusBadRequest, ErrTplSyntax, err.Error())
				return
			}
		}

		// Fetch old record for audit snapshot
		oldTemplate, _ := repo.GetByID(r.Context(), id)

		updated, err := repo.Update(r.Context(), id, req.Name, req.Format, req.TitleTemplate, req.BodyTemplate)
		if err != nil {
			WriteError(w, http.StatusNotFound, ErrConfigValidation, "template not found")
			return
		}

		WriteJSON(w, http.StatusOK, updated)

		// Audit log
		if auditErr := auditRepo.Insert(r.Context(), &audit.AuditLog{
			Operator:       GetUsernameFromContext(r.Context()),
			Action:         "update",
			TargetType:     "template",
			TargetID:       id,
			SnapshotBefore: marshalSnapshot(oldTemplate),
			Summary:        stringPtr(fmt.Sprintf("updated template %q", updated.Name)),
		}); auditErr != nil {
			slog.Error("audit log insert failed", "action", "update", "target_type", "template", "error", auditErr)
		}
	}
}

// DeleteTemplate deletes a template by ID.
func DeleteTemplate(db *sql.DB, auditRepo *audit.Repository) http.HandlerFunc {
	repo := templates.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		// Fetch old record for audit snapshot
		oldTemplate, _ := repo.GetByID(r.Context(), id)

		if err := repo.Delete(r.Context(), id); err != nil {
			WriteError(w, http.StatusNotFound, ErrConfigValidation, "template not found")
			return
		}
		WriteJSON(w, http.StatusNoContent, nil)

		// Audit log
		if auditErr := auditRepo.Insert(r.Context(), &audit.AuditLog{
			Operator:       GetUsernameFromContext(r.Context()),
			Action:         "delete",
			TargetType:     "template",
			TargetID:       id,
			SnapshotBefore: marshalSnapshot(oldTemplate),
			Summary:        stringPtr(fmt.Sprintf("deleted template %s", id)),
		}); auditErr != nil {
			slog.Error("audit log insert failed", "action", "delete", "target_type", "template", "error", auditErr)
		}
	}
}

// validateTemplateSyntax performs basic validation on template syntax.
func validateTemplateSyntax(title, body string) error {
	// Check for unbalanced {{ }} placeholders
	for _, s := range []string{title, body} {
		if err := checkTemplateBraces(s); err != nil {
			return err
		}
	}
	return nil
}

func checkTemplateBraces(s string) error {
	openCount := strings.Count(s, "{{")
	closeCount := strings.Count(s, "}}")
	if openCount != closeCount {
		return &ValidationError{Message: "unbalanced template braces: {{ and }} count mismatch"}
	}

	// Check for forbidden patterns
	if strings.Contains(s, "{{.") && !strings.Contains(s, "}}") {
		return &ValidationError{Message: "unclosed template placeholder"}
	}

	return nil
}
