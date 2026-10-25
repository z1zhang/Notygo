// Demo-mode implementations of the backend preview endpoints:
//   - POST /preview/parse     → analyzePayload
//   - POST /preview/mapping   → applyMapping
//   - POST /preview/template  → renderTemplate
// These mirror the Go logic (internal/api/handlers_preview.go, internal/mapping,
// internal/templates) for the common subset: dot paths with '#' array
// semantics, multi_result_mode, to_upper/to_lower/trim transforms, and Go
// templates with field access, upper/lower/trim/title/len/default and range.
import { DemoHttpError } from './mode'
import type {
  PreviewParseResponse,
  ValidationErrorItem,
} from '../types'

// ── Path lookup (gjson subset) ───────────────────────────

/**
 * Resolve a payload_path against a parsed payload. Supported: plain dot paths
 * ("a.b.c"), numeric array indexes ("a.0.b"), "#" (root array) and
 * "#.field" (collect the field from every array element).
 */
function getPath(root: unknown, path: string): { exists: boolean; value: unknown } {
  if (path === '#' || path === '') {
    return { exists: true, value: root }
  }
  const segments = path.split('.')
  if (segments[0] === '#') {
    if (!Array.isArray(root)) return { exists: false, value: undefined }
    segments.shift()
    const values = root
      .map((item) => walk(item, segments))
      .filter((v) => v !== undefined)
    return { exists: values.length > 0, value: values }
  }
  const value = walk(root, segments)
  return { exists: value !== undefined, value }
}

function walk(node: unknown, segments: string[]): unknown {
  let current = node
  for (const seg of segments) {
    if (current === null || current === undefined) return undefined
    if (Array.isArray(current)) {
      const idx = Number(seg)
      if (!Number.isInteger(idx)) return undefined
      current = current[idx]
      continue
    }
    if (typeof current !== 'object') return undefined
    current = (current as Record<string, unknown>)[seg]
  }
  return current
}

// ── POST /preview/mapping ────────────────────────────────

export interface DemoMappingEntry {
  payload_path: string
  target_key: string
  value_type?: string | null
  default_value?: string | null
  required?: boolean | null
  transform?: string | null
  multi_result_mode?: string | null
}

interface EntryOutcome {
  val: unknown
  errors: ValidationErrorItem[]
}

export function applyMapping(
  payloadRaw: string,
  fields: DemoMappingEntry[],
): { parsed_fields: Record<string, unknown>; validation_errors: ValidationErrorItem[] } {
  const root = parsePayload(payloadRaw)
  const parsed: Record<string, unknown> = {}
  const errors: ValidationErrorItem[] = []

  for (const entry of fields) {
    const { val, errors: errs } = applyEntry(entry, root)
    errors.push(...errs)
    if (val !== undefined) parsed[entry.target_key] = val
  }
  return { parsed_fields: parsed, validation_errors: errors }
}

function applyEntry(entry: DemoMappingEntry, root: unknown): EntryOutcome {
  const errors: ValidationErrorItem[] = []
  const { exists, value } = getPath(root, entry.payload_path ?? '')

  // Missing or null → default value, or required error
  if (!exists || value === null) {
    if (entry.default_value != null) {
      return { val: typedDefault(entry, errors), errors }
    }
    if (entry.required) {
      errors.push({
        field: entry.target_key,
        code: 'PARSE_002',
        detail: `required field "${entry.target_key}" not found at path "${entry.payload_path}"`,
      })
    }
    return { val: undefined, errors }
  }

  // Array result → multi_result_mode
  let val: unknown
  if (Array.isArray(value)) {
    val = applyMultiResultMode(entry, value, errors)
  } else {
    val = value
  }

  val = applyTransform(entry, val)

  if (typeMismatch(val, entry.value_type ?? '')) {
    errors.push({
      field: entry.target_key,
      code: 'PARSE_003',
      detail: `type mismatch for field "${entry.target_key}": expected ${entry.value_type}`,
    })
  }
  if (entry.required && isZero(val)) {
    errors.push({
      field: entry.target_key,
      code: 'PARSE_002',
      detail: `required field "${entry.target_key}" has zero value`,
    })
  }
  return { val, errors }
}

