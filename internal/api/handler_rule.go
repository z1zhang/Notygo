package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"notygo/internal/errcode"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"notygo/internal/audit"
	"notygo/internal/rule"
)

// ListRules returns a paginated list of rules.
func ListRules(db *pgxpool.Pool) http.HandlerFunc {
	repo := rule.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		page, size := ParsePagination(r)
		result, total, err := repo.List(r.Context(), page, size)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, errcode.SystemInternal, "failed to list rules")
			return
		}
		WriteJSON(w, http.StatusOK, PaginatedResponse[rule.Rule]{
			Data:  result,
			Total: total,
			Page:  page,
			Size:  size,
		})
	}
}

// CreateRule creates a new rule.
func CreateRule(db *pgxpool.Pool, auditRepo *audit.Repository) http.HandlerFunc {
	repo := rule.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		var req rule.Rule
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, http.StatusBadRequest, errcode.ConfigValidation, "invalid request body")
			return
		}

		if req.Name == "" {
			WriteError(w, http.StatusBadRequest, errcode.ConfigValidation, "name is required")
			return
		}

		// Validate conditions JSON
		if err := validateConditions(req.Conditions); err != nil {
			WriteError(w, http.StatusBadRequest, errcode.RuleConditionSyntax, err.Error())
			return
		}

		created, err := repo.Create(r.Context(), &req)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, errcode.SystemInternal, "failed to create rule")
			return
		}

		WriteJSON(w, http.StatusCreated, created)

		// Audit log
		if auditErr := auditRepo.Insert(r.Context(), &audit.AuditLog{
			Operator:   GetUsernameFromContext(r.Context()),
			Action:     "create",
			TargetType: "rule",
			TargetID:   created.ID,
			Summary:    new(fmt.Sprintf("created rule %q", created.Name)),
		}); auditErr != nil {
			slog.Error("audit log insert failed", "action", "create", "target_type", "rule", "error", auditErr)
		}
	}
}

// GetRule returns a single rule by ID.
func GetRule(db *pgxpool.Pool) http.HandlerFunc {
	repo := rule.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		rule, err := repo.GetByID(r.Context(), id)
		if err != nil {
			WriteError(w, http.StatusNotFound, errcode.ConfigValidation, "rule not found")
			return
		}
		WriteJSON(w, http.StatusOK, rule)
	}
}

// UpdateRule updates an existing rule, incrementing revision and validating conditions.
func UpdateRule(db *pgxpool.Pool, auditRepo *audit.Repository) http.HandlerFunc {
	repo := rule.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var req rule.Rule
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, http.StatusBadRequest, errcode.ConfigValidation, "invalid request body")
			return
		}

		if req.Name == "" {
			WriteError(w, http.StatusBadRequest, errcode.ConfigValidation, "name is required")
			return
		}

		// Validate conditions JSON
		if err := validateConditions(req.Conditions); err != nil {
			WriteError(w, http.StatusBadRequest, errcode.RuleConditionSyntax, err.Error())
			return
		}

		// Fetch old record for audit snapshot
		oldRule, _ := repo.GetByID(r.Context(), id)

		updated, err := repo.Update(r.Context(), id, &req)
		if err != nil {
			WriteError(w, http.StatusNotFound, errcode.ConfigValidation, "rule not found")
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
			Summary:        new(fmt.Sprintf("updated rule %q", updated.Name)),
		}); auditErr != nil {
			slog.Error("audit log insert failed", "action", "update", "target_type", "rule", "error", auditErr)
		}
	}
}

// DeleteRule deletes a rule by ID.
func DeleteRule(db *pgxpool.Pool, auditRepo *audit.Repository) http.HandlerFunc {
	repo := rule.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		// Fetch old record for audit snapshot
		oldRule, _ := repo.GetByID(r.Context(), id)

		if err := repo.Delete(r.Context(), id); err != nil {
			WriteError(w, http.StatusNotFound, errcode.ConfigValidation, "rule not found")
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
			Summary:        new(fmt.Sprintf("deleted rule %s", id)),
		}); auditErr != nil {
			slog.Error("audit log insert failed", "action", "delete", "target_type", "rule", "error", auditErr)
		}
	}
}

// validateConditions validates that a condition group has a valid structure.
func validateConditions(group rule.ConditionGroup) error {
	if len(group.Conditions) == 0 && len(group.Groups) == 0 {
		// Empty conditions are valid (matches everything)
		return nil
	}

	for _, cond := range group.Conditions {
		if cond.Field == "" {
			return &ValidationError{Message: "condition field is required"}
		}
		if !rule.ValidOperators[cond.Operator] {
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
