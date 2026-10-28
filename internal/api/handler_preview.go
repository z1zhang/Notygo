package api

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"notygo/internal/errcode"
	"strings"

	"github.com/tidwall/gjson"

	"github.com/jackc/pgx/v5/pgxpool"
	"notygo/internal/mapping"
	"notygo/internal/template"
)

// ---------------------------------------------------------------------------
// PreviewParse — POST /api/v1/preview/parse
// ---------------------------------------------------------------------------

// previewParseRequest is the request body for the parse preview endpoint.
type previewParseRequest struct {
	Payload string `json:"payload"`
}

// previewParseResponse is the response for the parse preview endpoint.
type previewParseResponse struct {
	Structure       structureInfo    `json:"structure"`
	SuggestedFields []suggestedField `json:"suggested_fields"`
}

type structureInfo struct {
	Type        string `json:"type"` // "array" or "object"
	RootIsArray bool   `json:"root_is_array"`
	ItemCount   int    `json:"item_count"` // array length or object key count
}

type suggestedField struct {
	PayloadPath     string `json:"payload_path"`
	TargetKey       string `json:"target_key"`
	ValueType       string `json:"value_type"`
	MultiResultMode string `json:"multi_result_mode,omitempty"`
	SampleValue     string `json:"sample_value"`
}

// PreviewParse analyzes a raw JSON payload and returns suggested mapping fields.
func PreviewParse() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req previewParseRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, http.StatusBadRequest, errcode.ConfigValidation, "invalid request body")
			return
		}
		if req.Payload == "" {
			WriteError(w, http.StatusBadRequest, errcode.ConfigValidation, "payload is required")
			return
		}

		if !gjson.Valid(req.Payload) {
			WriteError(w, http.StatusBadRequest, errcode.ConfigValidation, "payload is not valid JSON")
			return
		}

		root := gjson.Parse(req.Payload)
		resp := previewParseResponse{}

		if root.IsArray() {
			arr := root.Array()
			resp.Structure = structureInfo{
				Type:        "array",
				RootIsArray: true,
				ItemCount:   len(arr),
			}
			resp.SuggestedFields = analyzeArrayPayload(req.Payload, arr)
		} else if root.IsObject() {
			resp.Structure = structureInfo{
				Type:        "object",
				RootIsArray: false,
				ItemCount:   len(root.Map()),
			}
			resp.SuggestedFields = analyzeObjectPayload(req.Payload, root, "", 1)
		} else {
			WriteError(w, http.StatusBadRequest, errcode.ConfigValidation, "payload root must be a JSON object or array")
			return
		}

		WriteJSON(w, http.StatusOK, resp)
	}
}

// analyzeArrayPayload analyzes a top-level JSON array and returns suggested fields.
func analyzeArrayPayload(payload string, arr []gjson.Result) []suggestedField {
	var fields []suggestedField

	// Suggest # with multi_result_mode "array" (whole array of objects)
	fields = append(fields, suggestedField{
		PayloadPath:     "#",
		TargetKey:       "items",
		ValueType:       "array",
		MultiResultMode: "array",
		SampleValue:     fmt.Sprintf("[%d items]", len(arr)),
	})

	// Suggest # with multi_result_mode "count"
	fields = append(fields, suggestedField{
		PayloadPath:     "#",
		TargetKey:       "item_count",
		ValueType:       "number",
		MultiResultMode: "count",
		SampleValue:     fmt.Sprintf("%d", len(arr)),
	})

	// If first element is an object, walk its keys to suggest #.field paths
	if len(arr) > 0 && arr[0].IsObject() {
		firstObj := arr[0]
		walkObjectForArray(payload, firstObj, "#", 1, &fields)
	}

	return fields
}