function applyMultiResultMode(
  entry: DemoMappingEntry,
  arr: unknown[],
  errors: ValidationErrorItem[],
): unknown {
  const mode = entry.multi_result_mode ?? ''

  if (mode === 'first') {
    return arr.length > 0 ? arr[0] : (entry.default_value ?? undefined)
  }
  if (mode === 'array') return [...arr]
  if (mode === 'count') return arr.length
  if (mode.startsWith('join(') && mode.endsWith(')')) {
    return arr.map((v) => toGoString(v)).join(mode.slice(5, -1))
  }
  if (mode === '') {
    // No mode: arrays stay whole for "array" value_type, else first element
    if (entry.value_type === 'array') return [...arr]
    return arr.length > 0 ? arr[0] : undefined
  }
  errors.push({
    field: entry.target_key,
    code: 'PARSE_004',
    detail: `unknown multi_result_mode "${mode}" for field "${entry.target_key}"`,
  })
  return undefined
}

function applyTransform(entry: DemoMappingEntry, val: unknown): unknown {
  if (typeof val !== 'string') return val
  switch (entry.transform ?? '') {
    case 'to_upper': return val.toUpperCase()
    case 'to_lower': return val.toLowerCase()
    case 'trim': return val.trim()
    default: return val
  }
}

function typeMismatch(val: unknown, valueType: string): boolean {
  switch (valueType) {
    case '': return false
    case 'string': return typeof val !== 'string'
    case 'number': return typeof val !== 'number'
    case 'boolean': return typeof val !== 'boolean'
    case 'object': return typeof val !== 'object' || val === null || Array.isArray(val)
    case 'array': return !Array.isArray(val)
    default: return false
  }
}

function typedDefault(entry: DemoMappingEntry, errors: ValidationErrorItem[]): unknown {
  const raw = entry.default_value ?? ''
  switch (entry.value_type) {
    case 'number': {
      const n = Number(raw)
      if (Number.isNaN(n)) {
        errors.push({
          field: entry.target_key,
          code: 'PARSE_003',
          detail: `cannot convert default value "${raw}" to number`,
        })
        return raw
      }
      return n
    }
    case 'boolean': {
      const lower = raw.toLowerCase()
      if (lower === 'true' || lower === '1') return true
      if (lower === 'false' || lower === '0') return false
      errors.push({
        field: entry.target_key,
        code: 'PARSE_003',
        detail: `cannot convert default value "${raw}" to boolean`,
      })
      return raw
    }
    default:
      return raw
  }
}

function isZero(val: unknown): boolean {
  if (val === null || val === undefined) return true
  if (typeof val === 'string') return val === ''
  if (typeof val === 'boolean') return !val
  if (typeof val === 'number') return val === 0
  if (Array.isArray(val)) return val.length === 0
  if (typeof val === 'object') return Object.keys(val).length === 0
  return false
}

// ── POST /preview/parse ──────────────────────────────────

export function analyzePayload(payloadRaw: string): PreviewParseResponse {
  const root = parsePayload(payloadRaw)

  if (Array.isArray(root)) {
    const fields = [
      {
        payload_path: '#',
        target_key: 'items',
        value_type: 'array',
        multi_result_mode: 'array',
        sample_value: `[${root.length} items]`,
      },
      {
        payload_path: '#',
        target_key: 'item_count',
        value_type: 'number',
        multi_result_mode: 'count',
        sample_value: String(root.length),
      },
    ]
    if (isPlainObject(root[0])) {
      walkArrayObject(root, root[0], '#', 1, fields)
    }
    return {
      structure: { type: 'array', root_is_array: true, item_count: root.length },
      suggested_fields: fields,
    }
  }

  if (isPlainObject(root)) {
    return {
      structure: {
        type: 'object',
        root_is_array: false,
        item_count: Object.keys(root).length,
      },
      suggested_fields: analyzeObject(root, '', 1),
    }
  }

  throw new DemoHttpError(400, 'CONFIG_001', 'payload root must be a JSON object or array')
}

type SuggestedField = PreviewParseResponse['suggested_fields'][number]

function walkArrayObject(
  root: unknown[],
  firstObj: Record<string, unknown>,
  pathPrefix: string,
  depth: number,
  fields: SuggestedField[],
) {
  if (depth > 3) return
  for (const [key, val] of Object.entries(firstObj)) {
    const fieldPath = `${pathPrefix}.${key}`
    if (isPlainObject(val)) {
      walkArrayObject(root, val, fieldPath, depth + 1, fields)
      continue
    }
    if (Array.isArray(val)) {
      fields.push({
        payload_path: fieldPath,
        target_key: autoTargetKey(fieldPath),
        value_type: 'array',
        multi_result_mode: 'array',
        sample_value: `[${val.length} items]`,
      })
      continue
    }
    // Scalar: "#.field" collects the field from every element, so the
    // sample comes back as an array — mirror the backend's mode inference.
    const vt = inferType(val)
    const matches = root
      .map((item) => (isPlainObject(item) ? item[key] : undefined))
      .filter((v) => v !== undefined)
    if (vt === 'string') {
      fields.push({
        payload_path: fieldPath,
        target_key: autoTargetKey(fieldPath),
        value_type: 'string',
        multi_result_mode: 'join(, )',
        sample_value: matches.map((v) => toGoString(v)).join(', '),
      })
    } else {
      fields.push({
        payload_path: fieldPath,
        target_key: autoTargetKey(fieldPath),
        value_type: 'array',
        multi_result_mode: 'array',
        sample_value: `[${matches.length} items]`,
      })
    }
  }
}

