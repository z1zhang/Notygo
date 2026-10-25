// Demo request handler: routes every apiClient call to the in-browser mock
// database. Mirrors the Go API surface (internal/api/router.go) — CRUD for
// the five config entities, event/delivery reads with their detail shapes,
// test/replay/retry actions, dashboard stats, and the preview pipeline.
import { DemoHttpError } from './mode'
import { loadState, nextId, nowIso, persist } from './store'
import { analyzePayload, applyMapping, renderTemplate, type DemoMappingEntry } from './preview'
import type { DemoState } from './seed'
import type {
  AuditLogResponse,
  DashboardStatsResponse,
  DeliveryAttemptResponse,
  DeliveryResponse,
  EventRunResponse,
  QueueStatsResponse,
  QueueStatsRow,
  RuleSnapshot,
} from '../types'

const API = '/api/v1'

type Entity =
  | 'endpoints'
  | 'mappings'
  | 'rules'
  | 'templates'
  | 'channels'
  | 'events'
  | 'deliveries'

const ID_PREFIX: Record<Entity, string> = {
  endpoints: 'src',
  mappings: 'map',
  rules: 'rule',
  templates: 'tmpl',
  channels: 'dest',
  events: 'evt',
  deliveries: 'dly',
}

type Params = Record<string, string | number | undefined>

export async function demoRequest<T>(
  method: string,
  path: string,
  body?: unknown,
  params?: Params,
): Promise<T> {
  // A sliver of latency so loading states look real
  await new Promise((resolve) => setTimeout(resolve, 80))
  const state = loadState()
  const result = dispatch(method, path, (body ?? {}) as Record<string, unknown>, params ?? {}, state)
  persist()
  return result as T
}

function dispatch(
  method: string,
  path: string,
  body: Record<string, unknown>,
  params: Params,
  state: DemoState,
): unknown {
  if (path === '/healthz' && method === 'GET') return { status: 'ok' }
  if (path === `${API}/system` && method === 'GET') {
    return {
      name: 'Notygo',
      version: '0.7.1',
      queue: {
        event_concurrency: 4,
        delivery_concurrency: 8,
        job_retention_seconds: 86400,
        retry_base_delay_seconds: 30,
        retry_max_delay_seconds: 3600,
        retry_jitter_percent: 20,
      },
      ingest: { max_payload_bytes: 1048576, rate_limit: '100/200', rate_per_second: 100, burst: 200 },
      login: { max_failures: 5, lockout_seconds: 900 },
      log_level: 'info',
    }
  }

  if (!path.startsWith(`${API}/`)) notFound()
  const segs = path.slice(API.length + 1).split('/')

  if (segs[0] === 'auth' && segs[1] === 'login' && method === 'POST') {
    return handleLogin(body, state)
  }
  if (segs[0] === 'dashboard' && segs[1] === 'stats' && method === 'GET') {
    return dashboardStats(state)
  }
  if (segs[0] === 'queue' && segs[1] === 'stats' && method === 'GET') {
    return queueStats(state)
  }
  if (segs[0] === 'audit' && segs.length === 1 && method === 'GET') {
    return listAudit(params, state)
  }
  if (segs[0] === 'preview' && method === 'POST') {
    return handlePreview(segs[1], body, state)
  }

  const entity = segs[0] as Entity
  if (!(entity in ID_PREFIX)) notFound()

  if (entity === 'deliveries' && segs[1] === 'bulk-retry' && method === 'POST') {
    return bulkRetry(body, state)
  }

  if (segs.length === 1) {
    if (method === 'GET') return listEntity(entity, params, state)
    if (method === 'POST') return createEntity(entity, body, state)
    notFound()
  }

  const id = segs[1]
  if (segs.length === 2) {
    if (method === 'GET') return getEntity(entity, id, state)
    if (method === 'PUT') return updateEntity(entity, id, body, state)
    if (method === 'DELETE') return deleteEntity(entity, id, state)
    notFound()
  }

  if (segs.length === 3 && method === 'POST') {
    if (entity === 'channels' && segs[2] === 'test') return testChannel(id, state)
    if (entity === 'events' && segs[2] === 'replay') return replayEvent(id, state)
    if (entity === 'deliveries' && segs[2] === 'retry') return retryDelivery(id, state)
  }
  notFound()
}