// walkObjectForArray recursively walks an object (from the first array element)
// to suggest #.field, #.nested.field, etc. paths.
func walkObjectForArray(payload string, obj gjson.Result, pathPrefix string, depth int, fields *[]suggestedField) {
	if depth > 3 {
		return
	}
	for key, val := range obj.Map() {
		fieldPath := pathPrefix + "." + key

		if val.IsObject() {
			// Recurse into nested objects
			walkObjectForArray(payload, val, fieldPath, depth+1, fields)
			continue
		}

		if val.IsArray() {
			// For array values inside array elements, suggest array mode
			innerArr := val.Array()
			sample := fmt.Sprintf("[%d items]", len(innerArr))
			*fields = append(*fields, suggestedField{
				PayloadPath:     fieldPath,
				TargetKey:       autoTargetKey(fieldPath),
				ValueType:       "array",
				MultiResultMode: "array",
				SampleValue:     sample,
			})
			continue
		}

		// Scalar value: determine type and multi_result_mode
		vt := inferType(val)
		mrm := ""
		sample := ""

		// Get a sample by running gjson.Get on the full payload with the #.field path
		result := gjson.Get(payload, fieldPath)
		if result.Exists() {
			if result.IsArray() {
				// #.field returned an array — multi_result_mode determines the output type
				arr := result.Array()
				switch vt {
				case "string":
					mrm = "join(, )"
					// join always produces string, so vt stays "string"
					var parts []string
					for _, item := range arr {
						parts = append(parts, item.String())
					}
					sample = strings.Join(parts, ", ")
				default:
					mrm = "array"
					// array mode always produces []any, so override vt to "array"
					vt = "array"
					sample = fmt.Sprintf("[%d items]", len(arr))
				}
			} else {
				sample = formatSampleValue(result)
			}
		}

		if sample == "" {
			sample = formatSampleValue(val)
		}

		*fields = append(*fields, suggestedField{
			PayloadPath:     fieldPath,
			TargetKey:       autoTargetKey(fieldPath),
			ValueType:       vt,
			MultiResultMode: mrm,
			SampleValue:     sample,
		})
	}
}

// analyzeObjectPayload walks a top-level JSON object and returns suggested fields.
func analyzeObjectPayload(payload string, obj gjson.Result, pathPrefix string, depth int) []suggestedField {
	if depth > 3 {
		return nil
	}
	var fields []suggestedField

	for key, val := range obj.Map() {
		fieldPath := key
		if pathPrefix != "" {
			fieldPath = pathPrefix + "." + key
		}

		if val.IsObject() {
			// Recurse into nested objects
			nested := analyzeObjectPayload(payload, val, fieldPath, depth+1)
			fields = append(fields, nested...)
			continue
		}

		if val.IsArray() {
			arr := val.Array()
			vt := "array"
			mrm := "array"
			sample := fmt.Sprintf("[%d items]", len(arr))
			// If array of scalars, suggest join instead (produces string)
			if len(arr) > 0 && !arr[0].IsObject() && !arr[0].IsArray() {
				vt = "string" // join always produces string
				mrm = "join(, )"
				var parts []string
				for _, item := range arr {
					parts = append(parts, item.String())
				}
				sample = strings.Join(parts, ", ")
			}
			fields = append(fields, suggestedField{
				PayloadPath:     fieldPath,
				TargetKey:       autoTargetKey(fieldPath),
				ValueType:       vt,
				MultiResultMode: mrm,
				SampleValue:     sample,
			})
			continue
		}

		vt := inferType(val)
		fields = append(fields, suggestedField{
			PayloadPath: fieldPath,
			TargetKey:   autoTargetKey(fieldPath),
			ValueType:   vt,
			SampleValue: formatSampleValue(val),
		})
	}

	return fields
}

// inferType maps a gjson result to a value_type string.
func inferType(r gjson.Result) string {
	switch r.Type {
	case gjson.String:
		return "string"
	case gjson.Number:
		return "number"
	case gjson.True, gjson.False:
		return "boolean"
	case gjson.Null:
		return "string"
	case gjson.JSON:
		if r.IsArray() {
			return "array"
		}
		return "object"
	default:
		return "string"
	}
}

