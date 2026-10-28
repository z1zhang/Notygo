package rule

import (
	"encoding/json"
	"testing"
)

func TestEvaluate_Eq(t *testing.T) {
	rule := &Rule{
		Enabled: true,
		Conditions: ConditionGroup{
			Operator: "and",
			Conditions: []Condition{
				{Field: "status", Operator: "eq", Value: "ok"},
			},
		},
	}
	fields := map[string]any{"status": "ok"}

	result, err := Evaluate(rule, fields)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected rule to match")
	}
}

func TestEvaluate_Eq_Number(t *testing.T) {
	rule := &Rule{
		Enabled: true,
		Conditions: ConditionGroup{
			Operator: "and",
			Conditions: []Condition{
				{Field: "count", Operator: "eq", Value: "5"},
			},
		},
	}
	fields := map[string]any{"count": float64(5)}

	result, err := Evaluate(rule, fields)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected rule to match for number equality")
	}
}

func TestEvaluate_Eq_Bool(t *testing.T) {
	rule := &Rule{
		Enabled: true,
		Conditions: ConditionGroup{
			Operator: "and",
			Conditions: []Condition{
				{Field: "active", Operator: "eq", Value: "true"},
			},
		},
	}
	fields := map[string]any{"active": true}

	result, err := Evaluate(rule, fields)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected rule to match for boolean equality")
	}
}

func TestEvaluate_Ne(t *testing.T) {
	rule := &Rule{
		Enabled: true,
		Conditions: ConditionGroup{
			Operator: "and",
			Conditions: []Condition{
				{Field: "status", Operator: "ne", Value: "error"},
			},
		},
	}
	fields := map[string]any{"status": "ok"}

	result, err := Evaluate(rule, fields)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected ne condition to match")
	}
}

