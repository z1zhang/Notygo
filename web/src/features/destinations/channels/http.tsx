import { z } from 'zod'
import { useFieldArray, useWatch, type Control } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { Plus, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { asString, type AnyForm, type BodyFieldNode, type ChannelModule } from './types'

export interface HttpFormFields {
  url: string
  method: 'POST' | 'GET' | 'PUT' | 'PATCH'
  headers: { key: string; value: string }[]
  body_fields: BodyFieldNode[]
}

const HTTP_METHODS = ['POST', 'GET', 'PUT', 'PATCH'] as const

// ── Config normalization helpers ───────────────────────────────────────────

/** headers may be an object → convert to a key/value row list. */
function asHeaderRows(value: unknown): { key: string; value: string }[] {
  if (value && typeof value === 'object' && !Array.isArray(value)) {
    return Object.entries(value as Record<string, unknown>).map(([key, v]) => ({
      key,
      value: typeof v === 'string' ? v : String(v ?? ''),
    }))
  }
  return []
}

/** body_fields is a loosely-typed tree on the server side; normalize for the form. */
function asBodyFieldRows(value: unknown): BodyFieldNode[] {
  if (!Array.isArray(value)) return []
  return value.map((v) => {
    const o = (v && typeof v === 'object' && !Array.isArray(v) ? v : {}) as Record<string, unknown>
    const type = o.type === 'array' ? 'array' : o.type === 'object' ? 'object' : ''
    return {
      key: asString(o.key),
      source: type === '' ? asString(o.source) : '',
      value: asString(o.value),
      type,
      children: asBodyFieldRows(o.children),
    }
  })
}

/** Drop incomplete rows (no key / no source / empty group) before submitting. */
function cleanBodyFields(rows: BodyFieldNode[], parentIsArray: boolean): BodyFieldNode[] {
  const out: BodyFieldNode[] = []
  for (const r of rows) {
    const isGroup = r.type === 'object' || r.type === 'array'
    if (isGroup) {
      const children = cleanBodyFields(r.children ?? [], r.type === 'array')
      if (children.length === 0) continue
      if (!parentIsArray && r.key.trim() === '') continue
      out.push({ ...r, children })
    } else {
      if (r.source === '') continue
      if (!parentIsArray && r.key.trim() === '') continue
      out.push(r)
    }
  }
  return out
}

// ── Request-body field tree editor (recursive) ─────────────────────────────
// Paths are dynamic and recursive, which RHF's path types cannot express;
// they are cast at the hooks and boundary-checked by the zod schema.

const bodyFieldSchema: z.ZodType<BodyFieldNode> = z.lazy(() =>
  z.object({
    key: z.string(),
    source: z.string(),
    value: z.string(),
    type: z.string(),
    children: z.array(bodyFieldSchema),
  }),
)

const BODY_SOURCES = [
  { value: 'title', key: 'sourceTitle' },
  { value: 'body', key: 'sourceBody' },
  { value: 'format', key: 'sourceFormat' },
  { value: 'literal', key: 'sourceLiteral' },
] as const

// Radix Select treats an empty-string value as "nothing selected", so the
// plain-field option uses a sentinel that maps back to type "".
const ROW_TYPES = [
  { value: 'field', key: 'typeField' },
  { value: 'object', key: 'groupObject' },
  { value: 'array', key: 'groupArray' },
] as const

type SetFormValue = (path: string, value: unknown) => void

function BodyFieldRows({
  control,
  name,
  parentIsArray,
  setValue,
  label,
}: {
  control: Control<AnyForm>
  name: string
  parentIsArray: boolean
  setValue: SetFormValue
  label?: string
}) {
  const { t } = useTranslation()
  const { fields, append, remove } = useFieldArray({ control, name: name as never })
  const appendRow = (row: BodyFieldNode) => (append as (v: BodyFieldNode) => void)(row)
  const addButton = (
    <Button
      type="button"
      variant="outline"
      size="sm"
      onClick={() => appendRow({ key: '', source: 'body', value: '', type: '', children: [] })}
    >
      <Plus />
      {t('destinations.addField')}
    </Button>
  )

  // Top level renders a label row with the button on the right (matches the
  // custom-headers block); nested group children just right-align the button.
  return (
    <div className="space-y-2">
      {label ? (
        <div className="flex items-center justify-between">
          <Label>{label}</Label>
          {addButton}
        </div>
      ) : null}
      {fields.length === 0 ? (
        <p className="text-xs text-muted-foreground">{t('destinations.noBodyFields')}</p>
      ) : null}
      {fields.map((field, index) => (
        <BodyFieldRow
          key={field.id}
          control={control}
          name={`${name}.${index}`}
          parentIsArray={parentIsArray}
          setValue={setValue}
          onRemove={() => remove(index)}
        />
      ))}
      {label ? null : <div className="flex justify-end">{addButton}</div>}
    </div>
  )
}

function BodyFieldRow({
  control,
  name,
  parentIsArray,
  setValue,
  onRemove,
}: {
  control: Control<AnyForm>
  name: string
  parentIsArray: boolean
  setValue: SetFormValue
  onRemove: () => void
}) {
  const { t } = useTranslation()
  const row = useWatch({ control, name: name as never }) as BodyFieldNode | undefined
  const type = row?.type ?? ''
  const isGroup = type === 'object' || type === 'array'
  const source = row?.source ?? 'body'

  // Switching row type converts in place: leaf → group drops the source,
  // group → leaf drops the children and restores a default source.
  function handleTypeChange(v: string) {
    const nextType = v === 'field' ? '' : v
    setValue(`${name}.type`, nextType)
    if (nextType === '') {
      setValue(`${name}.children`, [])
      if (!source) setValue(`${name}.source`, 'body')
    } else {
      setValue(`${name}.source`, '')
    }
  }

  return (
    <div className={isGroup ? 'space-y-2 rounded-md border p-3' : 'space-y-2'}>
      <div className="flex items-center gap-2">
        {parentIsArray ? (
          <span className="min-w-14 shrink-0 text-sm text-muted-foreground">
            {t('destinations.arrayItem')}
          </span>
        ) : (
          <Input
            placeholder={t('destinations.fieldKey')}
            value={row?.key ?? ''}
            onChange={(e) => setValue(`${name}.key`, e.target.value)}
          />
        )}
        <Select value={type || 'field'} onValueChange={handleTypeChange}>
          <SelectTrigger className="w-24 shrink-0">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {ROW_TYPES.map((rt) => (
              <SelectItem key={rt.value} value={rt.value}>
                {t(`destinations.${rt.key}`)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {!isGroup ? (
          <>
            <Select value={source} onValueChange={(v) => setValue(`${name}.source`, v)}>
              <SelectTrigger className="w-32 shrink-0">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {BODY_SOURCES.map((s) => (
                  <SelectItem key={s.value} value={s.value}>
                    {t(`destinations.${s.key}`)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {source === 'literal' ? (
              <Input
                placeholder={t('destinations.fieldValue')}
                value={row?.value ?? ''}
                onChange={(e) => setValue(`${name}.value`, e.target.value)}
              />
            ) : null}
          </>
        ) : null}
        <Button
          type="button"
          variant="outline"
          size="icon-sm"
          className="ml-auto shrink-0"
          onClick={onRemove}
          aria-label={t('common.remove')}
        >
          <X className="size-4" />
        </Button>
      </div>
      {isGroup ? (
        <div className="ml-4 border-l pl-3">
          <BodyFieldRows
            control={control}
            name={`${name}.children`}
            parentIsArray={type === 'array'}
            setValue={setValue}
          />
        </div>
      ) : null}
    </div>
  )
}

function HttpWebhookFields({ form }: { form: AnyForm }) {
  const { t } = useTranslation()
  const { fields: headerFields, append: appendHeader, remove: removeHeader } = useFieldArray({
    control: form.control,
    name: 'headers',
  })
  const errors = form.formState.errors
  // Bridge for the recursive body-field editor: RHF's path types cannot
  // express dynamic recursive paths, so the setter is loosened here.
  const setBodyValue: SetFormValue = (path, value) =>
    (form.setValue as SetFormValue)(path, value)

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-[1fr_140px] gap-4">
        <div className="space-y-2">
          <Label htmlFor="destination-url">{t('destinations.targetUrl')}</Label>
          <Input id="destination-url" placeholder="https://..." {...form.register('url')} />
          {errors.url ? (
            <p className="text-xs text-destructive">{t('destinations.urlRequired')}</p>
          ) : null}
        </div>
        <div className="space-y-2">
          <Label>{t('destinations.httpMethod')}</Label>
          <Select
            value={form.watch('method')}
            onValueChange={(v) => form.setValue('method', v)}
          >
            <SelectTrigger className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {HTTP_METHODS.map((method) => (
                <SelectItem key={method} value={method}>
                  {method}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>

      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <Label>{t('destinations.headers')}</Label>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => appendHeader({ key: '', value: '' })}
          >
            <Plus />
            {t('destinations.addHeader')}
          </Button>
        </div>
        {headerFields.length === 0 ? (
          <p className="text-xs text-muted-foreground">{t('destinations.noHeaders')}</p>
        ) : null}
        <div className="space-y-2">
          {headerFields.map((field, index) => (
            <div key={field.id} className="flex items-center gap-2">
              <Input
                placeholder={t('destinations.headerKey')}
                {...form.register(`headers.${index}.key`)}
              />
              <Input
                placeholder={t('destinations.headerValue')}
                {...form.register(`headers.${index}.value`)}
              />
              <Button
                type="button"
                variant="outline"
                size="icon-sm"
                onClick={() => removeHeader(index)}
                aria-label={t('common.remove')}
              >
                <X className="size-4" />
              </Button>
            </div>
          ))}
        </div>
      </div>

      <div className="space-y-2">
        <BodyFieldRows
          control={form.control}
          name="body_fields"
          parentIsArray={false}
          setValue={setBodyValue}
          label={t('destinations.bodyFields')}
        />
      </div>
    </div>
  )
}

export const httpModule: ChannelModule = {
  types: ['http'],
  formSchema: {
    url: z.string(),
    method: z.enum(HTTP_METHODS),
    headers: z.array(z.object({ key: z.string(), value: z.string() })),
    body_fields: z.array(bodyFieldSchema),
  },
  defaults: { url: '', method: 'POST', headers: [], body_fields: [] },
  Fields: HttpWebhookFields,
  toConfig: (data) => {
    const config: Record<string, unknown> = { url: data.url, method: data.method }
    const headers = Object.fromEntries(
      data.headers
        .filter((h: { key: string }) => h.key !== '')
        .map((h: { key: string; value: string }) => [h.key, h.value]),
    )
    if (Object.keys(headers).length > 0) config.headers = headers
    // Structured body tree: each field maps a JSON key (nestable via
    // groups) to a value source. Server-side json.Marshal handles
    // all escaping.
    const fields = cleanBodyFields(data.body_fields ?? [], false)
    if (fields.length > 0) config.body_fields = fields
    return config
  },
  fromConfig: (config) => ({
    url: asString(config.url),
    method: HTTP_METHODS.includes(asString(config.method) as (typeof HTTP_METHODS)[number])
      ? asString(config.method)
      : 'POST',
    headers: asHeaderRows(config.headers),
    body_fields: asBodyFieldRows(config.body_fields),
  }),
  validate: (data, ctx) => {
    if (data.type === 'http' && !(data.url ?? '').trim()) {
      ctx.addIssue({ code: 'custom', path: ['url'], message: 'urlRequired' })
    }
  },
}
