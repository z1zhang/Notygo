package mapping

import (
	"testing"
)

func TestApply_BasicExtraction(t *testing.T) {
	m := &Mapping{
		Fields: []MappingEntry{
			{PayloadPath: "name", TargetKey: "name", ValueType: "string"},
			{PayloadPath: "age", TargetKey: "age", ValueType: "number"},
			{PayloadPath: "active", TargetKey: "active", ValueType: "boolean"},
		},
	}
	payload := `{"name":"Alice","age":30,"active":true}`

	fields, errs, err := Apply(m, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("unexpected validation errors: %v", errs)
	}
	if fields["name"] != "Alice" {
		t.Errorf("name = %v, want Alice", fields["name"])
	}
	if fields["age"] != float64(30) {
		t.Errorf("age = %v, want 30", fields["age"])
	}
	if fields["active"] != true {
		t.Errorf("active = %v, want true", fields["active"])
	}
}

func TestApply_NestedPath(t *testing.T) {
	m := &Mapping{
		Fields: []MappingEntry{
			{PayloadPath: "data.user.email", TargetKey: "email", ValueType: "string"},
			{PayloadPath: "data.items.0.name", TargetKey: "first_item", ValueType: "string"},
		},
	}
	payload := `{"data":{"user":{"email":"test@example.com"},"items":[{"name":"item1"},{"name":"item2"}]}}`

	fields, errs, err := Apply(m, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("unexpected validation errors: %v", errs)
	}
	if fields["email"] != "test@example.com" {
		t.Errorf("email = %v, want test@example.com", fields["email"])
	}
	if fields["first_item"] != "item1" {
		t.Errorf("first_item = %v, want item1", fields["first_item"])
	}
}

