// Throwaway smoke test for the demo-mode logic (run with:
//   node --experimental-strip-types scripts/demo-smoke.mts )
import { renderTemplate, applyMapping, analyzePayload } from '../src/lib/demo/preview.ts'
import { buildSeed } from '../src/lib/demo/seed.ts'

let failed = 0
function check(name: string, actual: unknown, expected: unknown) {
  const ok = JSON.stringify(actual) === JSON.stringify(expected)
  console.log(`${ok ? 'PASS' : 'FAIL'} ${name}${ok ? '' : `\n  actual:   ${JSON.stringify(actual)}\n  expected: ${JSON.stringify(expected)}`}`)
  if (!ok) failed++
}

// ── renderTemplate ───────────────────────────────────────
check('field access + trim', renderTemplate('{{ .image }} / {{ .status }}', { image: 'nginx:1.27', status: 'new' }), 'nginx:1.27 / new')
check('funcs', renderTemplate('{{ upper .s }} {{ title .s }} {{ len .arr }}', { s: 'hello world', arr: [1, 2, 3] }), 'HELLO WORLD Hello World 3')
check('default', renderTemplate('{{ default "none" .missing }}', { x: 1 }), 'none')
check('missing key', renderTemplate('{{ .missing }}', { x: 1 }), '<no value>')
check('range + len', renderTemplate('{{ len .names }}\n{{ range .names }}• {{ . }}\n{{ end }}', { names: ['a', 'b'] }), '2\n• a\n• b\n')
check('trim markers', renderTemplate('a\n{{- .x -}}\nb', { x: 'X' }), 'aXb')
check('nested path', renderTemplate('{{ .manifest.digest }}', { manifest: { digest: 'sha256:abc' } }), 'sha256:abc')

// ── applyMapping ─────────────────────────────────────────
const diunPayload = JSON.stringify({
  hostname: 'lab', status: 'new', image: 'nginx:1.27', provider: 'file',
  manifest: { digest: 'sha256:abc' },
})
const fields = [
  { source_path: 'image', target_key: 'image', value_type: 'string', required: true },
  { source_path: 'manifest.digest', target_key: 'digest', value_type: 'string' },
  { source_path: 'status', target_key: 'status', value_type: 'string' },
]
const mapped = applyMapping(diunPayload, fields as never)
check('mapping basic', mapped.parsed_fields, { image: 'nginx:1.27', digest: 'sha256:abc', status: 'new' })
check('mapping no errors', mapped.validation_errors, [])

const missing = applyMapping(JSON.stringify({ status: 'new' }), [
  { source_path: 'image', target_key: 'image', value_type: 'string', required: true },
] as never)
check('mapping required error', missing.validation_errors[0]?.code, 'PARSE_002')

const multi = applyMapping(diunPayload, [
  { source_path: 'status', target_key: 's', value_type: 'string', transform: 'to_upper' },
] as never)
check('transform', multi.parsed_fields.s, 'NEW')

// root array + count + join
const arrPayload = JSON.stringify([{ name: 'a' }, { name: 'b' }])
const arrMapped = applyMapping(arrPayload, [
  { source_path: '#', target_key: 'items', value_type: 'array', multi_result_mode: 'array' },
  { source_path: '#', target_key: 'count', value_type: 'number', multi_result_mode: 'count' },
  { source_path: '#.name', target_key: 'names', value_type: 'string', multi_result_mode: 'join(, )' },
] as never)
check('root array mapping', arrMapped.parsed_fields, { items: [{ name: 'a' }, { name: 'b' }], count: 2, names: 'a, b' })

// ── analyzePayload ───────────────────────────────────────
const parsed = analyzePayload(arrPayload)
check('array structure', parsed.structure, { type: 'array', root_is_array: true, item_count: 2 })
check('array field paths', parsed.suggested_fields.map((f) => f.source_path), ['#', '#', '#.name'])

const objParsed = analyzePayload(diunPayload)
check('object structure', objParsed.structure, { type: 'object', root_is_array: false, item_count: 5 })
check('object suggestions', objParsed.suggested_fields.map((f) => `${f.source_path}:${f.target_key}`), [
  'hostname:hostname', 'status:status', 'image:image', 'provider:provider', 'manifest.digest:digests',
])

