package templates

import "fmt"

// SMALLINT storage codes for templates.format (0=text 1=markdown 2=html),
// mirroring deliveries.rendered_format. The string values remain the endpoint
// of truth for the API; conversion happens only at the storage boundary.
// Keep in sync with the CHECK constraints in migrations/postgres/001_init.sql.

var formatCodes = map[string]int16{
	"text":     0,
	"markdown": 1,
	"html":     2,
}

func formatToCode(format string) (int16, error) {
	c, ok := formatCodes[format]
	if !ok {
		return 0, fmt.Errorf("unknown template format %q", format)
	}
	return c, nil
}

func formatFromCode(code int16) (string, error) {
	for name, c := range formatCodes {
		if c == code {
			return name, nil
		}
	}
	return "", fmt.Errorf("unknown template format code %d", code)
}