// formatSampleValue returns a human-readable sample string for a gjson value.
func formatSampleValue(r gjson.Result) string {
	switch r.Type {
	case gjson.String:
		return r.String()
	case gjson.Number:
		return r.Raw
	case gjson.True:
		return "true"
	case gjson.False:
		return "false"
	case gjson.Null:
		return "null"
	default:
		return r.Raw
	}
}

// autoTargetKey generates a target_key from a endpoint path.
// Examples: "#.image.name" → "image_names", "#.name" → "names", "name" → "name",
// "#.count" → "count", "host.name" → "host_name"
func autoTargetKey(payloadPath string) string {
	// Strip leading "#"
	s := strings.TrimPrefix(payloadPath, "#")
	s = strings.TrimPrefix(s, ".")

	if s == "" {
		return "value"
	}

	parts := strings.Split(s, ".")
	if len(parts) == 1 {
		return parts[0]
	}

	// For paths like "image.name", produce "image_name" (last segment is the leaf)
	// For "#.image.name" → "image_names" (pluralize the leaf since it's multi-result)
	leaf := parts[len(parts)-1]

	// Simple pluralization
	if strings.HasSuffix(leaf, "s") || strings.HasSuffix(leaf, "x") || strings.HasSuffix(leaf, "ch") || strings.HasSuffix(leaf, "sh") {
		leaf = leaf + "es"
	} else if strings.HasSuffix(leaf, "y") && len(leaf) > 1 && !isVowel(rune(leaf[len(leaf)-2])) {
		leaf = leaf[:len(leaf)-1] + "ies"
	} else {
		leaf = leaf + "s"
	}

	// If the path has a parent, prepend it with underscore for context
	if len(parts) > 1 && payloadPath != "" && payloadPath[0] == '#' {
		// Multi-result: include parent for disambiguation
		parent := parts[len(parts)-2]
		return parent + "_" + leaf
	}

	return leaf
}

func isVowel(c rune) bool {
	return c == 'a' || c == 'e' || c == 'i' || c == 'o' || c == 'u'
}

// ---------------------------------------------------------------------------
// PreviewMapping — POST /api/v1/preview/mapping
// ---------------------------------------------------------------------------

// previewMappingRequest is the request body for the mapping preview endpoint.
type previewMappingRequest struct {
	Payload string                 `json:"payload"`
	Fields  []mapping.MappingEntry `json:"fields"`
}

// PreviewMapping applies a mapping to a raw payload and returns the extracted fields.
func PreviewMapping() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req previewMappingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, http.StatusBadRequest, errcode.ConfigValidation, "invalid request body")
			return
		}
		if req.Payload == "" {
			WriteError(w, http.StatusBadRequest, errcode.ConfigValidation, "payload is required")
			return
		}

		// Build a temporary Mapping from the request fields
		m := &mapping.Mapping{
			Fields: req.Fields,
		}

		parsedFields, validationErrors, err := mapping.Apply(m, req.Payload)
		if err != nil {
			WriteError(w, http.StatusBadRequest, errcode.ParseTypeConvert, "mapping apply failed: "+err.Error())
			return
		}

		// Ensure validation_errors is never null in JSON
		if validationErrors == nil {
			validationErrors = []mapping.ValidationError{}
		}

		WriteJSON(w, http.StatusOK, map[string]any{
			"parsed_fields":     parsedFields,
			"validation_errors": validationErrors,
		})
	}
}

// ---------------------------------------------------------------------------
// PreviewDelivery — POST /api/v1/preview/delivery
// ---------------------------------------------------------------------------

// previewDeliveryRequest is the request body for the delivery preview endpoint.
type previewDeliveryRequest struct {
	Payload       string                 `json:"payload"`
	Fields        []mapping.MappingEntry `json:"fields"`
	TitleTemplate string                 `json:"title_template"`
	BodyTemplate  string                 `json:"body_template"`
	Format        string                 `json:"format"`
}

