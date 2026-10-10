package templates

import "time"

// Template represents a notification template.
type Template struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Format        string    `json:"format"`
	TitleTemplate string    `json:"title_template"`
	BodyTemplate  string    `json:"body_template"`
	Revision      int       `json:"revision"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