// ── buildSeed consistency ─────────────────────────────────
const seed = buildSeed()
check('seed sources', seed.sources.length, 2)
check('seed events count', seed.events.length, 15)
// every completed event has a run; every delivery's event exists
const eventIds = new Set(seed.events.map((e) => e.id))
check('delivery events exist', seed.deliveries.every((d) => eventIds.has(d.event_id)), true)
const destIds = new Set(seed.destinations.map((d) => d.id))
check('delivery dests exist', seed.deliveries.every((d) => destIds.has(d.destination_id)), true)
// every completed diun event fanned out to both rule destinations
const completedDiun = seed.events.filter((e) => e.source_id === 'src-diun' && e.status === 'completed')
check('fanout coverage', completedDiun.every(
  (e) => seed.deliveries.filter((d) => d.event_id === e.id).length === 2,
), true)
// event detail shape: latest_run resolvable
const withRuns = seed.events.filter((e) => seed.eventRuns.some((r) => r.event_id === e.id))
check('runs exist for processed events', withRuns.length, 14)
// statuses present for the demo
check('status mix', [
  seed.events.some((e) => e.status === 'failed'),
  seed.events.some((e) => e.status === 'deduped'),
  seed.deliveries.some((d) => d.status === 'delivered'),
  seed.deliveries.some((d) => d.status === 'failed'),
  seed.deliveries.some((d) => d.status === 'retry_wait'),
  seed.deliveries.some((d) => d.status === 'pending'),
], [true, true, true, true, true, true])

// template from seed renders against its own parsed fields
const tmpl = seed.templates[0]
const run = seed.eventRuns.find((r) => r.event_id === seed.events.find((e) => e.source_id === 'src-diun' && e.status === 'completed')!.id)!
const title = renderTemplate(tmpl.title_template, run.parsed_fields)
check('seed template renders', title.startsWith('镜像更新: '), true)
console.log('  sample:', title)

// ── handler routing (with localStorage stub) ─────────────
const mem = new Map<string, string>()
;(globalThis as unknown as { localStorage: Storage }).localStorage = {
  getItem: (k) => mem.get(k) ?? null,
  setItem: (k, v) => void mem.set(k, String(v)),
  removeItem: (k) => void mem.delete(k),
  clear: () => void mem.clear(),
  key: () => null,
  length: 0,
} as Storage
const { demoRequest } = await import('../src/lib/demo/handler.ts')

const login = await demoRequest<{ token: string }>('POST', '/api/v1/auth/login', { username: 'demo', password: 'x' })
check('login token', typeof login.token === 'string' && login.token.startsWith('demo-'), true)

const list = await demoRequest<{ data: unknown[]; total: number }>('GET', '/api/v1/destinations', undefined, { page: 1, size: 20 })
check('dest list', [list.total, list.data.length], [4, 4])

const evtDetail = await demoRequest<{ event: { id: string }; latest_run: { matched_rule_ids: string[] } }>('GET', '/api/v1/events/evt-0001', undefined, {})
check('event detail', [evtDetail.event.id, evtDetail.latest_run?.matched_rule_ids?.length ?? 0], ['evt-0001', 1])

const dlyList = await demoRequest<{ total: number }>('GET', '/api/v1/deliveries', undefined, { status: 'failed' })
check('delivery filter', dlyList.total, 1)

const stats = await demoRequest<{ total_events: number; failed_deliveries: number }>('GET', '/api/v1/dashboard/stats')
check('dashboard', [stats.total_events, stats.failed_deliveries], [15, 1])

const created = await demoRequest<{ id: string; slug: string; revision: number }>('POST', '/api/v1/sources', {
  name: 'test source', auth_type: 'none', mapping_id: null, enabled: true,
})
check('create source', [created.slug.length, created.revision], [32, 1])

const updated = await demoRequest<{ enabled: boolean; revision: number }>('PUT', `/api/v1/sources/${created.id}`, { enabled: false })
check('update source', [updated.enabled, updated.revision], [false, 2])

const removed = await demoRequest<void>('DELETE', `/api/v1/sources/${created.id}`)
check('delete source', removed, undefined)

const failedList = await demoRequest<{ data: Array<{ id: string }> }>('GET', '/api/v1/deliveries', undefined, { status: 'failed' })
const retried = await demoRequest<{ delivery: { status: string }; message: string }>('POST', `/api/v1/deliveries/${failedList.data[0].id}/retry`)
check('retry delivery', retried.delivery.status, 'delivered')

const replayed = await demoRequest<{ event_id: string }>('POST', '/api/v1/events/evt-0002/replay')
check('replay event', replayed.event_id, 'evt-0002')

const auditList = await demoRequest<{ total: number; data: Array<{ action: string }> }>('GET', '/api/v1/audit', undefined, { page: 1, size: 10 })
check('audit grew', auditList.total > 7, true)
check('audit newest is replay', auditList.data[0]?.action, 'replay')

const prevParse = await demoRequest<{ suggested_fields: unknown[] }>('POST', '/api/v1/preview/parse', { payload: arrPayload })
check('preview parse', prevParse.suggested_fields.length, 3)

const prevTmpl = await demoRequest<{ title: string }>('POST', '/api/v1/preview/template', {
  title_template: 'hi {{ upper .name }}', sample_data: { name: 'bob' },
})
check('preview template', prevTmpl.title, 'hi BOB')

process.exit(failed > 0 ? 1 : 0)
