package rule

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// ConditionItem can hold either a Condition or a nested ConditionGroup.
// It is stored as json.RawMessage and decoded on demand.
type ConditionItem json.RawMessage

// ValidOperators is the single source of truth for operators accepted by
// evaluateCondition. API-level validation reuses this set so the two can
// never drift apart.
var ValidOperators = map[string]bool{
	"eq": true, "ne": true, "neq": true,
	"contains": true, "not_contains": true, "regex": true,
	"gt": true, "lt": true, "gte": true, "lte": true,
	"exists": true, "not_exists": true,
	"empty": true, "not_empty": true,
}

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

	case "empty":
		return isEmptyValue(fieldVal), nil

	case "not_empty":
		return !isEmptyValue(fieldVal), nil

	default:
		return false, fmt.Errorf("unknown operator %q", cond.Operator)
	}
}

// isEmptyValue reports whether a resolved field value is "empty":
// missing, an empty (or whitespace-only) string, an empty array, or an empty object.
// Zero numbers and false booleans are values, not emptiness.
func isEmptyValue(val any) bool {
	switch v := val.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(v) == ""
	case []any:
		return len(v) == 0
	case map[string]any:
		return len(v) == 0
	default:
		return false
	}
}

// resolveFieldPath resolves a "fields.X" or dotted path from the fields map.
// If the path starts with "fields.", it strips that prefix.
func resolveFieldPath(path string, fields map[string]any) any {
	cleanPath := strings.TrimPrefix(path, "fields.")

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

// StringifyValue converts a field value to its string form for comparison
// and logging. Numbers are formatted without scientific notation (e.g.
// 1000000000, not 1e+09) so string operators match user-provided values;
// bools render as true/false. Exposed for reuse by the processor's logging.
func StringifyValue(val any) string {
	switch v := val.(type) {
	case float64:
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%f", v)
	case float32:
		if v == float32(int32(v)) {
			return fmt.Sprintf("%d", int32(v))
		}
		return fmt.Sprintf("%f", v)
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case int32:
		return fmt.Sprintf("%d", v)
	case bool:
		return strconv.FormatBool(v)
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
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
		return StringifyValue(fieldVal) == condValue, nil
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
		s = StringifyValue(fieldVal)
	}
	return strings.Contains(s, condValue), nil
}

// regexCache caches compiled patterns so regex conditions don't recompile on
// every evaluation (each event × each condition). Bounded in practice by the
// number of distinct regex conditions stored in the rules table (256 chars
// each); invalid patterns are never cached.
var regexCache sync.Map // string -> *regexp.Regexp

// matchRegex matches the field value against a regex pattern.
// Pattern length is limited to 256 characters for safety.
func matchRegex(fieldVal any, pattern string) (bool, error) {
	if len(pattern) > 256 {
		return false, fmt.Errorf("regex pattern exceeds 256 character limit (got %d)", len(pattern))
	}
	var re *regexp.Regexp
	if cached, ok := regexCache.Load(pattern); ok {
		re = cached.(*regexp.Regexp)
	} else {
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return false, fmt.Errorf("invalid regex %q: %w", pattern, err)
		}
		regexCache.Store(pattern, compiled)
		re = compiled
	}
	if fieldVal == nil {
		return false, nil
	}
	s, ok := fieldVal.(string)
	if !ok {
		s = StringifyValue(fieldVal)
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
