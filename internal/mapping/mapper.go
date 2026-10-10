package mapping

import (
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
)

// ValidationError describes a field-level validation problem encountered during mapping.
type ValidationError struct {
	Field  string `json:"field"`
	Code   string `json:"code"`   // PARSE_001, PARSE_002, PARSE_003, PARSE_004
	Detail string `json:"detail"`
}

func (ve ValidationError) Error() string {
	return fmt.Sprintf("%s: %s (%s)", ve.Field, ve.Detail, ve.Code)
}

// Apply evaluates every MappingEntry against the raw JSON payload and returns
// the extracted fields map, any validation errors, and an optional fatal error.
func Apply(mapping *Mapping, payloadRaw string) (map[string]any, []ValidationError, error) {
	fields := make(map[string]any, len(mapping.Fields))
	var validationErrors []ValidationError

	for i := range mapping.Fields {
		entry := &mapping.Fields[i]
		val, errs := applyEntry(entry, payloadRaw)
		if len(errs) > 0 {
			validationErrors = append(validationErrors, errs...)
		}
		if val != nil {
			fields[entry.TargetKey] = val
		}
	}

	return fields, validationErrors, nil
}

// applyEntry processes a single MappingEntry and returns the extracted value
// along with any validation errors.
func applyEntry(entry *MappingEntry, payloadRaw string) (any, []ValidationError) {
	var errs []ValidationError

	result := gjson.Get(payloadRaw, entry.SourcePath)

	// Step 1: Handle missing values
	if !result.Exists() {
		if entry.DefaultValue != nil {
			return typedValue(*entry.DefaultValue, entry.ValueType, entry.TargetKey, &errs)
		}
		if entry.Required {
			errs = append(errs, ValidationError{
				Field:  entry.TargetKey,
				Code:   "PARSE_002",
				Detail: fmt.Sprintf("required field %q not found at path %q", entry.TargetKey, entry.SourcePath),
			})
		}
		return nil, errs
	}

	// Step 2: Handle null values
	if result.Type == gjson.Null {
		if entry.DefaultValue != nil {
			return typedValue(*entry.DefaultValue, entry.ValueType, entry.TargetKey, &errs)
		}
		if entry.Required {
			errs = append(errs, ValidationError{
				Field:  entry.TargetKey,
				Code:   "PARSE_002",
				Detail: fmt.Sprintf("required field %q is null at path %q", entry.TargetKey, entry.SourcePath),
			})
		}
		return nil, errs
	}

	// Step 3: Apply multi_result_mode for array results
	var val any
	if result.IsArray() {
		val = applyMultiResultMode(entry, result, &errs)
	} else {
		val = result.Value()
	}

	// Step 4: Apply transform
	val = applyTransform(entry, val)

	// Step 5: Validate value_type
	if err := validateType(val, entry.ValueType); err != nil {
		errs = append(errs, ValidationError{
			Field:  entry.TargetKey,
			Code:   "PARSE_003",
			Detail: fmt.Sprintf("type mismatch for field %q: %s", entry.TargetKey, err.Error()),
		})
	}

	// Step 6: Validate required
	if entry.Required && isZero(val) {
		errs = append(errs, ValidationError{
			Field:  entry.TargetKey,
			Code:   "PARSE_002",
			Detail: fmt.Sprintf("required field %q has zero value", entry.TargetKey),
		})
	}

	return val, errs
}

