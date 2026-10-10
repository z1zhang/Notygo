package rules

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ConditionItem can hold either a Condition or a nested ConditionGroup.
// It is stored as json.RawMessage and decoded on demand.
type ConditionItem json.RawMessage

// Evaluate checks whether a rule's conditions match against the provided fields.
func Evaluate(rule *Rule, fields map[string]any) (bool, error) {
	if !rule.Enabled {
		return false, nil
	}
	return evaluateGroup(&rule.Conditions, fields)
}

// evaluateGroup evaluates a ConditionGroup recursively.
func evaluateGroup(group *ConditionGroup, fields map[string]any) (bool, error) {
	// Evaluate direct conditions
	var conditionResults []bool
	for i := range group.Conditions {
		result, err := evaluateCondition(&group.Conditions[i], fields)
		if err != nil {
			return false, err
		}
		conditionResults = append(conditionResults, result)
	}

	// Evaluate nested groups
	for i := range group.Groups {
		result, err := evaluateGroup(&group.Groups[i], fields)
		if err != nil {
			return false, err
		}
		conditionResults = append(conditionResults, result)
	}

	if len(conditionResults) == 0 {
		return true, nil // empty group evaluates to true
	}

	op := group.Operator
	if op == "" {
		op = "and" // default to AND
	}

	switch op {
	case "and":
		for _, r := range conditionResults {
			if !r {
				return false, nil
			}
		}
		return true, nil
	case "or":
		for _, r := range conditionResults {
			if r {
				return true, nil
			}
		}
		return false, nil
	default:
		return false, fmt.Errorf("unknown condition group operator %q", op)
	}
}

// evaluateCondition evaluates a single Condition against the fields map.
func evaluateCondition(cond *Condition, fields map[string]any) (bool, error) {
	fieldVal := resolveFieldPath(cond.Field, fields)

	switch cond.Operator {
	case "eq":
		return compareEqual(fieldVal, cond.Value)

	case "ne", "neq":
		result, err := compareEqual(fieldVal, cond.Value)
		if err != nil {
			return false, err
		}
		return !result, nil

	case "gt":
		return compareNumbers(fieldVal, cond.Value, func(a, b float64) bool { return a > b })

	case "lt":
		return compareNumbers(fieldVal, cond.Value, func(a, b float64) bool { return a < b })

	case "gte":
		return compareNumbers(fieldVal, cond.Value, func(a, b float64) bool { return a >= b })

	case "lte":
		return compareNumbers(fieldVal, cond.Value, func(a, b float64) bool { return a <= b })

	case "contains":
		return compareContains(fieldVal, cond.Value)

	case "not_contains":
		result, err := compareContains(fieldVal, cond.Value)
		if err != nil {
			return false, err
		}
		return !result, nil

	case "regex":
		return matchRegex(fieldVal, cond.Value)

	case "exists":
		return fieldVal != nil, nil

	case "not_exists":
		return fieldVal == nil, nil

	default:
		return false, fmt.Errorf("unknown operator %q", cond.Operator)
	}
}

// resolveFieldPath resolves a "fields.X" or dotted path from the fields map.
// If the path starts with "fields.", it strips that prefix.
func resolveFieldPath(path string, fields map[string]any) any {
	cleanPath := path
	if strings.HasPrefix(cleanPath, "fields.") {
		cleanPath = strings.TrimPrefix(cleanPath, "fields.")
	}

	parts := strings.Split(cleanPath, ".")
	var current any = fields

	for _, part := range parts {
		if current == nil {
			return nil
		}
		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = m[part]
	}
	return current
}

// compareEqual checks equality with type coercion.
func compareEqual(fieldVal any, condValue string) (bool, error) {
	if fieldVal == nil {
		return condValue == "", nil
	}

	switch v := fieldVal.(type) {
	case string:
		return v == condValue, nil
	case bool:
		return strconv.FormatBool(v) == condValue, nil
	case float64:
		cf, err := strconv.ParseFloat(condValue, 64)
		if err != nil {
			return false, nil // type mismatch, not equal
		}
		return v == cf, nil
	case json.Number:
		cf, err := strconv.ParseFloat(condValue, 64)
		if err != nil {
			return false, nil
		}
		fv, err := v.Float64()
		if err != nil {
			return false, nil
		}
		return fv == cf, nil
	default:
		return fmt.Sprintf("%v", fieldVal) == condValue, nil
	}
}

// compareNumbers compares two values numerically with type coercion.
func compareNumbers(fieldVal any, condValue string, cmp func(float64, float64) bool) (bool, error) {
	a, err := toFloat64(fieldVal)
	if err != nil {
		return false, fmt.Errorf("field value is not a number: %w", err)
	}
	b, err := strconv.ParseFloat(condValue, 64)
	if err != nil {
		return false, fmt.Errorf("condition value %q is not a number: %w", condValue, err)
	}
	return cmp(a, b), nil
}

// compareContains checks if the field value contains the condition value as a substring.
func compareContains(fieldVal any, condValue string) (bool, error) {
	if fieldVal == nil {
		return false, nil
	}
	s, ok := fieldVal.(string)
	if !ok {
		s = fmt.Sprintf("%v", fieldVal)
	}
	return strings.Contains(s, condValue), nil
}

// matchRegex matches the field value against a regex pattern.
// Pattern length is limited to 256 characters for safety.
func matchRegex(fieldVal any, pattern string) (bool, error) {
	if len(pattern) > 256 {
		return false, fmt.Errorf("regex pattern exceeds 256 character limit (got %d)", len(pattern))
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, fmt.Errorf("invalid regex %q: %w", pattern, err)
	}
	if fieldVal == nil {
		return false, nil
	}
	s, ok := fieldVal.(string)
	if !ok {
		s = fmt.Sprintf("%v", fieldVal)
	}
	return re.MatchString(s), nil
}

// toFloat64 converts a value to float64 with type coercion.
func toFloat64(val any) (float64, error) {
	switch v := val.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case string:
		return strconv.ParseFloat(v, 64)
	case json.Number:
		return v.Float64()
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", val)
	}
}
