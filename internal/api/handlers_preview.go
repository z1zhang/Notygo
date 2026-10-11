package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"zpush/internal/templates"
)

// PreviewTemplate renders a template with sample data and returns the result.
// Uses the same simple {{.key}} placeholder replacement as the production
// event processor (internal/processor/processor.go) so that the preview
// matches actual delivery output.
func PreviewTemplate(db *sql.DB) http.HandlerFunc {
	repo := templates.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			TemplateID string         `json:"template_id"`
			SampleData map[string]any `json:"sample_data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, http.StatusBadRequest, ErrConfigValidation, "invalid request body")
			return
		}

		if req.TemplateID == "" {
			WriteError(w, http.StatusBadRequest, ErrConfigValidation, "template_id is required")
			return
		}

		if req.SampleData == nil {
			req.SampleData = map[string]any{}
		}

		// Look up the template
		tmpl, err := repo.GetByID(r.Context(), req.TemplateID)
		if err != nil {
			WriteError(w, http.StatusNotFound, ErrConfigValidation, "template not found")
			return
		}

		// Render using the same simple placeholder replacement as the production processor
		title := tmpl.TitleTemplate
		body := tmpl.BodyTemplate

		for key, val := range req.SampleData {
			placeholder := "{{." + key + "}}"
			replacement := fmt.Sprintf("%v", val)
			title = strings.ReplaceAll(title, placeholder, replacement)
			body = strings.ReplaceAll(body, placeholder, replacement)
		}

		WriteJSON(w, http.StatusOK, map[string]any{
			"title":  title,
			"body":   body,
			"format": tmpl.Format,
		})
	}
}
