package api

// Error code domains and constants following the design doc format: DOMAIN_NNN

// Error represents a structured API error.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ErrorResponse is the standard error envelope.
type ErrorResponse struct {
	Error Error `json:"error"`
}

// NewError creates a new Error with the given code and message.
func NewError(code, message string) Error {
	return Error{Code: code, Message: message}
}

// --- Ingest errors ---

const (
	ErrIngestAuthFailed    = "INGEST_001"
	ErrIngestBodyTooLarge  = "INGEST_002"
	ErrIngestInvalidJSON   = "INGEST_003"
	ErrIngestRateLimited   = "INGEST_004"
	ErrIngestSourceMissing = "INGEST_005"
	ErrIngestDBUnavailable = "INGEST_006"
	ErrIngestDuplicate     = "INGEST_007"
)

// --- Auth errors ---

const (
	ErrAuthLocked = "AUTH_001" // too many failed login attempts
)

// --- Parse errors ---

const (
	ErrParsePathSyntax    = "PARSE_001"
	ErrParseRequiredField = "PARSE_002"
	ErrParseTypeConvert   = "PARSE_003"
	ErrParseMultiResult   = "PARSE_004"
)

// --- Rule errors ---

const (
	ErrRuleConditionSyntax = "RULE_001"
	ErrRuleEvalTimeout     = "RULE_002"
	ErrRuleRegexError      = "RULE_003"
)

// --- Template errors ---

const (
	ErrTplSyntax    = "TPL_001"
	ErrTplTimeout   = "TPL_002"
	ErrTplForbidden = "TPL_003"
)

// --- Delivery errors ---

const (
	ErrDeliverConnectFailed = "DELIVER_001"
	ErrDeliverAuthFailed    = "DELIVER_002"
	ErrDeliverTimeout       = "DELIVER_003"
	ErrDeliverBusinessError = "DELIVER_004"
	ErrDeliverRateLimited   = "DELIVER_005"
	ErrDeliverCircuitOpen   = "DELIVER_006"
	ErrDeliverLeaseExpired  = "DELIVER_007"
)

// --- Config errors ---

const (
	ErrConfigValidation = "CONFIG_001"
	ErrConfigReferenced = "CONFIG_002"
)

// --- System errors ---

const (
	ErrSystemMigration  = "SYSTEM_001"
	ErrSystemKeyInvalid = "SYSTEM_002"
)