// ── Auth ─────────────────────────────────────────────────

function handleLogin(body: Record<string, unknown>, state: DemoState) {
  if (!body.username || !body.password) {
    throw new DemoHttpError(400, 'AUTH_000', 'username and password are required')
  }
  state.operator = String(body.username)
  addAudit(state, 'login', 'endpoint', '-', `${state.operator} logged in`)
  return {
    token: `demo-${Date.now().toString(36)}`,
    expires_at: new Date(Date.now() + 7 * 24 * 3600_000).toISOString(),
  }
}

// ── Generic entity CRUD ──────────────────────────────────

type AnyRow = Record<string, unknown> & { id: string }

function tableFor(entity: Entity, state: DemoState): AnyRow[] {
  return state[entity] as unknown as AnyRow[]
}

function listEntity(entity: Entity, params: Params, state: DemoState) {
  let rows = tableFor(entity, state)
  if (entity === 'events') {
    if (params.status) rows = rows.filter((r) => r.status === params.status)
    if (params.endpoint_id) rows = rows.filter((r) => r.endpoint_id === params.endpoint_id)
  }
  if (entity === 'deliveries') {
    if (params.status) rows = rows.filter((r) => r.status === params.status)
    if (params.channel_id) rows = rows.filter((r) => r.channel_id === params.channel_id)
    if (params.event_id) rows = rows.filter((r) => r.event_id === params.event_id)
  }
  const sorted = [...rows].sort((a, b) =>
    String(timeOf(b) ?? '').localeCompare(String(timeOf(a) ?? '')),
  )
  return paginate(sorted, params)
}

function timeOf(row: Record<string, unknown>): string | undefined {
  const t = row.received_at ?? row.created_at
  return typeof t === 'string' ? t : undefined
}

function paginate(rows: AnyRow[], params: Params) {
  const page = Math.max(1, Number(params.page) || 1)
  const size = Math.max(1, Number(params.size) || 20)
  const start = (page - 1) * size
  return { data: rows.slice(start, start + size), total: rows.length, page, size }
}

function createEntity(entity: Entity, body: Record<string, unknown>, state: DemoState) {
  const now = nowIso()
  const record: AnyRow = {
    ...body,
    id: nextId(ID_PREFIX[entity]),
    revision: 1,
    created_at: now,
    updated_at: now,
  }
  // Entity-specific defaults for fields the backend fills server-side
  if (entity === 'endpoints') {
    record.slug = record.slug || randomSlug()
    record.payload_content_type = record.payload_content_type || 'application/json'
    record.max_body_bytes = record.max_body_bytes ?? 1048576
    record.enabled = record.enabled ?? true
  }
  if (entity === 'mappings') {
    record.rule_ids = []
    record.continue_on_match = record.continue_on_match ?? false
  }
  if (entity === 'channels') {
    record.timeout_ms = record.timeout_ms ?? 10000
    record.max_attempts = record.max_attempts ?? 5
    record.enabled = record.enabled ?? true
  }
  if (entity === 'rules') {
    record.enabled = record.enabled ?? true
    record.priority = record.priority ?? 0
  }
  if (entity === 'events' || entity === 'deliveries') {
    throw new DemoHttpError(405, 'CONFIG_001', `${entity} are read-only in demo mode`)
  }
  tableFor(entity, state).unshift(record)
  addAudit(
    state,
    'create',
    singular(entity),
    record.id,
    `created ${singular(entity)} "${record.name}"`,
  )
  return record
}

