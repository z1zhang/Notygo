package event

import "fmt"

// SMALLINT storage codes for events.status, events.run_type and
// event_runs.status / event_runs.run_type. The string constants in model.go
// remain the source of truth for business logic and the API; these codes only
// exist at the storage boundary. Keep in sync with the CHECK constraints in
// migrations/postgres/001_init.sql.

const (
	eventStatusCodeReceived   int16 = 0
	eventStatusCodeProcessing int16 = 1
	eventStatusCodeCompleted  int16 = 2
	eventStatusCodeDeduped    int16 = 3
	eventStatusCodeFailed     int16 = 4
)

const (
	eventRunStatusCodeProcessing int16 = 0
	eventRunStatusCodeCompleted  int16 = 1
	eventRunStatusCodeFailed     int16 = 2
)

const (
	runTypeCodeInitial int16 = 0
	runTypeCodeReplay  int16 = 1
)

var eventStatusCodes = map[string]int16{
	EventStatusReceived:   eventStatusCodeReceived,
	EventStatusProcessing: eventStatusCodeProcessing,
	EventStatusCompleted:  eventStatusCodeCompleted,
	EventStatusDeduped:    eventStatusCodeDeduped,
	EventStatusFailed:     eventStatusCodeFailed,
}

var eventRunStatusCodes = map[string]int16{
	EventRunStatusProcessing: eventRunStatusCodeProcessing,
	EventRunStatusCompleted:  eventRunStatusCodeCompleted,
	EventRunStatusFailed:     eventRunStatusCodeFailed,
}

var runTypeCodes = map[string]int16{
	EventRunTypeInitial: runTypeCodeInitial,
	EventRunTypeReplay:  runTypeCodeReplay,
}

// StatusToCode converts an event status name to its storage code.
func StatusToCode(status string) (int16, error) {
	c, ok := eventStatusCodes[status]
	if !ok {
		return 0, fmt.Errorf("unknown event status %q", status)
	}
	return c, nil
}

// StatusFromCode converts an event status storage code to its name.
func StatusFromCode(code int16) (string, error) {
	for name, c := range eventStatusCodes {
		if c == code {
			return name, nil
		}
	}
	return "", fmt.Errorf("unknown event status code %d", code)
}

// RunStatusToCode converts an event run status name to its storage code.
func RunStatusToCode(status string) (int16, error) {
	c, ok := eventRunStatusCodes[status]
	if !ok {
		return 0, fmt.Errorf("unknown event run status %q", status)
	}
	return c, nil
}

// RunStatusFromCode converts an event run status storage code to its name.
func RunStatusFromCode(code int16) (string, error) {
	for name, c := range eventRunStatusCodes {
		if c == code {
			return name, nil
		}
	}
	return "", fmt.Errorf("unknown event run status code %d", code)
}

// RunTypeToCode converts a run type name to its storage code.
func RunTypeToCode(runType string) (int16, error) {
	c, ok := runTypeCodes[runType]
	if !ok {
		return 0, fmt.Errorf("unknown run type %q", runType)
	}
	return c, nil
}

// RunTypeFromCode converts a run type storage code to its name.
func RunTypeFromCode(code int16) (string, error) {
	for name, c := range runTypeCodes {
		if c == code {
			return name, nil
		}
	}
	return "", fmt.Errorf("unknown run type code %d", code)
}