function analyzeObject(
  obj: Record<string, unknown>,
  pathPrefix: string,
  depth: number,
): SuggestedField[] {
  if (depth > 3) return []
  const fields: SuggestedField[] = []
  for (const [key, val] of Object.entries(obj)) {
    const fieldPath = pathPrefix ? `${pathPrefix}.${key}` : key
    if (isPlainObject(val)) {
      fields.push(...analyzeObject(val, fieldPath, depth + 1))
      continue
    }
    if (Array.isArray(val)) {
      // Scalar arrays suggest join (string); object arrays stay array
      if (val.length > 0 && !isPlainObject(val[0]) && !Array.isArray(val[0])) {
        fields.push({
          payload_path: fieldPath,
          target_key: autoTargetKey(fieldPath),
          value_type: 'string',
          multi_result_mode: 'join(, )',
          sample_value: val.map((v) => toGoString(v)).join(', '),
        })
      } else {
        fields.push({
          payload_path: fieldPath,
          target_key: autoTargetKey(fieldPath),
          value_type: 'array',
          multi_result_mode: 'array',
          sample_value: `[${val.length} items]`,
        })
      }
      continue
    }
    fields.push({
      payload_path: fieldPath,
      target_key: autoTargetKey(fieldPath),
      value_type: inferType(val),
      multi_result_mode: null,
      sample_value: toGoString(val),
    })
  }
  return fields
}

function inferType(val: unknown): string {
  if (val === null) return 'string'
  if (typeof val === 'string') return 'string'
  if (typeof val === 'number') return 'number'
  if (typeof val === 'boolean') return 'boolean'
  if (Array.isArray(val)) return 'array'
  return 'object'
}

// "image.name" → "image_name", "#.name" → "names" (mirror of autoTargetKey)
function autoTargetKey(payloadPath: string): string {
  const s = payloadPath.replace(/^#/, '').replace(/^\./, '')
  if (s === '') return 'value'
  const parts = s.split('.')
  if (parts.length === 1) return parts[0]

  let leaf = parts[parts.length - 1]
  if (/(s|x|ch|sh)$/.test(leaf)) {
    leaf += 'es'
  } else if (/[^aeiou]y$/.test(leaf) && leaf.length > 1) {
    leaf = leaf.slice(0, -1) + 'ies'
  } else {
    leaf += 's'
  }
  if (payloadPath.startsWith('#')) {
    return `${parts[parts.length - 2]}_${leaf}`
  }
  return leaf
}

// ── Go template rendering (safe subset) ──────────────────

type Node =
  | { type: 'text'; value: string }
  | { type: 'action'; pipeline: string }
  | { type: 'range'; pipeline: string; body: Node[] }

/** Render a Go text/template subset: {{ .field }}, funcs, {{ range }}/{{ end }}, trim markers. */
export function renderTemplate(template: string, data: Record<string, unknown>): string {
  const nodes = parseTemplate(template)
  return execNodes(nodes, data, data)
}

function parseTemplate(template: string): Node[] {
  const re = /\{\{(-?)\s*([\s\S]*?)\s*(-?)\}\}/g
  const parts: { text?: string; action?: string }[] = []
  let last = 0
  let trimNext = false
  let m: RegExpExecArray | null
  while ((m = re.exec(template)) !== null) {
    let text = template.slice(last, m.index)
    if (m[1] === '-') text = text.replace(/\s+$/, '')
    if (trimNext) text = text.replace(/^\s+/, '')
    if (text !== '') parts.push({ text })
    trimNext = m[3] === '-'
    parts.push({ action: m[2] })
    last = re.lastIndex
  }
  let tail = template.slice(last)
  if (trimNext) tail = tail.replace(/^\s+/, '')
  if (tail !== '') parts.push({ text: tail })

  const root: Node[] = []
  const stack: Node[][] = []
  let current = root
  for (const part of parts) {
    if (part.text !== undefined) {
      current.push({ type: 'text', value: part.text })
      continue
    }
    const action = part.action ?? ''
    const verb = action.split(/\s+/)[0]
    if (verb === 'range') {
      const block: Node = {
        type: 'range',
        pipeline: action.replace(/^range\s+/, ''),
        body: [],
      }
      current.push(block)
      stack.push(current)
      current = block.body
    } else if (verb === 'end') {
      const frame = stack.pop()
      if (frame === undefined) throw new Error('unexpected {{ end }}')
      current = frame
    } else {
      current.push({ type: 'action', pipeline: action })
    }
  }
  if (stack.length > 0) throw new Error('missing {{ end }}')
  return root
}

function execNodes(nodes: Node[], data: Record<string, unknown>, dot: unknown): string {
  let out = ''
  for (const node of nodes) {
    if (node.type === 'text') {
      out += node.value
    } else if (node.type === 'action') {
      out += toGoString(evalPipeline(node.pipeline, dot))
    } else {
      const list = evalPipeline(node.pipeline, dot)
      const items = Array.isArray(list) ? list : list == null ? [] : [list]
      for (const item of items) {
        out += execNodes(node.body, data, item)
      }
    }
  }
  return out
}

const TEMPLATE_FUNCS = new Set(['upper', 'lower', 'trim', 'title', 'len', 'default'])

function evalPipeline(pipeline: string, dot: unknown): unknown {
  const tokens = pipeline.match(/"(?:[^"\\]|\\.)*"|\S+/g) ?? []
  const head = tokens[0] ?? ''
  if (head.includes('|')) throw new Error(`pipeline "|" is not supported in demo mode`)

  if (TEMPLATE_FUNCS.has(head)) {
    const args = tokens.slice(1).map((t) => evalArg(t, dot))
    return applyFunc(head, args)
  }
  if (tokens.length > 1) throw new Error(`unknown function "${head}"`)
  return evalArg(head, dot)
}