function updateEntity(entity: Entity, id: string, body: Record<string, unknown>, state: DemoState) {
  const rows = tableFor(entity, state)
  const index = rows.findIndex((r) => r.id === id)
  if (index === -1) notFound()
  const before = JSON.stringify(rows[index])
  const merged: AnyRow = {
    ...rows[index],
    ...body,
    id, // id is immutable
    revision: Number(rows[index].revision ?? 1) + 1,
    updated_at: nowIso(),
  }
  rows[index] = merged
  addAudit(state, 'update', singular(entity), id, `updated ${singular(entity)} "${merged.name}"`, before)
  return merged
}

function deleteEntity(entity: Entity, id: string, state: DemoState) {
  const rows = tableFor(entity, state)
  const index = rows.findIndex((r) => r.id === id)
  if (index === -1) notFound()
  const [removed] = rows.splice(index, 1)
  addAudit(
    state,
    'delete',
    singular(entity),
    id,
    `deleted ${singular(entity)} "${removed.name}"`,
    JSON.stringify(removed),
  )
  return undefined // 204
}

function getEntity(entity: Entity, id: string, state: DemoState) {
  const rows = tableFor(entity, state)
  const row = rows.find((r) => r.id === id)
  if (!row) notFound()

  if (entity === 'events') {
    const runs = state.eventRuns
      .filter((r) => r.event_id === id)
      .sort((a, b) => String(b.started_at ?? '').localeCompare(String(a.started_at ?? '')))
    return { event: row, latest_run: runs[0] ?? null }
  }
  if (entity === 'deliveries') {
    const attempts = state.deliveryAttempts
      .filter((a) => a.delivery_id === id)
      .sort((a, b) => a.attempt_no - b.attempt_no)
    return { delivery: row, delivery_attempts: attempts }
  }
  return row
}

function singular(entity: Entity): string {
  return entity.replace(/s$/, '')
}

function randomSlug(): string {
  return (crypto.randomUUID?.() ?? `${Date.now()}-${Math.random()}`).replace(/-/g, '')
}

// ── Audit list ───────────────────────────────────────────

function listAudit(params: Params, state: DemoState) {
  let rows = state.auditLogs
  if (params.action) rows = rows.filter((r) => r.action === params.action)
  if (params.target_type) rows = rows.filter((r) => r.target_type === params.target_type)
  if (params.start_time) {
    rows = rows.filter((r) => (r.created_at ?? '') >= String(params.start_time))
  }
  if (params.end_time) {
    rows = rows.filter((r) => (r.created_at ?? '') <= String(params.end_time))
  }
  return paginate(rows as unknown as AnyRow[], params)
}

// ── Dashboard ────────────────────────────────────────────

// Mirrors the real endpoint's shape: river_job counts by queue and state.
// The demo derives a plausible snapshot from the business rows.
function queueStats(state: DemoState): QueueStatsResponse {
  const pending = state.deliveries.filter(
    (d) => d.status === 'pending' || d.status === 'retry_wait',
  ).length
  const processing = state.deliveries.filter((d) => d.status === 'processing').length
  const rows: QueueStatsRow[] = [
    { queue: 'event_process', state: 'available', count: state.events.filter((e) => e.status === 'received').length },
    { queue: 'delivery_send', state: 'available', count: pending },
    { queue: 'delivery_send', state: 'running', count: processing },
    { queue: 'delivery_send', state: 'completed', count: state.deliveries.filter((d) => d.status === 'delivered').length },
    { queue: 'maintenance', state: 'completed', count: 1 },
  ]
  return { queues: rows, retention_seconds: 86400 }
}