// applyMultiResultMode handles array results according to the entry's MultiResultMode.
func applyMultiResultMode(entry *MappingEntry, result gjson.Result, errs *[]ValidationError) any {
	mode := ""
	if entry.MultiResultMode != nil {
		mode = *entry.MultiResultMode
	}

	switch {
	case mode == "first":
		arr := result.Array()
		if len(arr) == 0 {
			if entry.DefaultValue != nil {
				return *entry.DefaultValue
			}
			return nil
		}
		return arr[0].Value()

	case mode == "array":
		var items []any
		result.ForEach(func(_, v gjson.Result) bool {
			items = append(items, v.Value())
			return true
		})
		return items

	case strings.HasPrefix(mode, "join(") && strings.HasSuffix(mode, ")"):
		sep := mode[5 : len(mode)-1]
		var parts []string
		result.ForEach(func(_, v gjson.Result) bool {
			parts = append(parts, v.String())
			return true
		})
		return strings.Join(parts, sep)

	case mode == "count":
		return float64(len(result.Array()))

	default:
		// If no multi_result_mode specified, return first element by default
		// (or the whole array value if it's not configured)
		if mode == "" {
			// No mode specified: keep the array as-is for "array" value_type,
			// otherwise take first element
			if entry.ValueType == "array" {
				var items []any
				result.ForEach(func(_, v gjson.Result) bool {
					items = append(items, v.Value())
					return true
				})
				return items
			}
			arr := result.Array()
			if len(arr) == 0 {
				return nil
			}
			return arr[0].Value()
		}
		*errs = append(*errs, ValidationError{
			Field:  entry.TargetKey,
			Code:   "PARSE_004",
			Detail: fmt.Sprintf("unknown multi_result_mode %q for field %q", mode, entry.TargetKey),
		})
		return nil
	}
}

// applyTransform applies the configured transform to the value.
func applyTransform(entry *MappingEntry, val any) any {
	if entry.Transform == nil {
		return val
	}
	t := *entry.Transform

	str, ok := val.(string)
	if !ok {
		// Transforms only apply to string values
		return val
	}

	switch t {
	case "to_upper":
		return strings.ToUpper(str)
	case "to_lower":
		return strings.ToLower(str)
	case "trim":
		return strings.TrimSpace(str)
	default:
		return val
	}
}

// validateType checks whether the value matches the expected value_type.
func validateType(val any, valueType string) error {
	if valueType == "" {
		return nil
	}

	switch valueType {
	case "string":
		if _, ok := val.(string); !ok {
			return fmt.Errorf("expected string, got %T", val)
		}
	case "number":
		switch val.(type) {
		case float64, int, int64, float32, int32:
			// OK
		default:
			return fmt.Errorf("expected number, got %T", val)
		}
	case "boolean":
		if _, ok := val.(bool); !ok {
			return fmt.Errorf("expected boolean, got %T", val)
		}
	case "object":
		if _, ok := val.(map[string]any); !ok {
			return fmt.Errorf("expected object, got %T", val)
		}
	case "array":
		if _, ok := val.([]any); !ok {
			return fmt.Errorf("expected array, got %T", val)
		}
	}
	return nil
}

// typedValue converts a string default value into the expected value_type.
func typedValue(raw, valueType, field string, errs *[]ValidationError) (any, []ValidationError) {
	if valueType == "" || valueType == "string" {
		return raw, nil
	}

	switch valueType {
	case "number":
		// gjson would return float64 for numbers
		var f float64
		if _, err := fmt.Sscanf(raw, "%f", &f); err != nil {
			*errs = append(*errs, ValidationError{
				Field:  field,
				Code:   "PARSE_003",
				Detail: fmt.Sprintf("cannot convert default value %q to number", raw),
			})
			return raw, *errs
		}
		return f, *errs
	case "boolean":
		switch strings.ToLower(raw) {
		case "true", "1":
			return true, *errs
		case "false", "0":
			return false, *errs
		default:
			*errs = append(*errs, ValidationError{
				Field:  field,
				Code:   "PARSE_003",
				Detail: fmt.Sprintf("cannot convert default value %q to boolean", raw),
			})
			return raw, *errs
		}
	default:
		return raw, nil
	}
}

// isZero returns true if the value is the zero value for its type.
// Used for required-field checks: empty string, 0, false, nil are considered zero.
func isZero(val any) bool {
	if val == nil {
		return true
	}
	switch v := val.(type) {
	case string:
		return v == ""
	case bool:
		return !v
	case float64:
		return v == 0
	case int:
		return v == 0
	case int64:
		return v == 0
	case []any:
		return len(v) == 0
	case map[string]any:
		return len(v) == 0
	}
	return false
}