func TestEvaluate_Gt(t *testing.T) {
	rule := &Rule{
		Enabled: true,
		Conditions: ConditionGroup{
			Operator: "and",
			Conditions: []Condition{
				{Field: "count", Operator: "gt", Value: "10"},
			},
		},
	}

	result, err := Evaluate(rule, map[string]any{"count": float64(15)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected 15 > 10")
	}

	result, err = Evaluate(rule, map[string]any{"count": float64(5)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected 5 > 10 to be false")
	}
}

func TestEvaluate_Lt(t *testing.T) {
	rule := &Rule{
		Enabled: true,
		Conditions: ConditionGroup{
			Operator: "and",
			Conditions: []Condition{
				{Field: "count", Operator: "lt", Value: "10"},
			},
		},
	}

	result, err := Evaluate(rule, map[string]any{"count": float64(5)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected 5 < 10")
	}
}

func TestEvaluate_Gte_Lte(t *testing.T) {
	rule := &Rule{
		Enabled: true,
		Conditions: ConditionGroup{
			Operator: "and",
			Conditions: []Condition{
				{Field: "count", Operator: "gte", Value: "10"},
				{Field: "count", Operator: "lte", Value: "20"},
			},
		},
	}

	result, err := Evaluate(rule, map[string]any{"count": float64(10)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected 10 >= 10 and 10 <= 20")
	}

	result, err = Evaluate(rule, map[string]any{"count": float64(25)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected 25 to fail lte 20")
	}
}

func TestEvaluate_Contains(t *testing.T) {
	rule := &Rule{
		Enabled: true,
		Conditions: ConditionGroup{
			Operator: "and",
			Conditions: []Condition{
				{Field: "message", Operator: "contains", Value: "error"},
			},
		},
	}

	result, err := Evaluate(rule, map[string]any{"message": "an error occurred"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected contains to match")
	}

	result, err = Evaluate(rule, map[string]any{"message": "all is well"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected contains to not match")
	}
}

func TestEvaluate_NotContains(t *testing.T) {
	rule := &Rule{
		Enabled: true,
		Conditions: ConditionGroup{
			Operator: "and",
			Conditions: []Condition{
				{Field: "message", Operator: "not_contains", Value: "error"},
			},
		},
	}

	result, err := Evaluate(rule, map[string]any{"message": "all is well"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected not_contains to match")
	}
}

func TestEvaluate_Regex(t *testing.T) {
	rule := &Rule{
		Enabled: true,
		Conditions: ConditionGroup{
			Operator: "and",
			Conditions: []Condition{
				{Field: "email", Operator: "regex", Value: `^[\w.]+@[\w]+\.[\w]+$`},
			},
		},
	}

	result, err := Evaluate(rule, map[string]any{"email": "user@example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected regex to match valid email")
	}

	result, err = Evaluate(rule, map[string]any{"email": "not-an-email"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected regex to not match invalid email")
	}
}

func TestEvaluate_Regex_TooLong(t *testing.T) {
	longPattern := ""
	for range 257 {
		longPattern += "a"
	}
	rule := &Rule{
		Enabled: true,
		Conditions: ConditionGroup{
			Operator: "and",
			Conditions: []Condition{
				{Field: "x", Operator: "regex", Value: longPattern},
			},
		},
	}
	fields := map[string]any{"x": "aaa"}

	_, err := Evaluate(rule, fields)
	if err == nil {
		t.Error("expected error for too-long regex pattern")
	}
}

func TestEvaluate_Exists(t *testing.T) {
	rule := &Rule{
		Enabled: true,
		Conditions: ConditionGroup{
			Operator: "and",
			Conditions: []Condition{
				{Field: "name", Operator: "exists", Value: ""},
			},
		},
	}

	result, err := Evaluate(rule, map[string]any{"name": "Alice"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected exists to be true")
	}

	result, err = Evaluate(rule, map[string]any{"other": "value"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected exists to be false for missing field")
	}
}

func TestEvaluate_NotExists(t *testing.T) {
	rule := &Rule{
		Enabled: true,
		Conditions: ConditionGroup{
			Operator: "and",
			Conditions: []Condition{
				{Field: "name", Operator: "not_exists", Value: ""},
			},
		},
	}

	result, err := Evaluate(rule, map[string]any{"other": "value"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected not_exists to be true for missing field")
	}
}

func TestEvaluate_NotEmpty(t *testing.T) {
	rule := &Rule{
		Enabled: true,
		Conditions: ConditionGroup{
			Operator: "and",
			Conditions: []Condition{
				{Field: "name", Operator: "not_empty", Value: ""},
			},
		},
	}

	cases := []struct {
		name   string
		fields map[string]any
		want   bool
	}{
		{"present non-empty string", map[string]any{"name": "Alice"}, true},
		{"missing field", map[string]any{"other": "value"}, false},
		{"empty string", map[string]any{"name": ""}, false},
		{"whitespace-only string", map[string]any{"name": "  "}, false},
		{"zero number is a value", map[string]any{"name": float64(0)}, true},
		{"empty array", map[string]any{"name": []any{}}, false},
		{"non-empty array", map[string]any{"name": []any{"x"}}, true},
	}

	for _, tc := range cases {
		result, err := Evaluate(rule, tc.fields)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.name, err)
		}
		if result != tc.want {
			t.Errorf("%s: expected %v, got %v", tc.name, tc.want, result)
		}
	}
}

func TestEvaluate_Empty(t *testing.T) {
	rule := &Rule{
		Enabled: true,
		Conditions: ConditionGroup{
			Operator: "and",
			Conditions: []Condition{
				{Field: "name", Operator: "empty", Value: ""},
			},
		},
	}

	result, err := Evaluate(rule, map[string]any{"name": ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected empty to be true for empty string")
	}

	result, err = Evaluate(rule, map[string]any{"name": "Alice"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected empty to be false for non-empty string")
	}
}

func TestEvaluate_Contains_LargeNumber(t *testing.T) {
	// gjson parses payload numbers as float64; a naive %v renders 1e+09 and
	// breaks substring/regex matching against user values like "1000000000".
	rule := &Rule{
		Enabled: true,
		Conditions: ConditionGroup{
			Operator: "and",
			Conditions: []Condition{
				{Field: "size", Operator: "contains", Value: "1000000000"},
			},
		},
	}

	result, err := Evaluate(rule, map[string]any{"size": float64(1000000000)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected contains to match large integer without scientific notation")
	}

	if s := StringifyValue(float64(1000000000)); s != "1000000000" {
		t.Errorf("StringifyValue(1e9) = %q, want %q", s, "1000000000")
	}
}

func TestEvaluate_AndGroup(t *testing.T) {
	rule := &Rule{
		Enabled: true,
		Conditions: ConditionGroup{
			Operator: "and",
			Conditions: []Condition{
				{Field: "status", Operator: "eq", Value: "ok"},
				{Field: "count", Operator: "gt", Value: "0"},
			},
		},
	}

	result, err := Evaluate(rule, map[string]any{"status": "ok", "count": float64(5)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected AND group to match when all conditions true")
	}

	result, err = Evaluate(rule, map[string]any{"status": "ok", "count": float64(0)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected AND group to not match when one condition false")
	}
}

func TestEvaluate_OrGroup(t *testing.T) {
	rule := &Rule{
		Enabled: true,
		Conditions: ConditionGroup{
			Operator: "or",
			Conditions: []Condition{
				{Field: "status", Operator: "eq", Value: "ok"},
				{Field: "status", Operator: "eq", Value: "warning"},
			},
		},
	}

	result, err := Evaluate(rule, map[string]any{"status": "ok"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected OR group to match when one condition true")
	}

	result, err = Evaluate(rule, map[string]any{"status": "warning"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected OR group to match for warning")
	}

	result, err = Evaluate(rule, map[string]any{"status": "error"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected OR group to not match when no condition true")
	}
}

func TestEvaluate_NestedGroups(t *testing.T) {
	rule := &Rule{
		Enabled: true,
		Conditions: ConditionGroup{
			Operator: "and",
			Conditions: []Condition{
				{Field: "type", Operator: "eq", Value: "alert"},
			},
			Groups: []ConditionGroup{
				{
					Operator: "or",
					Conditions: []Condition{
						{Field: "severity", Operator: "eq", Value: "critical"},
						{Field: "severity", Operator: "eq", Value: "high"},
					},
				},
			},
		},
	}

	result, err := Evaluate(rule, map[string]any{"type": "alert", "severity": "critical"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected nested group to match: type=alert AND (severity=critical OR severity=high)")
	}

	result, err = Evaluate(rule, map[string]any{"type": "alert", "severity": "low"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected nested group to not match: type=alert AND severity=low fails OR")
	}
}

func TestEvaluate_DisabledRule(t *testing.T) {
	rule := &Rule{
		Enabled: false,
		Conditions: ConditionGroup{
			Operator: "and",
			Conditions: []Condition{
				{Field: "x", Operator: "eq", Value: "y"},
			},
		},
	}

	result, err := Evaluate(rule, map[string]any{"x": "y"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("disabled rule should not match")
	}
}

func TestEvaluate_FieldsPrefix(t *testing.T) {
	rule := &Rule{
		Enabled: true,
		Conditions: ConditionGroup{
			Operator: "and",
			Conditions: []Condition{
				{Field: "fields.container_name", Operator: "eq", Value: "web"},
			},
		},
	}
	fields := map[string]any{"container_name": "web"}

	result, err := Evaluate(rule, fields)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected fields.container_name to resolve to container_name")
	}
}

func TestEvaluate_DotNotationPath(t *testing.T) {
	rule := &Rule{
		Enabled: true,
		Conditions: ConditionGroup{
			Operator: "and",
			Conditions: []Condition{
				{Field: "server.host", Operator: "eq", Value: "localhost"},
			},
		},
	}
	fields := map[string]any{
		"server": map[string]any{"host": "localhost"},
	}

	result, err := Evaluate(rule, fields)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected dot notation path to resolve nested object")
	}
}

func TestEvaluate_NilFieldValue(t *testing.T) {
	rule := &Rule{
		Enabled: true,
		Conditions: ConditionGroup{
			Operator: "and",
			Conditions: []Condition{
				{Field: "missing", Operator: "eq", Value: ""},
			},
		},
	}
	fields := map[string]any{}

	result, err := Evaluate(rule, fields)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("nil field compared to empty string should be true")
	}
}

func TestEvaluate_UnknownOperator(t *testing.T) {
	rule := &Rule{
		Enabled: true,
		Conditions: ConditionGroup{
			Operator: "and",
			Conditions: []Condition{
				{Field: "x", Operator: "unknown_op", Value: "y"},
			},
		},
	}

	_, err := Evaluate(rule, map[string]any{"x": "y"})
	if err == nil {
		t.Error("expected error for unknown operator")
	}
}

func TestEvaluate_EmptyGroup(t *testing.T) {
	rule := &Rule{
		Enabled: true,
		Conditions: ConditionGroup{
			Operator: "and",
		},
	}

	result, err := Evaluate(rule, map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("empty condition group should evaluate to true")
	}
}

// FuzzEvaluate exercises the rule engine against arbitrary field maps and
// condition values across every operator. The contract: no panic; the
// outcome is a (result, error) pair, errors being legitimate (bad regex,
// non-numeric comparison).
func FuzzEvaluate(f *testing.F) {
	f.Add(`{"a":"x"}`, "a", "x")
	f.Add(`{"n":1000000000}`, "n", "1000000000")
	f.Add(`{"deep":{"x":[{"y":"z"}]}}`, "deep.x.0.y", "z")
	f.Add(`{"nested":{"flag":true}}`, "nested.flag", "false")
	f.Add(`{"empty":""}`, "empty", "")

	f.Fuzz(func(t *testing.T, payloadJSON, field, value string) {
		var fields map[string]any
		if json.Unmarshal([]byte(payloadJSON), &fields) != nil {
			return // gjson-derived maps always decode from an object
		}
		ops := []string{"eq", "ne", "neq", "contains", "not_contains", "regex",
			"gt", "lt", "gte", "lte", "exists", "not_exists", "empty", "not_empty"}
		for _, op := range ops {
			r := &Rule{Enabled: true, Conditions: ConditionGroup{
				Operator:   "and",
				Conditions: []Condition{{Field: field, Operator: op, Value: value}},
				Groups: []ConditionGroup{{
					Operator:   "or",
					Conditions: []Condition{{Field: field, Operator: op, Value: value}},
				}},
			}}
			_, _ = Evaluate(r, fields)
		}
		// Fuzzing generates unbounded distinct regex patterns; the compile
		// cache exists for the (finite) rules table, so drain it here to
		// keep long fuzz runs flat on memory. Fuzz workers are separate
		// processes and iterations are sequential, so this is race-free.
		regexCache.Range(func(k, v any) bool { regexCache.Delete(k); return true })
	})
}