function dashboardStats(state: DemoState): DashboardStatsResponse {
  const failedEvents = state.events
    .filter((e) => e.status === 'failed')
    .sort((a, b) => (b.received_at ?? '').localeCompare(a.received_at ?? ''))
  const failedDeliveries = state.deliveries
    .filter((d) => d.status === 'failed')
    .sort((a, b) => (b.created_at ?? '').localeCompare(a.created_at ?? ''))
  return {
    total_events: state.events.length,
    processing_events: state.events.filter((e) => e.status === 'processing').length,
    pending_deliveries: state.deliveries.filter(
      (d) => d.status === 'pending' || d.status === 'retry_wait' || d.status === 'processing',
    ).length,
    failed_deliveries: failedDeliveries.length,
    recent_failed_events: failedEvents.slice(0, 5).map((e) => ({
      id: e.id,
      endpoint_id: e.endpoint_id,
      status: e.status,
      error_code: e.error_code,
      error_summary: e.error_summary,
      received_at: e.received_at ?? '',
    })),
    recent_failed_deliveries: failedDeliveries.slice(0, 5).map((d) => ({
      id: d.id,
      channel_id: d.channel_id,
      status: d.status,
      last_error_code: d.last_error_code,
      last_error_summary: d.last_error_summary,
      created_at: d.created_at ?? '',
    })),
  }
}

// ── Actions ──────────────────────────────────────────────

function testChannel(id: string, state: DemoState) {
  const dest = tableFor('channels', state).find((r) => r.id === id)
  if (!dest) notFound()
  return {
    success: true,
    status_code: 200,
    response_body: 'demo mode: test message delivered (simulated)',
  }
}

function replayEvent(id: string, state: DemoState) {
  const event = tableFor('events', state).find((r) => r.id === id)
  if (!event) notFound()
  const previous = state.eventRuns
    .filter((r) => r.event_id === id)
    .sort((a, b) => String(b.started_at ?? '').localeCompare(String(a.started_at ?? '')))[0]

  const ruleSnapshot: RuleSnapshot[] = state.rules.map((r) => ({
    id: r.id,
    name: r.name,
    priority: r.priority,
    conditions: r.conditions,
    channels: r.channel_ids,
  }))
  const run: EventRunResponse = {
    id: nextId('run'),
    event_id: id,
    run_type: 'replay',
    mapping_id: previous?.mapping_id ?? null,
    mapping_revision: previous?.mapping_revision ?? null,
    rule_set_snapshot: ruleSnapshot,
    parsed_fields: previous?.parsed_fields ?? {},
    validation_result: { valid: true },
    matched_rule_ids: previous?.matched_rule_ids ?? [],
    status: 'completed',
    started_at: nowIso(),
    finished_at: nowIso(),
    error_code: null,
    error_summary: null,
  }
  state.eventRuns.push(run)
  addAudit(state, 'replay', 'event', id, `replayed event ${id}`)
  return { event_id: id, message: 'event replayed (demo)' }
}

function retryDelivery(id: string, state: DemoState) {
  const delivery = state.deliveries.find((d) => d.id === id)
  if (!delivery) notFound()
  simulateDeliverySuccess(delivery, state)
  addAudit(state, 'retry', 'delivery', id, `retried delivery ${id}`)
  return { delivery, message: 'delivery retried (demo)' }
}

function bulkRetry(body: Record<string, unknown>, state: DemoState) {
  const status = typeof body.status === 'string' ? body.status : undefined
  const channelId = typeof body.channel_id === 'string' ? body.channel_id : undefined
  const targets = state.deliveries.filter(
    (d) =>
      (d.status === 'failed' || d.status === 'retry_wait') &&
      (!status || d.status === status) &&
      (!channelId || d.channel_id === channelId),
  )
  for (const delivery of targets) {
    simulateDeliverySuccess(delivery, state)
  }
  addAudit(state, 'bulk_retry', 'delivery', '-', `bulk retried ${targets.length} deliveries`)
  return { reset_count: targets.length, message: `${targets.length} deliveries reset (demo)` }
}