func TestApply_MissingField(t *testing.T) {
	defVal := "default"
	m := &Mapping{
		Fields: []MappingEntry{
			{PayloadPath: "missing", TargetKey: "missing_optional", ValueType: "string"},
			{PayloadPath: "missing_required", TargetKey: "missing_required", ValueType: "string", Required: true},
			{PayloadPath: "missing_with_default", TargetKey: "missing_default", ValueType: "string", DefaultValue: &defVal},
		},
	}
	payload := `{}`

	fields, errs, err := Apply(m, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Optional missing field: not in fields, no error
	if _, ok := fields["missing_optional"]; ok {
		t.Error("missing_optional should not be in fields")
	}

	// Required missing field: error
	found := false
	for _, e := range errs {
		if e.Field == "missing_required" && e.Code == "PARSE_002" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected PARSE_002 for missing_required, got errors: %v", errs)
	}

	// Missing with default: default value used
	if fields["missing_default"] != "default" {
		t.Errorf("missing_default = %v, want default", fields["missing_default"])
	}
}

func TestApply_NullValue(t *testing.T) {
	defVal := "fallback"
	m := &Mapping{
		Fields: []MappingEntry{
			{PayloadPath: "x", TargetKey: "null_optional", ValueType: "string"},
			{PayloadPath: "y", TargetKey: "null_required", ValueType: "string", Required: true},
			{PayloadPath: "z", TargetKey: "null_with_default", ValueType: "string", DefaultValue: &defVal},
		},
	}
	payload := `{"x":null,"y":null,"z":null}`

	fields, errs, err := Apply(m, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := fields["null_optional"]; ok {
		t.Error("null_optional should not be in fields")
	}

	found := false
	for _, e := range errs {
		if e.Field == "null_required" && e.Code == "PARSE_002" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected PARSE_002 for null_required, got: %v", errs)
	}

	if fields["null_with_default"] != "fallback" {
		t.Errorf("null_with_default = %v, want fallback", fields["null_with_default"])
	}
}

func TestApply_EmptyStringAndFalseAndZero(t *testing.T) {
	m := &Mapping{
		Fields: []MappingEntry{
			{PayloadPath: "empty_str", TargetKey: "empty_str", ValueType: "string", Required: true},
			{PayloadPath: "false_val", TargetKey: "false_val", ValueType: "boolean", Required: true},
			{PayloadPath: "zero_val", TargetKey: "zero_val", ValueType: "number", Required: true},
		},
	}
	payload := `{"empty_str":"","false_val":false,"zero_val":0}`

	fields, errs, err := Apply(m, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Empty string, false, and 0 are valid values that exist, but are "zero" values.
	// Required check flags them.
	if len(errs) != 3 {
		t.Errorf("expected 3 required errors, got %d: %v", len(errs), errs)
	}

	// Values should still be present in the fields map
	if fields["empty_str"] != "" {
		t.Errorf("empty_str = %v, want empty string", fields["empty_str"])
	}
	if fields["false_val"] != false {
		t.Errorf("false_val = %v, want false", fields["false_val"])
	}
	if fields["zero_val"] != float64(0) {
		t.Errorf("zero_val = %v, want 0", fields["zero_val"])
	}
}

func TestApply_DefaultValues(t *testing.T) {
	strDef := "hello"
	boolDef := "true"
	numDef := "42"

	m := &Mapping{
		Fields: []MappingEntry{
			{PayloadPath: "s", TargetKey: "str_default", ValueType: "string", DefaultValue: &strDef},
			{PayloadPath: "b", TargetKey: "bool_default", ValueType: "boolean", DefaultValue: &boolDef},
			{PayloadPath: "n", TargetKey: "num_default", ValueType: "number", DefaultValue: &numDef},
		},
	}
	payload := `{}`

	fields, errs, err := Apply(m, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("unexpected validation errors: %v", errs)
	}
	if fields["str_default"] != "hello" {
		t.Errorf("str_default = %v, want hello", fields["str_default"])
	}
	if fields["bool_default"] != true {
		t.Errorf("bool_default = %v, want true", fields["bool_default"])
	}
	if fields["num_default"] != float64(42) {
		t.Errorf("num_default = %v, want 42", fields["num_default"])
	}
}

func TestApply_TypeValidation(t *testing.T) {
	m := &Mapping{
		Fields: []MappingEntry{
			{PayloadPath: "not_a_number", TargetKey: "num_field", ValueType: "number"},
			{PayloadPath: "not_a_bool", TargetKey: "bool_field", ValueType: "boolean"},
			{PayloadPath: "not_an_array", TargetKey: "arr_field", ValueType: "array"},
			{PayloadPath: "not_an_object", TargetKey: "obj_field", ValueType: "object"},
		},
	}
	payload := `{"not_a_number":"abc","not_a_bool":"yes","not_an_array":1,"not_an_object":"str"}`

	_, errs, err := Apply(m, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	codes := map[string]bool{}
	for _, e := range errs {
		codes[e.Code] = true
	}
	if !codes["PARSE_003"] {
		t.Errorf("expected PARSE_003 type mismatch errors, got: %v", errs)
	}
	if len(errs) != 4 {
		t.Errorf("expected 4 type errors, got %d: %v", len(errs), errs)
	}
}

func TestApply_MultiResultMode_First(t *testing.T) {
	mode := "first"
	m := &Mapping{
		Fields: []MappingEntry{
			{PayloadPath: "items", TargetKey: "first_item", ValueType: "string", MultiResultMode: &mode},
		},
	}
	payload := `{"items":["alpha","beta","gamma"]}`

	fields, errs, err := Apply(m, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("unexpected validation errors: %v", errs)
	}
	if fields["first_item"] != "alpha" {
		t.Errorf("first_item = %v, want alpha", fields["first_item"])
	}
}

func TestApply_MultiResultMode_Array(t *testing.T) {
	mode := "array"
	m := &Mapping{
		Fields: []MappingEntry{
			{PayloadPath: "tags", TargetKey: "tag_list", ValueType: "array", MultiResultMode: &mode},
		},
	}
	payload := `{"tags":["go","rust","zig"]}`

	fields, errs, err := Apply(m, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("unexpected validation errors: %v", errs)
	}
	arr, ok := fields["tag_list"].([]any)
	if !ok {
		t.Fatalf("tag_list is not []any, got %T", fields["tag_list"])
	}
	if len(arr) != 3 {
		t.Errorf("tag_list length = %d, want 3", len(arr))
	}
	if arr[0] != "go" || arr[1] != "rust" || arr[2] != "zig" {
		t.Errorf("tag_list = %v, want [go rust zig]", arr)
	}
}

func TestApply_MultiResultMode_Join(t *testing.T) {
	mode := "join(, )"
	m := &Mapping{
		Fields: []MappingEntry{
			{PayloadPath: "names", TargetKey: "joined", ValueType: "string", MultiResultMode: &mode},
		},
	}
	payload := `{"names":["Alice","Bob","Carol"]}`

	fields, errs, err := Apply(m, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("unexpected validation errors: %v", errs)
	}
	if fields["joined"] != "Alice, Bob, Carol" {
		t.Errorf("joined = %v, want 'Alice, Bob, Carol'", fields["joined"])
	}
}

func TestApply_MultiResultMode_Count(t *testing.T) {
	mode := "count"
	m := &Mapping{
		Fields: []MappingEntry{
			{PayloadPath: "items", TargetKey: "item_count", ValueType: "number", MultiResultMode: &mode},
		},
	}
	payload := `{"items":[1,2,3,4,5]}`

	fields, errs, err := Apply(m, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("unexpected validation errors: %v", errs)
	}
	if fields["item_count"] != float64(5) {
		t.Errorf("item_count = %v, want 5", fields["item_count"])
	}
}

func TestApply_MultiResultMode_EmptyArray(t *testing.T) {
	mode := "first"
	defVal := "none"
	m := &Mapping{
		Fields: []MappingEntry{
			{PayloadPath: "items", TargetKey: "first_item", ValueType: "string", MultiResultMode: &mode, DefaultValue: &defVal},
		},
	}
	payload := `{"items":[]}`

	fields, errs, err := Apply(m, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("unexpected validation errors: %v", errs)
	}
	if fields["first_item"] != "none" {
		t.Errorf("first_item = %v, want none (default)", fields["first_item"])
	}
}

func TestApply_MultiResultMode_Unknown(t *testing.T) {
	mode := "invalid_mode"
	m := &Mapping{
		Fields: []MappingEntry{
			{PayloadPath: "items", TargetKey: "bad_mode", ValueType: "string", MultiResultMode: &mode},
		},
	}
	payload := `{"items":["a","b"]}`

	_, errs, err := Apply(m, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, e := range errs {
		if e.Code == "PARSE_004" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected PARSE_004 for unknown multi_result_mode, got: %v", errs)
	}
}

func TestApply_Transform_Upper(t *testing.T) {
	transform := "to_upper"
	m := &Mapping{
		Fields: []MappingEntry{
			{PayloadPath: "name", TargetKey: "name_upper", ValueType: "string", Transform: &transform},
		},
	}
	payload := `{"name":"alice"}`

	fields, errs, err := Apply(m, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("unexpected validation errors: %v", errs)
	}
	if fields["name_upper"] != "ALICE" {
		t.Errorf("name_upper = %v, want ALICE", fields["name_upper"])
	}
}

func TestApply_Transform_Lower(t *testing.T) {
	transform := "to_lower"
	m := &Mapping{
		Fields: []MappingEntry{
			{PayloadPath: "name", TargetKey: "name_lower", ValueType: "string", Transform: &transform},
		},
	}
	payload := `{"name":"ALICE"}`

	fields, errs, err := Apply(m, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("unexpected validation errors: %v", errs)
	}
	if fields["name_lower"] != "alice" {
		t.Errorf("name_lower = %v, want alice", fields["name_lower"])
	}
}

func TestApply_Transform_Trim(t *testing.T) {
	transform := "trim"
	m := &Mapping{
		Fields: []MappingEntry{
			{PayloadPath: "name", TargetKey: "name_trimmed", ValueType: "string", Transform: &transform},
		},
	}
	payload := `{"name":"  alice  "}`

	fields, errs, err := Apply(m, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("unexpected validation errors: %v", errs)
	}
	if fields["name_trimmed"] != "alice" {
		t.Errorf("name_trimmed = %v, want alice", fields["name_trimmed"])
	}
}

func TestApply_Transform_NonString_NoOp(t *testing.T) {
	transform := "to_upper"
	m := &Mapping{
		Fields: []MappingEntry{
			{PayloadPath: "count", TargetKey: "count", ValueType: "number", Transform: &transform},
		},
	}
	payload := `{"count":42}`

	fields, errs, err := Apply(m, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("unexpected validation errors: %v", errs)
	}
	if fields["count"] != float64(42) {
		t.Errorf("count = %v, want 42 (transform should be no-op on numbers)", fields["count"])
	}
}

func TestApply_ObjectExtraction(t *testing.T) {
	m := &Mapping{
		Fields: []MappingEntry{
			{PayloadPath: "metadata", TargetKey: "meta", ValueType: "object"},
		},
	}
	payload := `{"metadata":{"key":"value","nested":true}}`

	fields, errs, err := Apply(m, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("unexpected validation errors: %v", errs)
	}
	obj, ok := fields["meta"].(map[string]any)
	if !ok {
		t.Fatalf("meta is not map[string]any, got %T", fields["meta"])
	}
	if obj["key"] != "value" {
		t.Errorf("meta.key = %v, want value", obj["key"])
	}
}

func TestApply_NoValueType(t *testing.T) {
	m := &Mapping{
		Fields: []MappingEntry{
			{PayloadPath: "anything", TargetKey: "any_val"},
		},
	}
	payload := `{"anything":42}`

	fields, errs, err := Apply(m, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("unexpected validation errors: %v", errs)
	}
	if fields["any_val"] != float64(42) {
		t.Errorf("any_val = %v, want 42", fields["any_val"])
	}
}

func TestApply_DefaultValueBooleanConversions(t *testing.T) {
	boolFalse := "false"
	boolZero := "0"
	m := &Mapping{
		Fields: []MappingEntry{
			{PayloadPath: "missing1", TargetKey: "b1", ValueType: "boolean", DefaultValue: &boolFalse},
			{PayloadPath: "missing2", TargetKey: "b2", ValueType: "boolean", DefaultValue: &boolZero},
		},
	}
	payload := `{}`

	fields, errs, err := Apply(m, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("unexpected validation errors: %v", errs)
	}
	if fields["b1"] != false {
		t.Errorf("b1 = %v, want false", fields["b1"])
	}
	if fields["b2"] != false {
		t.Errorf("b2 = %v, want false", fields["b2"])
	}
}

func TestApply_DefaultValueInvalidNumber(t *testing.T) {
	invalidNum := "not_a_number"
	m := &Mapping{
		Fields: []MappingEntry{
			{PayloadPath: "missing", TargetKey: "n", ValueType: "number", DefaultValue: &invalidNum},
		},
	}
	payload := `{}`

	_, errs, err := Apply(m, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, e := range errs {
		if e.Code == "PARSE_003" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected PARSE_003 for invalid number default, got: %v", errs)
	}
}

func TestApply_MultiResultMode_ArrayOfBooleans(t *testing.T) {
	// Regression: #.image.tag.semver returns [false, false, false, true] from Diun
	// When multi_result_mode is "array", value_type must be "array" (not "boolean")
	arrMode := "array"
	m := &Mapping{
		Fields: []MappingEntry{
			{PayloadPath: "#.tag.semver", TargetKey: "tag_semvers", ValueType: "array", MultiResultMode: &arrMode},
		},
	}
	payload := `[{"tag":{"semver":false}},{"tag":{"semver":false}},{"tag":{"semver":true}}]`

	fields, errs, err := Apply(m, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("unexpected validation errors: %v", errs)
	}
	arr, ok := fields["tag_semvers"].([]any)
	if !ok {
		t.Fatalf("tag_semvers is not []any, got %T", fields["tag_semvers"])
	}
	if len(arr) != 3 {
		t.Errorf("tag_semvers length = %d, want 3", len(arr))
	}
}

func TestApply_MultiResultMode_JoinArrayOfBooleans(t *testing.T) {
	// join(, ) on boolean elements produces string output, so value_type must be "string"
	joinMode := "join(, )"
	m := &Mapping{
		Fields: []MappingEntry{
			{PayloadPath: "#.tag.semver", TargetKey: "tag_semvers_joined", ValueType: "string", MultiResultMode: &joinMode},
		},
	}
	payload := `[{"tag":{"semver":false}},{"tag":{"semver":true}}]`

	fields, errs, err := Apply(m, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("unexpected validation errors: %v", errs)
	}
	if fields["tag_semvers_joined"] != "false, true" {
		t.Errorf("tag_semvers_joined = %v, want 'false, true'", fields["tag_semvers_joined"])
	}
}

func TestApply_MultiResultMode_FirstArrayOfBooleans(t *testing.T) {
	// "first" mode extracts one boolean element, so value_type "boolean" is correct
	firstMode := "first"
	m := &Mapping{
		Fields: []MappingEntry{
			{PayloadPath: "#.tag.semver", TargetKey: "first_semver", ValueType: "boolean", MultiResultMode: &firstMode},
		},
	}
	payload := `[{"tag":{"semver":false}},{"tag":{"semver":true}}]`

	fields, errs, err := Apply(m, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("unexpected validation errors: %v", errs)
	}
	if fields["first_semver"] != false {
		t.Errorf("first_semver = %v, want false", fields["first_semver"])
	}
}

func TestApply_HashPath_WholeArray(t *testing.T) {
	// Regression: gjson "#" returns array count (a number), not the array itself.
	// We must use gjson.Parse for bare "#" to get the root array.
	arrMode := "array"
	m := &Mapping{
		Fields: []MappingEntry{
			{PayloadPath: "#", TargetKey: "items", ValueType: "array", MultiResultMode: &arrMode},
		},
	}
	payload := `[{"name":"dbx"},{"name":"redis"},{"name":"mysql"},{"name":"piclist"}]`

	fields, errs, err := Apply(m, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("unexpected validation errors: %v", errs)
	}
	arr, ok := fields["items"].([]any)
	if !ok {
		t.Fatalf("items is not []any, got %T", fields["items"])
	}
	if len(arr) != 4 {
		t.Errorf("items length = %d, want 4", len(arr))
	}
}

func TestApply_HashPath_Count(t *testing.T) {
	// "#" with count mode should still return the array length
	countMode := "count"
	m := &Mapping{
		Fields: []MappingEntry{
			{PayloadPath: "#", TargetKey: "item_count", ValueType: "number", MultiResultMode: &countMode},
		},
	}
	payload := `[{"name":"dbx"},{"name":"redis"},{"name":"mysql"}]`

	fields, errs, err := Apply(m, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("unexpected validation errors: %v", errs)
	}
	if fields["item_count"] != float64(3) {
		t.Errorf("item_count = %v, want 3", fields["item_count"])
	}
}

func TestApply_HashPath_First(t *testing.T) {
	// "#" with first mode should return the first element
	firstMode := "first"
	m := &Mapping{
		Fields: []MappingEntry{
			{PayloadPath: "#", TargetKey: "first_item", ValueType: "object", MultiResultMode: &firstMode},
		},
	}
	payload := `[{"name":"dbx"},{"name":"redis"}]`

	fields, errs, err := Apply(m, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("unexpected validation errors: %v", errs)
	}
	obj, ok := fields["first_item"].(map[string]any)
	if !ok {
		t.Fatalf("first_item is not map[string]any, got %T", fields["first_item"])
	}
	if obj["name"] != "dbx" {
		t.Errorf("first_item.name = %v, want dbx", obj["name"])
	}
}
