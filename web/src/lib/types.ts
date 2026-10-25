// API contract types — ported from the Go backend DTOs (internal/api handlers).

// ── Generic ──────────────────────────────────────────────

export interface PaginatedResponse<T> {
  data: T[]
  total: number
  page: number
  size: number
}

// ── Endpoint ───────────────────────────────────────────────

export interface EndpointResponse {
  id: string
  name: string
  slug: string
  enabled: boolean
  auth_type: string
  /** "configured" | "not_configured" marker from the server — never re-submit */
  secret?: string | null
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
  payload_path: string
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
  channel_ids: string[]
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

// ── Channel ──────────────────────────────────────────

export interface ChannelResponse {
  id: string
  name: string
  type: string
  enabled: boolean
  config: Record<string, unknown>
  /** Decrypted plaintext secret from the server */
  secret?: string | null
  timeout_ms: number
  max_attempts?: number
  revision: number
  created_at?: string
  updated_at?: string
}

// ── Event ────────────────────────────────────────────────

export interface EventResponse {
  id: string
  endpoint_id: string
  upstream_event_id?: string | null
  request_id: string
  received_at?: string
  content_type: string
  payload_raw: string
  payload_hash: string
  status: string
  error_code?: string | null
  error_summary?: string | null
}

export interface RuleSnapshot {
  id: string
  name: string
  priority: number
  conditions: unknown
  channels: string[]
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
  channel_id: string
  idempotency_key: string
  status: string
  attempt_count: number
  max_attempts: number
  next_attempt_at?: string
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
  payload_path: string
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

// ── Dashboard ────────────────────────────────────────────

export interface FailedEventSummary {
  id: string
  endpoint_id: string
  status: string
  error_code?: string | null
  error_summary?: string | null
  received_at: string
}

export interface FailedDeliverySummary {
  id: string
  channel_id: string
  status: string
  last_error_code?: string | null
  last_error_summary?: string | null
  created_at: string
}

export interface DashboardStatsResponse {
  total_events: number
  processing_events: number
  pending_deliveries: number
  failed_deliveries: number
  recent_failed_events: FailedEventSummary[] | null
  recent_failed_deliveries: FailedDeliverySummary[] | null
}

export interface QueueStatsRow {
  queue: string
  state: string
  count: number
}

export interface QueueStatsResponse {
  queues: QueueStatsRow[] | null
  /** How long finished jobs stay visible before River deletes them (seconds) */
  retention_seconds?: number
}

/** Non-sensitive runtime configuration exposed by GET /api/v1/system */
export interface SystemInfoResponse {
  name: string
  version: string
  queue: {
    event_concurrency: number
    delivery_concurrency: number
    job_retention_seconds: number
    retry_base_delay_seconds: number
    retry_max_delay_seconds: number
    retry_jitter_percent: number
  }
  ingest: {
    max_payload_bytes: number
    rate_limit: string
    rate_per_second: number
    burst: number
  }
  login: {
    max_failures: number
    lockout_seconds: number
  }
  log_level: string
}
