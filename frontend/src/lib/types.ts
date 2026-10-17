// API contract types — ported from the Go backend DTOs (internal/api handlers).

// ── Generic ──────────────────────────────────────────────

export interface PaginatedResponse<T> {
  data: T[]
  total: number
  page: number
  size: number
}

// ── Source ───────────────────────────────────────────────

export interface SourceResponse {
  id: string
  name: string
  slug: string
  enabled: boolean
  auth_type: string
  /** "configured" | "not_configured" marker from the server — never re-submit */
  secret_ref?: string | null
  payload_content_type: string
  max_body_bytes: number
  /** JSON string like {"rate":N,"burst":N} */
  rate_limit?: string | null
  mapping_id?: string | null
  created_at?: string
  updated_at?: string
}

// ── Mapping ──────────────────────────────────────────────

export interface MappingEntry {
  source_path: string
  target_key: string
  value_type: string
  default_value?: string | null
  required: boolean
  transform?: string | null
  multi_result_mode?: string | null
  description?: string | null
}

export interface MappingResponse {
  id: string
  name: string
  revision: number
  rule_ids?: string[]
  continue_on_match: boolean
  fields: MappingEntry[]
  created_at?: string
  updated_at?: string
}

// ── Rules ────────────────────────────────────────────────

export interface Condition {
  field: string
  operator: string
  value: string
}

export interface ConditionGroup {
  operator?: string
  conditions?: Condition[]
  groups?: ConditionGroup[]
}

export interface DedupeConfig {
  window_seconds: number
}

export interface RuleResponse {
  id: string
  name: string
  enabled: boolean
  priority: number
  conditions: ConditionGroup
  template_id?: string | null
  destination_ids: string[]
  dedupe_config?: DedupeConfig | null
  revision: number
  created_at?: string
  updated_at?: string
}

// ── Template ─────────────────────────────────────────────

export interface TemplateResponse {
  id: string
  name: string
  format: string
  title_template: string
  body_template: string
  revision: number
  created_at?: string
  updated_at?: string
}

// ── Destination ──────────────────────────────────────────

export interface DestinationResponse {
  id: string
  name: string
  type: string
  enabled: boolean
  config: Record<string, unknown>
  /** Decrypted plaintext secret from the server */
  secret_ref?: string | null
  timeout_ms: number
  revision: number
  created_at?: string
  updated_at?: string
}

// ── Event ────────────────────────────────────────────────

export interface EventResponse {
  id: string
  source_id: string
  source_event_id?: string | null
  request_id: string
  received_at?: string
  content_type: string
  payload_raw: string
  payload_hash: string
  status: string
  error_code?: string | null
  error_summary?: string | null
  lease_until?: string | null
  lease_owner?: string | null
}

export interface RuleSnapshot {
  id: string
  name: string
  priority: number
  conditions: unknown
  destinations: string[]
}

export interface EventRunResponse {
  id: string
  event_id: string
  run_type: string
  mapping_id?: string | null
  mapping_revision?: number | null
  rule_set_snapshot: RuleSnapshot[]
  parsed_fields: Record<string, unknown>
  validation_result: Record<string, unknown>
  matched_rule_ids: string[]
  status: string
  started_at?: string
  finished_at?: string | null
  error_code?: string | null
  error_summary?: string | null
}

export interface GetEventResponse {
  event: EventResponse
  latest_run: EventRunResponse | null
}

// ── Delivery ─────────────────────────────────────────────

export interface DeliveryResponse {
  id: string
  event_id: string
  event_run_id: string
  rule_id: string
  destination_id: string
  idempotency_key: string
  status: string
  attempt_count: number
  max_attempts: number
  next_attempt_at?: string
  lease_until?: string | null
  lease_owner?: string | null
  rendered_title?: string | null
  rendered_body?: string | null
  rendered_format?: string | null
  last_error_code?: string | null
  last_error_summary?: string | null
  created_at?: string
  updated_at?: string
}

export interface DeliveryAttemptResponse {
  id: string
  delivery_id: string
  attempt_no: number
  started_at?: string
  finished_at?: string | null
  request_summary?: string | null
  response_status?: number | null
  response_summary?: string | null
  error_code?: string | null
  error_summary?: string | null
  duration_ms?: number | null
}

export interface GetDeliveryResponse {
  delivery: DeliveryResponse
  delivery_attempts: DeliveryAttemptResponse[]
}

// ── Audit ────────────────────────────────────────────────

export interface AuditLogResponse {
  id: string
  operator: string
  action: string
  target_type: string
  target_id: string
  snapshot_before?: string | null
  summary?: string | null
  created_at?: string
}

// ── Preview ──────────────────────────────────────────────

export interface StructureInfo {
  type: string // "array" | "object"
  root_is_array: boolean
  item_count: number
}

export interface SuggestedField {
  source_path: string
  target_key: string
  value_type: string
  multi_result_mode: string | null
  sample_value: string
}

export interface PreviewParseResponse {
  structure: StructureInfo
  suggested_fields: SuggestedField[]
}

export interface ValidationErrorItem {
  field: string
  code: string
  detail: string
}

export interface PreviewMappingResponse {
  parsed_fields: Record<string, unknown>
  validation_errors: ValidationErrorItem[]
}

export interface PreviewDeliveryResponse {
  parsed_fields: Record<string, unknown>
  validation_errors: ValidationErrorItem[]
  rendered_title: string
  rendered_body: string
  format: string
}

export interface PreviewTemplateResponse {
  title: string
  body: string
  format: string
}

// ── Auth ─────────────────────────────────────────────────

export interface LoginResponse {
  token: string
  expires_at: string
}

// ── API error shape (WriteError envelope) ────────────────

export interface ApiErrorEnvelope {
  error: {
    code: string
    message: string
  }
}