// PreviewDelivery runs the full pipeline: apply mapping then render template.
func PreviewDelivery() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req previewDeliveryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, http.StatusBadRequest, errcode.ConfigValidation, "invalid request body")
			return
		}
		if req.Payload == "" {
			WriteError(w, http.StatusBadRequest, errcode.ConfigValidation, "payload is required")
			return
		}

		// Step 1: Apply mapping
		m := &mapping.Mapping{
			Fields: req.Fields,
		}
		parsedFields, validationErrors, err := mapping.Apply(m, req.Payload)
		if err != nil {
			WriteError(w, http.StatusBadRequest, errcode.ParseTypeConvert, "mapping apply failed: "+err.Error())
			return
		}
		if validationErrors == nil {
			validationErrors = []mapping.ValidationError{}
		}

		// Step 2: Render template
		tmpl := &template.Template{
			TitleTemplate: req.TitleTemplate,
			BodyTemplate:  req.BodyTemplate,
			Format:        req.Format,
		}

		// Build template data: mapped fields at top level
		data := make(map[string]any, len(parsedFields))
		maps.Copy(data, parsedFields)

		msg, renderErr := template.Render(tmpl, data)

		renderedTitle := ""
		renderedBody := ""
		if renderErr != nil {
			validationErrors = append(validationErrors, mapping.ValidationError{
				Field:  "template",
				Code:   errcode.ParseMultiResult,
				Detail: "template render error: " + renderErr.Error(),
			})
		} else {
			renderedTitle = msg.Title
			renderedBody = msg.Body
		}

		WriteJSON(w, http.StatusOK, map[string]any{
			"parsed_fields":     parsedFields,
			"validation_errors": validationErrors,
			"rendered_title":    renderedTitle,
			"rendered_body":     renderedBody,
			"format":            req.Format,
		})
	}
}

// ---------------------------------------------------------------------------
// PreviewTemplate — POST /api/v1/preview/template
// ---------------------------------------------------------------------------

// PreviewTemplate renders a template with sample data using the real Go template engine.
// It accepts either a stored template_id, or an inline template spec (title_template /
// body_template / format) for previewing unsaved content — no database writes either way.
func PreviewTemplate(db *pgxpool.Pool) http.HandlerFunc {
	repo := template.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			TemplateID string `json:"template_id"`
			// Inline spec — used when template_id is empty
			TitleTemplate string         `json:"title_template"`
			BodyTemplate  string         `json:"body_template"`
			Format        string         `json:"format"`
			SampleData    map[string]any `json:"sample_data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, http.StatusBadRequest, errcode.ConfigValidation, "invalid request body")
			return
		}

		if req.SampleData == nil {
			req.SampleData = map[string]any{}
		}

		var tmpl *template.Template
		if req.TemplateID != "" {
			// Look up the stored template
			t, err := repo.GetByID(r.Context(), req.TemplateID)
			if err != nil {
				WriteError(w, http.StatusNotFound, errcode.ConfigValidation, "template not found")
				return
			}
			tmpl = t
		} else if req.TitleTemplate != "" || req.BodyTemplate != "" {
			// Inline content preview
			if req.Format == "" {
				req.Format = "text"
			}
			tmpl = &template.Template{
				TitleTemplate: req.TitleTemplate,
				BodyTemplate:  req.BodyTemplate,
				Format:        req.Format,
			}
		} else {
			WriteError(w, http.StatusBadRequest, errcode.ConfigValidation, "template_id or template content is required")
			return
		}

		// Render using the real Go template engine — sample data at top level
		data := make(map[string]any, len(req.SampleData))
		maps.Copy(data, req.SampleData)
		msg, renderErr := template.Render(tmpl, data)
		if renderErr != nil {
			WriteError(w, http.StatusBadRequest, errcode.ConfigValidation, "template render error: "+renderErr.Error())
			return
		}

		WriteJSON(w, http.StatusOK, map[string]any{
			"title":  msg.Title,
			"body":   msg.Body,
			"format": msg.Format,
		})
	}
}
