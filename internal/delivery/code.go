package delivery

import "fmt"

// SMALLINT storage codes for deliveries.status and
// deliveries.rendered_format. The string constants in model.go remain the
// source of truth for business logic and the API; these codes only exist at
// the storage boundary. Keep in sync with the CHECK constraints in
// migrations/postgres/001_init.sql.

const (
	deliveryStatusCodePending    int16 = 0
	deliveryStatusCodeRetryWait  int16 = 1
	deliveryStatusCodeProcessing int16 = 2
	deliveryStatusCodeDelivered  int16 = 3
	deliveryStatusCodeFailed     int16 = 4
	deliveryStatusCodeSkipped    int16 = 5
)

var deliveryStatusCodes = map[string]int16{
	DeliveryStatusPending:    deliveryStatusCodePending,
	DeliveryStatusRetryWait:  deliveryStatusCodeRetryWait,
	DeliveryStatusProcessing: deliveryStatusCodeProcessing,
	DeliveryStatusDelivered:  deliveryStatusCodeDelivered,
	DeliveryStatusFailed:     deliveryStatusCodeFailed,
	DeliveryStatusSkipped:    deliveryStatusCodeSkipped,
}

// renderedFormatCodes mirrors template.format (0=text 1=markdown 2=html);
// both columns share the same value space.
var renderedFormatCodes = map[string]int16{
	"text":     0,
	"markdown": 1,
	"html":     2,
}

func statusToCode(status string) (int16, error) {
	c, ok := deliveryStatusCodes[status]
	if !ok {
		return 0, fmt.Errorf("unknown delivery status %q", status)
	}
	return c, nil
}

func statusFromCode(code int16) (string, error) {
	for name, c := range deliveryStatusCodes {
		if c == code {
			return name, nil
		}
	}
	return "", fmt.Errorf("unknown delivery status code %d", code)
}

func renderedFormatToCode(format string) (int16, error) {
	c, ok := renderedFormatCodes[format]
	if !ok {
		return 0, fmt.Errorf("unknown rendered format %q", format)
	}
	return c, nil
}

func renderedFormatFromCode(code int16) (string, error) {
	for name, c := range renderedFormatCodes {
		if c == code {
			return name, nil
		}
	}
	return "", fmt.Errorf("unknown rendered format code %d", code)
}
