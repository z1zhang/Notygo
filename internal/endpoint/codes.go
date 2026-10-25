package endpoint

import "fmt"

// SMALLINT storage codes for endpoints.auth_type (0=none 1=bearer). The string
// values remain the source of truth for the API; conversion happens only at
// the storage boundary. Keep in sync with the CHECK constraints in
// migrations/postgres/001_init.sql.

var authTypeCodes = map[string]int16{
	"none":   0,
	"bearer": 1,
}

func authTypeToCode(authType string) (int16, error) {
	c, ok := authTypeCodes[authType]
	if !ok {
		return 0, fmt.Errorf("unknown auth type %q", authType)
	}
	return c, nil
}

func authTypeFromCode(code int16) (string, error) {
	for name, c := range authTypeCodes {
		if c == code {
			return name, nil
		}
	}
	return "", fmt.Errorf("unknown auth type code %d", code)
}