// Demo stand-in for the delivery worker picking the job back up
function simulateDeliverySuccess(delivery: DeliveryResponse, state: DemoState) {
  const attempts = state.deliveryAttempts.filter((a) => a.delivery_id === delivery.id)
  const attemptNo = attempts.length + 1
  const duration = 200 + Math.floor(Math.random() * 500)
  const started = nowIso()
  const attempt: DeliveryAttemptResponse = {
    id: nextId('att'),
    delivery_id: delivery.id,
    attempt_no: attemptNo,
    started_at: started,
    finished_at: new Date(Date.now() + duration).toISOString(),
    request_summary: '(demo) resent',
    response_status: 200,
    response_summary: 'ok',
    error_code: null,
    error_summary: null,
    duration_ms: duration,
  }
  state.deliveryAttempts.push(attempt)
  Object.assign(delivery, {
    status: 'delivered',
    attempt_count: attemptNo,
    next_attempt_at: null,
    last_error_code: null,
    last_error_summary: null,
    updated_at: nowIso(),
  })
}

// ── Preview pipeline ─────────────────────────────────────

function handlePreview(kind: string | undefined, body: Record<string, unknown>, state: DemoState) {
  switch (kind) {
    case 'parse':
      return analyzePayload(stringField(body, 'payload'))
    case 'mapping':
      return applyMapping(stringField(body, 'payload'), (body.fields ?? []) as DemoMappingEntry[])
    case 'delivery': {
      const { parsed_fields, validation_errors } = applyMapping(
        stringField(body, 'payload'),
        (body.fields ?? []) as DemoMappingEntry[],
      )
      let renderedTitle = ''
      let renderedBody = ''
      try {
        renderedTitle = renderTemplate(String(body.title_template ?? ''), parsed_fields)
        renderedBody = renderTemplate(String(body.body_template ?? ''), parsed_fields)
      } catch (e) {
        validation_errors.push({
          field: 'template',
          code: 'PARSE_004',
          detail: `template render error: ${e instanceof Error ? e.message : String(e)}`,
        })
      }
      return {
        parsed_fields,
        validation_errors,
        rendered_title: renderedTitle,
        rendered_body: renderedBody,
        format: String(body.format ?? 'text'),
      }
    }
    case 'template': {
      let title: string
      let bodyTemplate: string
      let format: string
      if (typeof body.template_id === 'string' && body.template_id !== '') {
        const tmpl = state.templates.find((t) => t.id === body.template_id)
        if (!tmpl) throw new DemoHttpError(404, 'CONFIG_001', 'template not found')
        title = tmpl.title_template
        bodyTemplate = tmpl.body_template
        format = tmpl.format
      } else if (body.title_template || body.body_template) {
        title = String(body.title_template ?? '')
        bodyTemplate = String(body.body_template ?? '')
        format = String(body.format ?? 'text')
      } else {
        throw new DemoHttpError(400, 'CONFIG_001', 'template_id or template content is required')
      }
      const sampleData = (body.sample_data ?? {}) as Record<string, unknown>
      try {
        return {
          title: renderTemplate(title, sampleData),
          body: renderTemplate(bodyTemplate, sampleData),
          format,
        }
      } catch (e) {
        throw new DemoHttpError(
          400,
          'CONFIG_001',
          `template render error: ${e instanceof Error ? e.message : String(e)}`,
        )
      }
    }
    default:
      notFound()
  }
}

function stringField(body: Record<string, unknown>, key: string): string {
  const value = body[key]
  if (typeof value !== 'string' || value === '') {
    throw new DemoHttpError(400, 'CONFIG_001', `${key} is required`)
  }
  return value
}

// ── helpers ──────────────────────────────────────────────

function addAudit(
  state: DemoState,
  action: string,
  targetType: string,
  targetId: string,
  summary: string,
  snapshotBefore: string | null = null,
) {
  const entry: AuditLogResponse = {
    id: nextId('aud'),
    operator: state.operator,
    action,
    target_type: targetType,
    target_id: targetId,
    snapshot_before: snapshotBefore,
    summary,
    created_at: nowIso(),
  }
  state.auditLogs.unshift(entry)
}

function notFound(): never {
  throw new DemoHttpError(404, 'CONFIG_001', 'endpoint not found')
}
