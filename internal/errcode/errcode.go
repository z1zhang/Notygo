// Package errcode is the single registry of error codes (DOMAIN_NNN) that
// appear in API error envelopes, event error_code columns, and delivery
// attempt logs. It has zero internal dependencies so every layer — ingest,
// mapping, processor, delivery, api — can share it without import cycles.
//
// Registry policy: every code listed here is actively emitted somewhere.
// Code values are an API contract (the web frontend and demo mode hard-code
// them), so they never change once shipped; retiring a code means removing
// it from this file and deleting its emission sites in the same change.
package errcode

// Ingest error codes (INGEST_NNN): data-plane request failures.
const (
	IngestAuthFailed      = "INGEST_001" // HMAC signature invalid or missing
	IngestBodyTooLarge    = "INGEST_002"
	IngestInvalidJSON     = "INGEST_003" // body unreadable or not valid JSON
	IngestRateLimited     = "INGEST_004"
	IngestEndpointMissing = "INGEST_005" // slug absent, unknown, or disabled
	IngestDBUnavailable   = "INGEST_006"
	IngestDuplicate       = "INGEST_007" // upstream_event_id already seen
)

// Auth error codes (AUTH_NNN): control-plane login failures.
const (
	AuthLocked = "AUTH_001" // too many failed login attempts
)

// Parse error codes (PARSE_NNN): payload mapping/validation failures,
// emitted by mapping.Apply. No path-syntax code: unresolvable field paths
// resolve to nil and are handled by required/empty operators instead of
// erroring.
const (
	ParseRequiredField = "PARSE_002"
	ParseTypeConvert   = "PARSE_003"
	ParseMultiResult   = "PARSE_004" // multi-value mode produced the wrong shape
)

// Rule error codes (RULE_NNN): rule validation failures.
// No timeout code: evaluation is CPU-only on RE2 (linear-time, no
// catastrophic backtracking) and bounded payloads, so it cannot hang —
// unlike template rendering, which is time-bounded in the renderer.
const (
	RuleConditionSyntax = "RULE_001" // includes invalid regex in a condition
)

// Template error codes (TPL_NNN): template parse/render failures.
// Render timeouts surface as TPL_001 with the timeout error text; there is
// no separate timeout or forbidden-function code because no site emits one.
const (
	TplSyntax = "TPL_001"
)

// Delivery error codes (DELIVER_NNN): send failures, by cause.
// Network timeouts count as connect failures (DELIVER_001) — the sender's
// context deadline expires before a response is classified, so no distinct
// timeout code is emitted. No circuit-breaker or lease codes: those
// features do not exist.
const (
	DeliverConnectFailed = "DELIVER_001" // network error, channel missing, or HTTP >= 500
	DeliverAuthFailed    = "DELIVER_002" // HTTP 401/403
	DeliverBusinessError = "DELIVER_004" // other non-success status
	DeliverRateLimited   = "DELIVER_005" // HTTP 429
)

// Processor error codes (PROCESSOR_NNN): pipeline-stage failures recorded
// on the event (notygo_event.error_code). Numbering skips 002, which was
// never assigned; new processor codes continue from 006.
const (
	ProcessorEndpointNotFound = "PROCESSOR_001"
	ProcessorMappingNotFound  = "PROCESSOR_003"
	ProcessorRulesFetchFailed = "PROCESSOR_004"
	ProcessorTemplateRender   = "PROCESSOR_005"
)

// Config error codes (CONFIG_NNN): entity CRUD validation failures.
// No referenced-by code: delete handlers reject references with a plain
// validation error (CONFIG_001) instead of a distinct code.
const (
	ConfigValidation = "CONFIG_001"
)

// System error codes (SYSTEM_NNN): infrastructure failures.
// No migration code: migration failure happens at startup, before the HTTP
// plane exists to answer with an error envelope.
const (
	SystemKeyInvalid = "SYSTEM_002"
	SystemInternal   = "SYSTEM_999" // catch-all for unexpected handler errors
)
