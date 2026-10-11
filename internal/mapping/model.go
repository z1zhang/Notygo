package mapping

import "time"

// Mapping represents a field mapping configuration.
type Mapping struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Revision  int          `json:"revision"`
	Fields    []MappingEntry `json:"fields"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// MappingEntry describes a single field extraction rule within a mapping.
type MappingEntry struct {
	SourcePath     string  `json:"source_path"`
	TargetKey      string  `json:"target_key"`
	ValueType      string  `json:"value_type"`
	DefaultValue   *string `json:"default_value,omitempty"`
	Required       bool    `json:"required"`
	Transform      *string `json:"transform,omitempty"`
	MultiResultMode *string `json:"multi_result_mode,omitempty"`
	Description    *string `json:"description,omitempty"`
}
