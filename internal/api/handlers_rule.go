package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"zpush/internal/audit"
	"zpush/internal/rules"
)

// ListRules returns a paginated list of rules.
func ListRules(db *sql.DB) http.HandlerFunc {
	repo := rules.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		page, size := ParsePagination(r)
		result, total, err := repo.List(r.Context(), page, size)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to list rules")
			return
		}
		WriteJSON(w, http.StatusOK, PaginatedResponse[rules.Rule]{
			Data:  result,
			Total: total,
			Page:  page,
			Size:  size,
		})
	}
}

// CreateRule creates a new rule.
func CreateRule(db *sql.DB, auditRepo *audit.Repository) http.HandlerFunc {
	repo := rules.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		var rule rules.Rule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			WriteError(w, http.StatusBadRequest, ErrConfigValidation, "invalid request body")
			return
		}

		if rule.Name == "" {
			WriteError(w, http.StatusBadRequest, ErrConfigValidation, "name is required")
			return
		}

		// Validate conditions JSON
		if err := validateConditions(rule.Conditions); err != nil {
			WriteError(w, http.StatusBadRequest, ErrRuleConditionSyntax, err.Error())
			return
		}

		created, err := repo.Create(r.Context(), &rule)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to create rule")
			return
		}

		WriteJSON(w, http.StatusCreated, created)

		// Audit log
		if auditErr := auditRepo.Insert(r.Context(), &audit.AuditLog{
			Operator:   GetUsernameFromContext(r.Context()),
			Action:     "create",
			TargetType: "rule",
			TargetID:   created.ID,
			Summary:    stringPtr(fmt.Sprintf("created rule %q", created.Name)),
		}); auditErr != nil {
			slog.Error("audit log insert failed", "action", "create", "target_type", "rule", "error", auditErr)
		}
	}
}

// GetRule returns a single rule by ID.
func GetRule(db *sql.DB) http.HandlerFunc {
	repo := rules.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		rule, err := repo.GetByID(r.Context(), id)
		if err != nil {
			WriteError(w, http.StatusNotFound, ErrConfigValidation, "rule not found")
			return
		}
		WriteJSON(w, http.StatusOK, rule)
	}
}

// UpdateRule updates an existing rule, incrementing revision and validating conditions.
func UpdateRule(db *sql.DB, auditRepo *audit.Repository) http.HandlerFunc {
	repo := rules.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var rule rules.Rule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			WriteError(w, http.StatusBadRequest, ErrConfigValidation, "invalid request body")
			return
		}

		if rule.Name == "" {
			WriteError(w, http.StatusBadRequest, ErrConfigValidation, "name is required")
			return
		}

		// Validate conditions JSON
		if err := validateConditions(rule.Conditions); err != nil {
			WriteError(w, http.StatusBadRequest, ErrRuleConditionSyntax, err.Error())
			return
		}

		// Fetch old record for audit snapshot
		oldRule, _ := repo.GetByID(r.Context(), id)

		updated, err := repo.Update(r.Context(), id, &rule)
		if err != nil {
			WriteError(w, http.StatusNotFound, ErrConfigValidation, "rule not found")
			return
		}

		WriteJSON(w, http.StatusOK, updated)

		// Audit log
		if auditErr := auditRepo.Insert(r.Context(), &audit.AuditLog{
			Operator:       GetUsernameFromContext(r.Context()),
			Action:         "update",
			TargetType:     "rule",
			TargetID:       id,
			SnapshotBefore: marshalSnapshot(oldRule),
			Summary:        stringPtr(fmt.Sprintf("updated rule %q", updated.Name)),
		}); auditErr != nil {
			slog.Error("audit log insert failed", "action", "update", "target_type", "rule", "error", auditErr)
		}
	}
}

// DeleteRule deletes a rule by ID.
func DeleteRule(db *sql.DB, auditRepo *audit.Repository) http.HandlerFunc {
	repo := rules.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		// Fetch old record for audit snapshot
		oldRule, _ := repo.GetByID(r.Context(), id)

		if err := repo.Delete(r.Context(), id); err != nil {
			WriteError(w, http.StatusNotFound, ErrConfigValidation, "rule not found")
			return
		}
		WriteJSON(w, http.StatusNoContent, nil)

		// Audit log
		if auditErr := auditRepo.Insert(r.Context(), &audit.AuditLog{
			Operator:       GetUsernameFromContext(r.Context()),
			Action:         "delete",
			TargetType:     "rule",
			TargetID:       id,
			SnapshotBefore: marshalSnapshot(oldRule),
			Summary:        stringPtr(fmt.Sprintf("deleted rule %s", id)),
		}); auditErr != nil {
			slog.Error("audit log insert failed", "action", "delete", "target_type", "rule", "error", auditErr)
		}
	}
}

// validateConditions validates that a condition group has a valid structure.
func validateConditions(group rules.ConditionGroup) error {
	if len(group.Conditions) == 0 && len(group.Groups) == 0 {
		// Empty conditions are valid (matches everything)
		return nil
	}

	for _, cond := range group.Conditions {
		if cond.Field == "" {
			return &ValidationError{Message: "condition field is required"}
		}
		validOps := map[string]bool{
			"eq": true, "neq": true, "ne": true, "contains": true, "not_contains": true, "regex": true,
			"gt": true, "lt": true, "gte": true, "lte": true, "exists": true, "not_exists": true,
		}
		if !validOps[cond.Operator] {
			return &ValidationError{Message: "invalid condition operator: " + cond.Operator}
		}
	}

	for _, g := range group.Groups {
		if g.Operator != "" && g.Operator != "and" && g.Operator != "or" {
			return &ValidationError{Message: "invalid group operator: " + g.Operator}
		}
		if err := validateConditions(g); err != nil {
			return err
		}
	}

	return nil
}

// ValidationError represents a validation error.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}