function evalArg(token: string, dot: unknown): unknown {
  if (token === '') return dot
  if (token === '.') return dot
  if (token.startsWith('"')) {
    return token
      .slice(1, -1)
      .replace(/\\n/g, '\n')
      .replace(/\\t/g, '\t')
      .replace(/\\"/g, '"')
      .replace(/\\\\/g, '\\')
  }
  if (token.startsWith('.')) {
    return resolveField(token.slice(1), dot)
  }
  if (/^-?\d+(\.\d+)?$/.test(token)) return Number(token)
  throw new Error(`unknown function "${token}"`)
}

// Field access relative to the current dot; missing keys render <no value>
function resolveField(path: string, dot: unknown): unknown {
  const value = walk(dot, path.split('.'))
  if (value === undefined) return null // prints "<no value>"
  return value
}

function applyFunc(fn: string, args: unknown[]): unknown {
  switch (fn) {
    case 'upper': return toGoString(args[0]).toUpperCase()
    case 'lower': return toGoString(args[0]).toLowerCase()
    case 'trim': return toGoString(args[0]).trim()
    case 'title':
      return toGoString(args[0])
        .split(/\s+/)
        .map((w) => (w.length > 0 ? w[0].toUpperCase() + w.slice(1) : w))
        .join(' ')
    case 'len': {
      const v = args[0]
      if (typeof v === 'string' || Array.isArray(v)) return v.length
      if (isPlainObject(v)) return Object.keys(v).length
      throw new Error(`len: unsupported type ${typeof v}`)
    }
    case 'default':
      return args[1] === null || args[1] === undefined || args[1] === '' ? args[0] : args[1]
    default:
      throw new Error(`unknown function "${fn}"`)
  }
}

// Go %v-style stringification
function toGoString(val: unknown): string {
  if (val === null || val === undefined) return '<no value>'
  if (typeof val === 'string') return val
  if (typeof val === 'number' || typeof val === 'boolean') return String(val)
  if (Array.isArray(val)) return `[${val.map((v) => toGoString(v)).join(' ')}]`
  if (typeof val === 'object') {
    const entries = Object.entries(val as Record<string, unknown>)
      .map(([k, v]) => `${k}:${toGoString(v)}`)
      .join(' ')
    return `map[${entries}]`
  }
  return String(val)
}

// ── helpers ──────────────────────────────────────────────

function parsePayload(payloadRaw: string): unknown {
  try {
    return JSON.parse(payloadRaw)
  } catch {
    throw new DemoHttpError(400, 'CONFIG_001', 'payload is not valid JSON')
  }
}

function isPlainObject(val: unknown): val is Record<string, unknown> {
  return typeof val === 'object' && val !== null && !Array.isArray(val)
}
