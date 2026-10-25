// Seed data for demo mode: a self-consistent little deployment —
// Diun (docker image update watcher) + CI webhooks feeding notifications
// to 钉钉 / 飞书 / 邮件 channels. Times are generated relative to "now"
// so the demo always looks fresh.
import type {
  AuditLogResponse,
  DeliveryAttemptResponse,
  DeliveryResponse,
  ChannelResponse,
  EventResponse,
  EventRunResponse,
  MappingResponse,
  RuleResponse,
  EndpointResponse,
  TemplateResponse,
} from '../types'

export interface DemoState {
  endpoints: EndpointResponse[]
  mappings: MappingResponse[]
  rules: RuleResponse[]
  templates: TemplateResponse[]
  channels: ChannelResponse[]
  events: EventResponse[]
  eventRuns: EventRunResponse[]
  deliveries: DeliveryResponse[]
  deliveryAttempts: DeliveryAttemptResponse[]
  auditLogs: AuditLogResponse[]
  operator: string
}

const agoIso = (mins: number) => new Date(Date.now() - mins * 60_000).toISOString()
const aheadIso = (mins: number) => new Date(Date.now() + mins * 60_000).toISOString()
const reqId = (n: number) => `req-demo-${n.toString().padStart(4, '0')}`
const hash = (seed: string) => {
  let h = 0n
  for (const ch of seed) h = (h * 31n + BigInt(ch.charCodeAt(0))) & 0xffffffffffffffffn
  return h.toString(16).padStart(16, '0').repeat(4)
}

export function buildSeed(): DemoState {
  // ── Endpoints ─────────────────────────────────────────────
  const endpoints: EndpointResponse[] = [
    {
      id: 'src-diun',
      name: 'Diun 镜像监控',
      slug: 'diun',
      enabled: true,
      auth_type: 'hmac',
      secret: 'demo-hmac-secret',
      payload_content_type: 'application/json',
      max_body_bytes: 1048576,
      rate_limit: '{"rate":60,"burst":10}',
      mapping_id: 'map-diun',
      created_at: agoIso(60 * 24 * 14),
      updated_at: agoIso(60 * 24 * 6),
    },
    {
      id: 'src-ci',
      name: 'CI 构建通知',
      slug: 'ci',
      enabled: true,
      auth_type: 'none',
      secret: null,
      payload_content_type: 'application/json',
      max_body_bytes: 1048576,
      rate_limit: null,
      mapping_id: 'map-ci',
      created_at: agoIso(60 * 24 * 10),
      updated_at: agoIso(60 * 24 * 10),
    },
  ]

  // ── Mappings ────────────────────────────────────────────
  const mappings: MappingResponse[] = [
    {
      id: 'map-diun',
      name: 'Diun 字段映射',
      revision: 2,
      rule_ids: ['rule-image-update'],
      continue_on_match: false,
      fields: [
        { payload_path: 'image', target_key: 'image', value_type: 'string', required: true },
        { payload_path: 'status', target_key: 'status', value_type: 'string', required: true },
        { payload_path: 'provider', target_key: 'provider', value_type: 'string', default_value: 'file', required: false },
        { payload_path: 'link', target_key: 'link', value_type: 'string', required: false },
        { payload_path: 'manifest.digest', target_key: 'digest', value_type: 'string', required: false },
        { payload_path: 'hostname', target_key: 'hostname', value_type: 'string', required: false },
      ],
      created_at: agoIso(60 * 24 * 14),
      updated_at: agoIso(60 * 24 * 6),
    },
    {
      id: 'map-ci',
      name: 'CI 字段映射',
      revision: 1,
      rule_ids: ['rule-build-fail'],
      continue_on_match: false,
      fields: [
        { payload_path: 'repository', target_key: 'repo', value_type: 'string', required: true },
        { payload_path: 'branch', target_key: 'branch', value_type: 'string', required: false },
        { payload_path: 'status', target_key: 'status', value_type: 'string', required: true },
        { payload_path: 'actor', target_key: 'author', value_type: 'string', required: false },
        { payload_path: 'jobs', target_key: 'job_count', value_type: 'number', multi_result_mode: 'count', required: false },
        { payload_path: 'jobs', target_key: 'jobs', value_type: 'array', multi_result_mode: 'array', required: false },
      ],
      created_at: agoIso(60 * 24 * 10),
      updated_at: agoIso(60 * 24 * 10),
    },
  ]

  // ── Rules ───────────────────────────────────────────────
  const rules: RuleResponse[] = [
    {
      id: 'rule-image-update',
      name: '镜像有更新',
      enabled: true,
      priority: 10,
      conditions: {
        operator: 'and',
        conditions: [{ field: 'status', operator: 'eq', value: 'new' }],
      },
      template_id: 'tmpl-image-update',
      channel_ids: ['dest-ding-ops', 'dest-feishu-alert'],
      dedupe_config: { window_seconds: 300 },
      revision: 3,
      created_at: agoIso(60 * 24 * 13),
      updated_at: agoIso(60 * 24 * 5),
    },
    {
      id: 'rule-build-fail',
      name: '构建失败告警',
      enabled: true,
      priority: 5,
      conditions: {
        operator: 'and',
        conditions: [{ field: 'status', operator: 'eq', value: 'failure' }],
      },
      template_id: 'tmpl-ci-fail',
      channel_ids: ['dest-mail-ops'],
      dedupe_config: null,
      revision: 1,
      created_at: agoIso(60 * 24 * 9),
      updated_at: agoIso(60 * 24 * 9),
    },
    {
      id: 'rule-image-major',
      name: '重要镜像变更',
      enabled: false,
      priority: 20,
      conditions: {
        operator: 'and',
        conditions: [{ field: 'image', operator: 'contains', value: 'postgres' }],
      },
      template_id: 'tmpl-image-update',
      channel_ids: ['dest-ding-ops'],
      dedupe_config: { window_seconds: 60 },
      revision: 1,
      created_at: agoIso(60 * 24 * 7),
      updated_at: agoIso(60 * 24 * 7),
    },
  ]

  // ── Templates ───────────────────────────────────────────
  const templates: TemplateResponse[] = [
    {
      id: 'tmpl-image-update',
      name: '镜像更新通知',
      format: 'markdown',
      title_template: '镜像更新: {{ .image }}',
      body_template: '检测到新版本\n\n镜像: {{ .image }}\n状态: {{ .status }}\n提供商: {{ .provider }}\n主机: {{ .hostname }}\n摘要: {{ .digest }}\n\n[查看详情]({{ .link }})',
      revision: 2,
      created_at: agoIso(60 * 24 * 13),
      updated_at: agoIso(60 * 24 * 5),
    },
    {
      id: 'tmpl-ci-fail',
      name: 'CI 失败通知',
      format: 'text',
      title_template: '构建失败: {{ .repo }}',
      body_template: '仓库 {{ .repo }}（{{ .branch }}）构建失败\n\n提交者: {{ .author }}\nJob 数量: {{ len .jobs }}',
      revision: 1,
      created_at: agoIso(60 * 24 * 9),
      updated_at: agoIso(60 * 24 * 9),
    },
  ]

  // ── Channels ────────────────────────────────────────
  const channels: ChannelResponse[] = [
    {
      id: 'dest-ding-ops',
      name: '钉钉·运维群',
      type: 'dingtalk',
      enabled: true,
      config: {
        webhook_url: 'https://oapi.dingtalk.com/robot/send?access_token=demo-token',
        msg_type: 'markdown',
      },
      secret: 'SEC-demo-signing-key',
      timeout_ms: 10000,
      max_attempts: 5,
      revision: 2,
      created_at: agoIso(60 * 24 * 13),
      updated_at: agoIso(60 * 24 * 4),
    },
    {
      id: 'dest-feishu-alert',
      name: '飞书·告警群',
      type: 'feishu',
      enabled: true,
      config: {
        webhook_url: 'https://open.feishu.cn/open-apis/bot/v2/hook/demo-token',
        msg_type: 'interactive',
      },
      secret: null,
      timeout_ms: 10000,
      max_attempts: 3,
      revision: 1,
      created_at: agoIso(60 * 24 * 12),
      updated_at: agoIso(60 * 24 * 12),
    },
    {
      id: 'dest-mail-ops',
      name: '邮件·运维组',
      type: 'email',
      enabled: true,
      config: {
        smtp_host: 'smtp.example.com',
        smtp_port: 465,
        use_tls: true,
        from: 'notygo@example.com',
        to: ['ops@example.com'],
      },
      secret: 'demo-smtp-password',
      timeout_ms: 15000,
      max_attempts: 3,
      revision: 1,
      created_at: agoIso(60 * 24 * 9),
      updated_at: agoIso(60 * 24 * 9),
    },
    {
      id: 'dest-http-log',
      name: 'HTTP·日志管道',
      type: 'http',
      enabled: false,
      config: {
        url: 'https://logs.internal.example.com/hook',
        method: 'POST',
        headers: { 'X-Endpoint': 'notygo' },
        body_fields: [{ key: 'title', value_source: 'title' }, { key: 'body', value_source: 'body' }],
      },
      secret: null,
      timeout_ms: 10000,
      max_attempts: 5,
      revision: 1,
      created_at: agoIso(60 * 24 * 8),
      updated_at: agoIso(60 * 24 * 8),
    },
  ]

  // ── Events, runs, deliveries, attempts ──────────────────
  const events: EventResponse[] = []
  const eventRuns: EventRunResponse[] = []
  const deliveries: DeliveryResponse[] = []
  const deliveryAttempts: DeliveryAttemptResponse[] = []

  const DIUN_IMAGES: Array<[string, string, string]> = [
    ['docker.io/library/nginx', '1.27.4', 'https://hub.docker.com/_/nginx'],
    ['docker.io/library/redis', '7.4.2', 'https://hub.docker.com/_/redis'],
    ['docker.io/library/postgres', '17.2', 'https://hub.docker.com/_/postgres'],
    ['docker.io/library/mariadb', '11.7.1', 'https://hub.docker.com/_/mariadb'],
    ['docker.io/bitnami/grafana', '11.1.0', 'https://hub.docker.com/r/bitnami/grafana'],
    ['docker.io/library/alpine', '3.22.1', 'https://hub.docker.com/_/alpine'],
    ['ghcr.io/diun/diun', '4.28.0', 'https://github.com/diun/diun'],
  ]
  // 事件时间从 3 天前到现在递减排布
  const diunTimes = [4320, 3660, 2880, 1980, 1230, 510, 3]

  let evtNo = 0
  let dlyNo = 0
  let attNo = 0

  const addDiunEvent = (
    image: [string, string, string],
    minsAgo: number,
    opts: { deduped?: boolean; failed?: boolean } = {},
  ) => {
    evtNo += 1
    const eventId = `evt-${evtNo.toString().padStart(4, '0')}`
    const payload = opts.failed
      ? JSON.stringify({
          diun_version: '4.28.0',
          hostname: 'homelab-01',
          status: 'new',
          provider: 'file',
          // image missing → required field error
          link: image[2],
          kind: 'image',
        })
      : JSON.stringify({
          diun_version: '4.28.0',
          hostname: 'homelab-01',
          status: 'new',
          provider: 'file',
          image: `${image[0]}:${image[1]}`,
          link: image[2],
          kind: 'image',
          manifest: { digest: `sha256:${hash(image[0] + image[1])}`, created: agoIso(60 * 24 * 30) },
        })

    const event: EventResponse = {
      id: eventId,
      endpoint_id: 'src-diun',
      upstream_event_id: null,
      request_id: reqId(evtNo),
      received_at: agoIso(minsAgo),
      content_type: 'application/json',
      payload_raw: payload,
      payload_hash: hash(payload),
      status: opts.deduped
        ? 'deduped'
        : opts.failed
          ? 'failed'
          : 'completed',
      error_code: opts.failed ? 'PARSE_002' : null,
      error_summary: opts.failed
        ? 'required field "image" not found at path "image"'
        : null,
    }
    events.push(event)
    if (opts.deduped) return event

    const parsed: Record<string, unknown> = opts.failed
      ? { status: 'new', provider: 'file', link: image[2] }
      : {
          image: `${image[0]}:${image[1]}`,
          status: 'new',
          provider: 'file',
          link: image[2],
          digest: `sha256:${hash(image[0] + image[1])}`,
          hostname: 'homelab-01',
        }
    const run: EventRunResponse = {
      id: `run-${evtNo.toString().padStart(4, '0')}`,
      event_id: eventId,
      run_type: 'standard',
      mapping_id: 'map-diun',
      mapping_revision: 2,
      rule_set_snapshot: [
        {
          id: 'rule-image-update',
          name: '镜像有更新',
          priority: 10,
          conditions: { operator: 'and', conditions: [{ field: 'status', operator: 'eq', value: 'new' }] },
          channels: ['dest-ding-ops', 'dest-feishu-alert'],
        },
      ],
      parsed_fields: parsed,
      validation_result: opts.failed ? { valid: false } : { valid: true },
      matched_rule_ids: opts.failed ? [] : ['rule-image-update'],
      status: opts.failed ? 'failed' : 'completed',
      started_at: agoIso(minsAgo - 0.02),
      finished_at: agoIso(minsAgo - 0.05),
      error_code: opts.failed ? 'PARSE_002' : null,
      error_summary: opts.failed ? 'mapping validation failed: required field "image" not found' : null,
    }
    eventRuns.push(run)
    if (opts.failed) return event

    // Rule fan-out: 钉钉 + 飞书
    const renderedTitle = `镜像更新: ${image[0]}:${image[1]}`
    const renderedBody = `检测到新版本\n\n镜像: ${image[0]}:${image[1]}\n状态: new\n提供商: file\n主机: homelab-01\n摘要: sha256:${hash(image[0] + image[1])}\n\n[查看详情](${image[2]})`

    // 钉钉: 正常送达
    addDelivery(eventId, 'dest-ding-ops', minsAgo, renderedTitle, renderedBody, {
      status: 'delivered',
      attempts: [{ status: 200, durationMs: 340 + (evtNo * 53) % 400 }],
    })

    // 飞书: 混合状态——最近的一条 pending，之前有 retry_wait / failed
    return event
  }

  function addDelivery(
    eventId: string,
    channelId: string,
    minsAgo: number,
    renderedTitle: string,
    renderedBody: string,
    spec: {
      status: string
      attempts: Array<{ status: number | null; durationMs: number; errorCode?: string }>
    },
  ) {
    dlyNo += 1
    const deliveryId = `dly-${dlyNo.toString().padStart(4, '0')}`
    const attemptCount = spec.attempts.length
    const lastAttempt = spec.attempts[spec.attempts.length - 1]
    const dest = channels.find((d) => d.id === channelId)!
    const failed = spec.status === 'failed' || spec.status === 'retry_wait'

    deliveries.push({
      id: deliveryId,
      event_id: eventId,
      event_run_id: `run-${evtNo.toString().padStart(4, '0')}`,
      rule_id: 'rule-image-update',
      channel_id: channelId,
      idempotency_key: `${eventId}:${channelId}`,
      status: spec.status,
      attempt_count: attemptCount,
      max_attempts: dest.max_attempts ?? 5,
      next_attempt_at: spec.status === 'retry_wait' ? aheadIso(2) : undefined,
      rendered_title: renderedTitle,
      rendered_body: renderedBody,
      rendered_format: 'markdown',
      last_error_code: failed ? (lastAttempt.errorCode ?? 'DELIVER_001') : null,
      last_error_summary: failed
        ? lastAttempt.status != null
          ? `HTTP ${lastAttempt.status}: ${lastAttempt.errorCode === 'DELIVER_002' ? 'permission denied' : 'rate limited'}`
          : 'dial timeout after 10s'
        : null,
      created_at: agoIso(minsAgo - 0.1),
      updated_at: agoIso(minsAgo - 1),
    })

    let startedAt = minsAgo - 0.1
    spec.attempts.forEach((a, i) => {
      attNo += 1
      deliveryAttempts.push({
        id: `att-${attNo.toString().padStart(4, '0')}`,
        delivery_id: deliveryId,
        attempt_no: i + 1,
        started_at: agoIso(startedAt),
        finished_at: agoIso(startedAt - a.durationMs / 60_000),
        request_summary: `POST ${destUrl(dest)}`,
        response_status: a.status,
        response_summary: a.status === 200 ? 'ok' : a.errorCode ?? null,
        error_code: a.errorCode ?? null,
        error_summary:
          a.status === 200
            ? null
            : a.errorCode === 'DELIVER_002'
              ? `HTTP 403: permission denied`
              : a.errorCode === 'DELIVER_005'
                ? `HTTP 429: rate limited`
                : 'dial timeout after 10s',
        duration_ms: a.durationMs,
      })
      startedAt -= 5 // 退避重试间隔（分钟）
    })
  }

  function destUrl(dest: ChannelResponse): string {
    const url = (dest.config as Record<string, unknown>).webhook_url
    if (typeof url === 'string') return url.replace(/(access_token|hook)=.*$/, '$1=***')
    return (dest.config as Record<string, unknown>).url as string
  }

  for (let i = 0; i < DIUN_IMAGES.length; i++) {
    addDiunEvent(DIUN_IMAGES[i], diunTimes[i])
  }
  // 去重事件: 同一 payload 的重复推送
  addDiunEvent(DIUN_IMAGES[3], diunTimes[3] - 30, { deduped: true })
  // 映射失败事件
  addDiunEvent(DIUN_IMAGES[0], 1500, { failed: true })

  // 飞书渠道的混合投递状态（覆盖前面的正常事件）
  {
    const nginxEvent = events.find((e) => e.payload_raw.includes('nginx:1.27.4'))!
    // postgres 事件 → 飞书投递失败（403, 重试耗尽）
    const pgEvent = events.find((e) => e.payload_raw.includes('postgres:17.2'))!
    // redis 事件 → 飞书 429 限流, 等待重试
    const redisEvent = events.find((e) => e.payload_raw.includes('redis:7.4.2'))!
    // 最新事件（diun 3 分钟前）→ 飞书 pending
    const newestEvent = events.find((e) => e.payload_raw.includes('ghcr.io/diun/diun'))!

    addFeishuDelivery(pgEvent, 'failed', [
      { status: 403, durationMs: 210, errorCode: 'DELIVER_002' },
      { status: 403, durationMs: 198, errorCode: 'DELIVER_002' },
      { status: 403, durationMs: 224, errorCode: 'DELIVER_002' },
    ])
    addFeishuDelivery(redisEvent, 'retry_wait', [
      { status: 429, durationMs: 95, errorCode: 'DELIVER_005' },
    ])
    addFeishuDelivery(newestEvent, 'pending', [])
    addFeishuDelivery(nginxEvent, 'delivered', [{ status: 200, durationMs: 265 }])
    // 其余事件飞书也正常送达
    for (const e of events) {
      if (
        e.status === 'completed' &&
        e.endpoint_id === 'src-diun' &&
        ![nginxEvent.id, pgEvent.id, redisEvent.id, newestEvent.id].includes(e.id) &&
        !deliveries.some((d) => d.event_id === e.id && d.channel_id === 'dest-feishu-alert')
      ) {
        addFeishuDelivery(e, 'delivered', [{ status: 200, durationMs: 250 + (evtNo * 17) % 200 }])
      }
    }
  }

  function addFeishuDelivery(
    event: EventResponse,
    status: string,
    attempts: Array<{ status: number | null; durationMs: number; errorCode?: string }>,
  ) {
    const image = event.payload_raw.match(/"image":"([^"]+)"/)?.[1] ?? ''
    dlyNo += 1
    const deliveryId = `dly-${dlyNo.toString().padStart(4, '0')}`
    const lastAttempt = attempts[attempts.length - 1]
    const failed = status === 'failed' || status === 'retry_wait'
    deliveries.push({
      id: deliveryId,
      event_id: event.id,
      event_run_id: `run-${event.id.slice(4)}`,
      rule_id: 'rule-image-update',
      channel_id: 'dest-feishu-alert',
      idempotency_key: `${event.id}:dest-feishu-alert`,
      status,
      attempt_count: attempts.length,
      max_attempts: 3,
      next_attempt_at: status === 'retry_wait' ? aheadIso(2) : undefined,
      rendered_title: `镜像更新: ${image}`,
      rendered_body: '检测到新版本…',
      rendered_format: 'markdown',
      last_error_code: failed ? (lastAttempt?.errorCode ?? 'DELIVER_001') : null,
      last_error_summary: failed
        ? lastAttempt?.status != null
          ? `HTTP ${lastAttempt.status}: ${lastAttempt.errorCode === 'DELIVER_002' ? 'permission denied' : 'rate limited'}`
          : 'dial timeout after 10s'
        : null,
      created_at: event.received_at,
      updated_at: event.received_at,
    })
    attempts.forEach((a, i) => {
      attNo += 1
      deliveryAttempts.push({
        id: `att-${attNo.toString().padStart(4, '0')}`,
        delivery_id: deliveryId,
        attempt_no: i + 1,
        started_at: event.received_at!,
        finished_at: event.received_at!,
        request_summary: 'POST https://open.feishu.cn/open-apis/bot/v2/hook/***',
        response_status: a.status,
        response_summary: a.status === 200 ? 'ok' : (a.errorCode ?? null),
        error_code: a.errorCode ?? null,
        error_summary:
          a.status === 200
            ? null
            : a.errorCode === 'DELIVER_002'
              ? 'HTTP 403: permission denied'
              : 'HTTP 429: rate limited',
        duration_ms: a.durationMs,
      })
    })
  }

  // CI 事件: 成功几条(不匹配规则), 一条 failure → 邮件投递
  const CI_COMMITS = ['a1b5a3e', '3f35465', 'c462f50', '7b4e082', '3167767', 'f0d2c31']
  const ciTimes = [3610, 2900, 2200, 1400, 700, 45]
  CI_COMMITS.forEach((commit, i) => {
    evtNo += 1
    const eventId = `evt-${evtNo.toString().padStart(4, '0')}`
    const status = i === 2 ? 'failure' : 'success'
    const payload = JSON.stringify({
      repository: 'z1zhang/Notygo',
      branch: i % 2 === 0 ? 'dev' : 'master',
      status,
      actor: 'z1zhang',
      head_commit: commit,
      jobs: [
        { name: 'build', status: 'success' },
        { name: 'test', status: status === 'failure' ? 'failure' : 'success' },
      ],
    })
    events.push({
      id: eventId,
      endpoint_id: 'src-ci',
      upstream_event_id: null,
      request_id: reqId(evtNo),
      received_at: agoIso(ciTimes[i]),
      content_type: 'application/json',
      payload_raw: payload,
      payload_hash: hash(payload),
      status: 'completed',
      error_code: null,
      error_summary: null,
    })
    eventRuns.push({
      id: `run-${evtNo.toString().padStart(4, '0')}`,
      event_id: eventId,
      run_type: 'standard',
      mapping_id: 'map-ci',
      mapping_revision: 1,
      rule_set_snapshot: [
        {
          id: 'rule-build-fail',
          name: '构建失败告警',
          priority: 5,
          conditions: { operator: 'and', conditions: [{ field: 'status', operator: 'eq', value: 'failure' }] },
          channels: ['dest-mail-ops'],
        },
      ],
      parsed_fields: { repo: 'z1zhang/Notygo', branch: i % 2 === 0 ? 'dev' : 'master', status, author: 'z1zhang', job_count: 2, jobs: [ { name: 'build' }, { name: 'test' } ] },
      validation_result: { valid: true },
      matched_rule_ids: status === 'failure' ? ['rule-build-fail'] : [],
      status: 'completed',
      started_at: agoIso(ciTimes[i] - 0.02),
      finished_at: agoIso(ciTimes[i] - 0.05),
      error_code: null,
      error_summary: null,
    })

    if (status === 'failure') {
      dlyNo += 1
      attNo += 1
      deliveries.push({
        id: `dly-${dlyNo.toString().padStart(4, '0')}`,
        event_id: eventId,
        event_run_id: `run-${evtNo.toString().padStart(4, '0')}`,
        rule_id: 'rule-build-fail',
        channel_id: 'dest-mail-ops',
        idempotency_key: `${eventId}:dest-mail-ops`,
        status: 'delivered',
        attempt_count: 1,
        max_attempts: 3,
        next_attempt_at: undefined,
        rendered_title: '构建失败: z1zhang/Notygo',
        rendered_body: '仓库 z1zhang/Notygo（master）构建失败\n\n提交者: z1zhang\nJob 数量: 2',
        rendered_format: 'text',
        last_error_code: null,
        last_error_summary: null,
        created_at: agoIso(ciTimes[i] - 0.1),
        updated_at: agoIso(ciTimes[i] - 0.2),
      })
      deliveryAttempts.push({
        id: `att-${attNo.toString().padStart(4, '0')}`,
        delivery_id: `dly-${dlyNo.toString().padStart(4, '0')}`,
        attempt_no: 1,
        started_at: agoIso(ciTimes[i] - 0.1),
        finished_at: agoIso(ciTimes[i] - 0.12),
        request_summary: 'SMTP smtp.example.com:465 → ops@example.com',
        response_status: 250,
        response_summary: '2.0.0 Ok: queued',
        error_code: null,
        error_summary: null,
        duration_ms: 812,
      })
    }
  })

  // ── Audit logs ──────────────────────────────────────────
  const auditLogs: AuditLogResponse[] = [
    {
      id: 'aud-0001',
      operator: 'admin',
      action: 'login',
      target_type: 'endpoint',
      target_id: '-',
      snapshot_before: null,
      summary: 'admin logged in',
      created_at: agoIso(60 * 24 * 3),
    },
    {
      id: 'aud-0002',
      operator: 'admin',
      action: 'create',
      target_type: 'endpoint',
      target_id: 'src-diun',
      snapshot_before: null,
      summary: 'created endpoint "Diun 镜像监控"',
      created_at: agoIso(60 * 24 * 14),
    },
    {
      id: 'aud-0003',
      operator: 'admin',
      action: 'create',
      target_type: 'channel',
      target_id: 'dest-ding-ops',
      snapshot_before: null,
      summary: 'created channel "钉钉·运维群"',
      created_at: agoIso(60 * 24 * 13),
    },
    {
      id: 'aud-0004',
      operator: 'admin',
      action: 'update',
      target_type: 'rule',
      target_id: 'rule-image-update',
      snapshot_before: '{"priority":5}',
      summary: 'updated rule "镜像有更新"',
      created_at: agoIso(60 * 24 * 5),
    },
    {
      id: 'aud-0005',
      operator: 'admin',
      action: 'replay',
      target_type: 'event',
      target_id: 'evt-0001',
      snapshot_before: null,
      summary: 'replayed event evt-0001',
      created_at: agoIso(60 * 24 * 2),
    },
    {
      id: 'aud-0006',
      operator: 'admin',
      action: 'retry',
      target_type: 'delivery',
      target_id: 'dly-0009',
      snapshot_before: null,
      summary: 'retried delivery dly-0009',
      created_at: agoIso(180),
    },
    {
      id: 'aud-0007',
      operator: 'admin',
      action: 'login',
      target_type: 'endpoint',
      target_id: '-',
      summary: 'admin logged in',
      created_at: agoIso(30),
    },
  ]

  // Sort all logs newest-first
  auditLogs.sort((a, b) => (b.created_at ?? '').localeCompare(a.created_at ?? ''))

  return {
    endpoints,
    mappings,
    rules,
    templates,
    channels,
    events: events.sort((a, b) => (b.received_at ?? '').localeCompare(a.received_at ?? '')),
    eventRuns,
    deliveries: deliveries.sort((a, b) => (b.created_at ?? '').localeCompare(a.created_at ?? '')),
    deliveryAttempts,
    auditLogs,
    operator: 'admin',
  }
}
