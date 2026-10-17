import { useMemo, useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Check, CirclePlus, Loader2, Plus, Search, Trash2, Wand2, Eye, Waypoints } from 'lucide-react'
import { useI18n } from '@/i18n'
import {
  createMapping,
  createTemplate,
  previewDelivery,
  previewMapping,
  previewParse,
} from '@/lib/api'
import { getErrorMessage } from '@/lib/api-client'
import type { MappingEntry, SuggestedField, StructureInfo } from '@/lib/types'
import { PageHeader } from '@/components/layout/PageHeader'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { JsonViewer } from '@/components/data-display/JsonViewer'
import { RenderedBody } from './RenderedBody'
import { cn } from '@/lib/utils'

const VALUE_TYPES = ['string', 'number', 'boolean', 'object', 'array'] as const
const MULTI_MODES = ['first', 'join(, )', 'array', 'count'] as const
const FORMATS = ['text', 'markdown', 'html'] as const

interface FieldRow {
  source_path: string
  target_key: string
  value_type: string
  multi_result_mode: string | null
  default_value: string
  _sample_value: string
}

interface TemplateForm {
  format: string
  title_template: string
  body_template: string
}

const emptyField: FieldRow = {
  source_path: '',
  target_key: '',
  value_type: 'string',
  multi_result_mode: null,
  default_value: '',
  _sample_value: '',
}

export default function PlaygroundPage() {
  const { t } = useI18n()
  const queryClient = useQueryClient()

  // ── Section 1: raw payload ─────────────────────────────
  const [rawPayload, setRawPayload] = useState('')
  const [structureInfo, setStructureInfo] = useState<StructureInfo | null>(null)
  const [suggestedFields, setSuggestedFields] = useState<SuggestedField[]>([])

  // ── Section 2: mapping fields ──────────────────────────
  const [mappingFields, setMappingFields] = useState<FieldRow[]>([])
  const [mappingResult, setMappingResult] = useState<Record<string, unknown> | null>(null)
  const [mappingErrors, setMappingErrors] = useState<
    { field: string; code: string; detail: string }[]
  >([])

  // ── Section 3: template / render ───────────────────────
  const [templateForm, setTemplateForm] = useState<TemplateForm>({
    format: 'markdown',
    title_template: '',
    body_template: '',
  })
  const [deliveryResult, setDeliveryResult] = useState<{
    rendered_title: string
    rendered_body: string
    format: string
  } | null>(null)
  const [deliveryErrors, setDeliveryErrors] = useState<
    { field: string; code: string; detail: string }[]
  >([])

  // ── Create dialogs ─────────────────────────────────────
  const [mappingDialogOpen, setMappingDialogOpen] = useState(false)
  const [mappingName, setMappingName] = useState('')
  const [templateDialogOpen, setTemplateDialogOpen] = useState(false)
  const [templateName, setTemplateName] = useState('')

  // Track which source_path+mode combos are already added
  const addedKeys = useMemo(() => {
    const s = new Set<string>()
    for (const f of mappingFields) {
      s.add(f.source_path + (f.multi_result_mode ?? ''))
    }
    return s
  }, [mappingFields])

  function buildMappingEntries(): MappingEntry[] {
    return mappingFields
      .filter((f) => f.source_path !== '' || f.target_key !== '')
      .map((f) => ({
        source_path: f.source_path,
        target_key: f.target_key,
        value_type: f.value_type || 'string',
        multi_result_mode: f.multi_result_mode || null,
        default_value: f.default_value || null,
        required: false,
      }))
  }

  function requirePayload(): boolean {
    if (rawPayload.trim() === '') {
      toast.warning(t('playground.payloadRequired'))
      return false
    }
    return true
  }

  // ── Mutations ──────────────────────────────────────────

  const parseMutation = useMutation({
    mutationFn: () => previewParse(rawPayload),
    onSuccess: (res) => {
      setStructureInfo(res.structure)
      setSuggestedFields(res.suggested_fields ?? [])
      // Clear previous mapping/delivery results
      setMappingResult(null)
      setMappingErrors([])
      setDeliveryResult(null)
      setDeliveryErrors([])
      toast.success(t('playground.analyzed'))
    },
    onError: (e) => {
      toast.error(getErrorMessage(e, t('common.error')))
    },
  })

  const mappingMutation = useMutation({
    mutationFn: (fields: MappingEntry[]) => previewMapping(rawPayload, fields),
    onSuccess: (res) => {
      setMappingResult(res.parsed_fields)
      setMappingErrors(res.validation_errors ?? [])
    },
    onError: (e) => {
      toast.error(getErrorMessage(e, t('common.error')))
    },
  })

  const deliveryMutation = useMutation({
    mutationFn: (args: { fields: MappingEntry[]; template: TemplateForm }) =>
      previewDelivery(rawPayload, args.fields, {
        title_template: args.template.title_template,
        body_template: args.template.body_template,
        format: args.template.format,
      }),
    onSuccess: (res) => {
      setDeliveryResult({
        rendered_title: res.rendered_title,
        rendered_body: res.rendered_body,
        format: res.format,
      })
      setDeliveryErrors(res.validation_errors ?? [])
    },
    onError: (e) => {
      toast.error(getErrorMessage(e, t('common.error')))
    },
  })

  const createMappingMutation = useMutation({
    mutationFn: (name: string) => createMapping({ name, fields: buildMappingEntries() }),
    onSuccess: () => {
      toast.success(t('playground.mappingCreated'))
      setMappingDialogOpen(false)
      setMappingName('')
      void queryClient.invalidateQueries({ queryKey: ['mappings'] })
      void queryClient.invalidateQueries({ queryKey: ['lookup', 'mappings'] })
    },
    onError: (e) => {
      toast.error(getErrorMessage(e, t('common.failedToSave', { entity: t('nav.mappings') })))
    },
  })

  const createTemplateMutation = useMutation({
    mutationFn: (name: string) =>
      createTemplate({
        name,
        format: templateForm.format,
        title_template: templateForm.title_template,
        body_template: templateForm.body_template,
      }),
    onSuccess: () => {
      toast.success(t('playground.templateCreated'))
      setTemplateDialogOpen(false)
      setTemplateName('')
      void queryClient.invalidateQueries({ queryKey: ['templates'] })
      void queryClient.invalidateQueries({ queryKey: ['lookup', 'templates'] })
    },
    onError: (e) => {
      toast.error(getErrorMessage(e, t('common.failedToSave', { entity: t('nav.templates') })))
    },
  })

  // ── Handlers ───────────────────────────────────────────

  function handleAnalyze() {
    if (!requirePayload()) return
    try {
      JSON.parse(rawPayload)
    } catch {
      toast.error(t('playground.invalidPayloadJson'))
      return
    }
    setStructureInfo(null)
    setSuggestedFields([])
    parseMutation.mutate()
  }

  function handleApplyMapping(): Promise<boolean> {
    if (!requirePayload()) return Promise.resolve(false)
    const fields = buildMappingEntries()
    if (fields.length === 0) {
      toast.warning(t('playground.noFields'))
      return Promise.resolve(false)
    }
    setMappingResult(null)
    setMappingErrors([])
    return mappingMutation
      .mutateAsync(fields)
      .then((res) => (res.validation_errors ?? []).length === 0)
      .catch(() => false)
  }

  function handleRenderPreview() {
    if (!requirePayload()) return
    const fields = buildMappingEntries()
    if (fields.length === 0) {
      toast.warning(t('playground.noFields'))
      return
    }
    setDeliveryResult(null)
    setDeliveryErrors([])
    deliveryMutation.mutate({ fields, template: templateForm })
  }

  async function handleFullPipeline() {
    const ok = await handleApplyMapping()
    if (!ok) return
    const fields = buildMappingEntries()
    setDeliveryResult(null)
    setDeliveryErrors([])
    deliveryMutation.mutate({ fields, template: templateForm })
  }

  function addField() {
    setMappingFields((prev) => [...prev, { ...emptyField }])
  }

  function removeField(index: number) {
    setMappingFields((prev) => prev.filter((_, i) => i !== index))
  }

  function updateField(index: number, patch: Partial<FieldRow>) {
    setMappingFields((prev) =>
      prev.map((f, i) => (i === index ? { ...f, ...patch } : f)),
    )
  }

  function addFieldFromSuggestion(sf: SuggestedField) {
    setMappingFields((prev) => [
      ...prev,
      {
        source_path: sf.source_path,
        target_key: sf.target_key,
        value_type: sf.value_type || 'string',
        multi_result_mode: sf.multi_result_mode || null,
        default_value: '',
        _sample_value: sf.sample_value || '',
      },
    ])
  }

  const templateEmpty =
    templateForm.title_template === '' && templateForm.body_template === ''

  return (
    <div className="space-y-4">
      <PageHeader title={t('nav.playground')} />

      {/* Section 1: Raw payload */}
      <section className="rounded-lg border">
        <div className="flex items-center justify-between gap-2 border-b px-4 py-3">
          <h2 className="text-sm font-semibold">{t('playground.payload')}</h2>
          {structureInfo ? (
            <div className="flex items-center gap-2">
              <Badge
                className={
                  structureInfo.root_is_array
                    ? 'bg-amber-500/10 text-amber-700 dark:text-amber-400'
                    : 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-400'
                }
              >
                {structureInfo.root_is_array
                  ? t('playground.rootArray')
                  : t('playground.rootObject')}
              </Badge>
              <Badge variant="secondary" className="tabular-nums">
                {t('playground.itemCount')}: {structureInfo.item_count}
              </Badge>
            </div>
          ) : null}
        </div>
        <div className="space-y-3 p-4">
          <Textarea
            rows={8}
            value={rawPayload}
            onChange={(e) => setRawPayload(e.target.value)}
            placeholder={t('playground.payloadPlaceholder')}
            spellCheck={false}
            className="font-mono text-xs"
          />
          <Button onClick={handleAnalyze} disabled={parseMutation.isPending}>
            {parseMutation.isPending ? (
              <Loader2 className="animate-spin" />
            ) : (
              <Search />
            )}
            {t('playground.analyzeJson')}
          </Button>
        </div>
      </section>

      {/* Section 2: structure + mapping fields */}
      {structureInfo ? (
        <section className="rounded-lg border">
          <div className="border-b px-4 py-3">
            <h2 className="text-sm font-semibold">
              {t('playground.mappingFields')} / {t('playground.extractionResults')}
            </h2>
          </div>

          <div className="grid gap-4 p-4 lg:grid-cols-5">
            {/* Left: structure / suggested fields */}
            <div className="space-y-2 lg:col-span-2">
              <h3 className="text-xs font-semibold text-muted-foreground uppercase">
                {t('playground.structure')}
              </h3>
              <div className="max-h-[500px] overflow-y-auto rounded-md border bg-muted/20">
                {suggestedFields.map((sf) => {
                  const added = addedKeys.has(sf.source_path + (sf.multi_result_mode ?? ''))
                  return (
                    <div
                      key={sf.source_path + (sf.multi_result_mode ?? '')}
                      className={cn(
                        'flex items-center gap-2 border-b px-3 py-2 last:border-b-0 hover:bg-muted/50',
                        added && 'opacity-50',
                      )}
                    >
                      <span className="min-w-0 shrink-0 font-mono text-xs font-medium text-primary">
                        {sf.source_path}
                      </span>
                      <Badge variant="secondary" className="shrink-0">
                        {t(`mappings.${valueTypeKey(sf.value_type)}`, sf.value_type)}
                      </Badge>
                      <span className="min-w-0 flex-1 truncate text-xs text-muted-foreground">
                        {sf.sample_value}
                      </span>
                      {added ? (
                        <span className="flex size-6 shrink-0 items-center justify-center text-emerald-500">
                          <Check className="size-3.5" />
                        </span>
                      ) : (
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          className="shrink-0"
                          aria-label={t('playground.addField')}
                          onClick={() => addFieldFromSuggestion(sf)}
                        >
                          <Plus />
                        </Button>
                      )}
                    </div>
                  )
                })}
              </div>
            </div>

            {/* Right: mapping field cards */}
            <div className="space-y-3 lg:col-span-3">
              <div className="flex items-center justify-between">
                <h3 className="text-xs font-semibold text-muted-foreground uppercase">
                  {t('playground.mappingFields')} ({mappingFields.length})
                </h3>
                <Button variant="outline" size="sm" onClick={addField}>
                  <Plus />
                  {t('playground.addField')}
                </Button>
              </div>

              {mappingFields.length === 0 ? (
                <p className="rounded-md border border-dashed p-6 text-center text-sm text-muted-foreground">
                  {t('playground.clickToAdd')}
                </p>
              ) : (
                mappingFields.map((field, index) => (
                  <div key={index} className="space-y-3 rounded-md border bg-muted/20 p-3">
                    <div className="flex items-center justify-between">
                      <span className="text-sm font-medium">
                        {t('mappings.fieldNo', { n: index + 1 })}
                      </span>
                      <Button
                        variant="ghost"
                        size="sm"
                        className="text-destructive hover:text-destructive"
                        onClick={() => removeField(index)}
                      >
                        <Trash2 />
                        {t('common.remove')}
                      </Button>
                    </div>

                    <div className="grid gap-3 sm:grid-cols-2">
                      <div className="space-y-1.5">
                        <Label htmlFor={`field-source-${index}`} className="text-xs">
                          {t('playground.sourcePath')}
                        </Label>
                        <Input
                          id={`field-source-${index}`}
                          value={field.source_path}
                          onChange={(e) => updateField(index, { source_path: e.target.value })}
                          placeholder={t('mappings.sourcePathPlaceholder')}
                          className="font-mono text-xs"
                        />
                      </div>
                      <div className="space-y-1.5">
                        <Label htmlFor={`field-target-${index}`} className="text-xs">
                          {t('playground.targetKey')}
                        </Label>
                        <Input
                          id={`field-target-${index}`}
                          value={field.target_key}
                          onChange={(e) => updateField(index, { target_key: e.target.value })}
                          placeholder={t('mappings.targetKeyPlaceholder')}
                          className="font-mono text-xs"
                        />
                      </div>
                    </div>

                    <div className="grid gap-3 sm:grid-cols-3">
                      <div className="space-y-1.5">
                        <Label className="text-xs">{t('playground.valueType')}</Label>
                        <Select
                          value={field.value_type || 'string'}
                          onValueChange={(v) => updateField(index, { value_type: v })}
                        >
                          <SelectTrigger size="sm">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {VALUE_TYPES.map((vt) => (
                              <SelectItem key={vt} value={vt}>
                                {t(`mappings.${valueTypeKey(vt)}`, vt)}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>
                      <div className="space-y-1.5">
                        <Label className="text-xs">{t('playground.multiResultMode')}</Label>
                        <Select
                          value={field.multi_result_mode ?? 'none'}
                          onValueChange={(v) =>
                            updateField(index, { multi_result_mode: v === 'none' ? null : v })
                          }
                        >
                          <SelectTrigger size="sm">
                            <SelectValue placeholder={t('mappings.selectMode')} />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="none">-</SelectItem>
                            {MULTI_MODES.map((m) => (
                              <SelectItem key={m} value={m}>
                                {t(`mappings.${multiModeKey(m)}`, m)}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>
                      <div className="space-y-1.5">
                        <Label htmlFor={`field-default-${index}`} className="text-xs">
                          {t('playground.defaultValue')}
                        </Label>
                        <Input
                          id={`field-default-${index}`}
                          value={field.default_value}
                          onChange={(e) => updateField(index, { default_value: e.target.value })}
                          placeholder={t('mappings.optionalDefault')}
                          className="text-xs"
                        />
                      </div>
                    </div>

                    {field._sample_value !== '' ? (
                      <div>
                        <Badge variant="secondary" className="max-w-full">
                          <span className="truncate">
                            {t('playground.sampleValue')}: {field._sample_value}
                          </span>
                        </Badge>
                      </div>
                    ) : null}
                  </div>
                ))
              )}
            </div>
          </div>

          <div className="flex flex-wrap gap-2 border-t px-4 py-3">
            <Button
              onClick={() => void handleApplyMapping()}
              disabled={mappingMutation.isPending || mappingFields.length === 0}
            >
              {mappingMutation.isPending ? (
                <Loader2 className="animate-spin" />
              ) : (
                <Waypoints />
              )}
              {t('playground.applyMapping')}
            </Button>
            <Button
              variant="outline"
              disabled={createMappingMutation.isPending || mappingFields.length === 0}
              onClick={() => {
                setMappingName('')
                setMappingDialogOpen(true)
              }}
            >
              <CirclePlus />
              {t('playground.createMapping')}
            </Button>
          </div>

          {/* Extraction results */}
          {mappingResult !== null || mappingErrors.length > 0 ? (
            <div className="space-y-3 border-t px-4 py-3">
              <h3 className="text-xs font-semibold text-muted-foreground uppercase">
                {t('playground.extractionResults')}
              </h3>
              {mappingErrors.length > 0 ? <ValidationErrors errors={mappingErrors} /> : null}
              {mappingResult !== null ? (
                <JsonViewer
                  value={JSON.stringify(mappingResult, null, 2)}
                  maxHeight={320}
                />
              ) : null}
            </div>
          ) : null}
        </section>
      ) : null}

      {/* Section 3: template editor + render preview */}
      <section className="rounded-lg border">
        <div className="border-b px-4 py-3">
          <h2 className="text-sm font-semibold">
            {t('playground.templateEditor')} / {t('playground.renderResult')}
          </h2>
        </div>

        <div className="grid gap-4 p-4 lg:grid-cols-2">
          {/* Left: template editor */}
          <div className="space-y-3">
            <div className="space-y-1.5">
              <Label className="text-xs">{t('templates.format')}</Label>
              <Select
                value={templateForm.format}
                onValueChange={(v) =>
                  setTemplateForm((prev) => ({ ...prev, format: v }))
                }
              >
                <SelectTrigger className="w-40">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {FORMATS.map((f) => (
                    <SelectItem key={f} value={f}>
                      {t(`enums.templateFormat.${f}`)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="pg-title" className="text-xs">
                {t('templates.titleTemplate')}
              </Label>
              <Textarea
                id="pg-title"
                rows={3}
                value={templateForm.title_template}
                onChange={(e) =>
                  setTemplateForm((prev) => ({ ...prev, title_template: e.target.value }))
                }
                placeholder={t('templates.titlePlaceholder')}
                spellCheck={false}
                className="font-mono text-xs"
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="pg-body" className="text-xs">
                {t('templates.bodyTemplate')}
              </Label>
              <Textarea
                id="pg-body"
                rows={10}
                value={templateForm.body_template}
                onChange={(e) =>
                  setTemplateForm((prev) => ({ ...prev, body_template: e.target.value }))
                }
                placeholder={t('templates.bodyPlaceholder')}
                spellCheck={false}
                className="font-mono text-xs"
              />
            </div>
          </div>

          {/* Right: render preview */}
          <div className="space-y-3">
            <h3 className="text-xs font-semibold text-muted-foreground uppercase">
              {t('playground.renderResult')}
            </h3>
            {deliveryErrors.length > 0 ? <ValidationErrors errors={deliveryErrors} /> : null}
            {deliveryResult ? (
              <div className="space-y-3">
                <div className="space-y-1.5">
                  <h4 className="text-xs font-medium text-muted-foreground">
                    {t('templates.title')}
                  </h4>
                  <div className="rounded-md border bg-muted/40 p-3 text-sm whitespace-pre-wrap break-words">
                    {deliveryResult.rendered_title}
                  </div>
                </div>
                <div className="space-y-1.5">
                  <h4 className="text-xs font-medium text-muted-foreground">
                    {t('templates.body')}
                  </h4>
                  <RenderedBody
                    body={deliveryResult.rendered_body}
                    format={deliveryResult.format}
                  />
                </div>
              </div>
            ) : deliveryErrors.length === 0 ? (
              <p className="rounded-md border border-dashed p-6 text-center text-sm text-muted-foreground">
                {t('playground.noResults')}
              </p>
            ) : null}
          </div>
        </div>

        <div className="flex flex-wrap gap-2 border-t px-4 py-3">
          <Button
            onClick={handleRenderPreview}
            disabled={deliveryMutation.isPending || mappingFields.length === 0}
          >
            {deliveryMutation.isPending ? <Loader2 className="animate-spin" /> : <Eye />}
            {t('playground.renderPreview')}
          </Button>
          <Button
            variant="outline"
            onClick={() => void handleFullPipeline()}
            disabled={deliveryMutation.isPending || mappingMutation.isPending || mappingFields.length === 0}
          >
            {deliveryMutation.isPending ? <Loader2 className="animate-spin" /> : <Wand2 />}
            {t('playground.fullPipeline')}
          </Button>
          <Button
            variant="outline"
            disabled={templateEmpty || createTemplateMutation.isPending}
            onClick={() => {
              setTemplateName('')
              setTemplateDialogOpen(true)
            }}
          >
            <CirclePlus />
            {t('playground.createTemplate')}
          </Button>
        </div>
      </section>

      {/* Create mapping dialog */}
      <Dialog
        open={mappingDialogOpen}
        onOpenChange={(open) => {
          if (!createMappingMutation.isPending) setMappingDialogOpen(open)
        }}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t('playground.createMapping')}</DialogTitle>
            <DialogDescription className="sr-only">
              {t('playground.mappingNamePrompt')}
            </DialogDescription>
          </DialogHeader>
          <form
            className="space-y-4"
            onSubmit={(e) => {
              e.preventDefault()
              if (mappingName.trim() === '') {
                toast.warning(t('mappings.nameRequired'))
                return
              }
              createMappingMutation.mutate(mappingName.trim())
            }}
          >
            <div className="space-y-2">
              <Label htmlFor="pg-mapping-name">{t('playground.mappingName')}</Label>
              <Input
                id="pg-mapping-name"
                value={mappingName}
                onChange={(e) => setMappingName(e.target.value)}
                placeholder={t('playground.mappingNamePrompt')}
                autoFocus
              />
            </div>
            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                onClick={() => setMappingDialogOpen(false)}
                disabled={createMappingMutation.isPending}
              >
                {t('common.cancel')}
              </Button>
              <Button type="submit" disabled={createMappingMutation.isPending}>
                {createMappingMutation.isPending ? (
                  <Loader2 className="animate-spin" />
                ) : null}
                {t('common.confirm')}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Create template dialog */}
      <Dialog
        open={templateDialogOpen}
        onOpenChange={(open) => {
          if (!createTemplateMutation.isPending) setTemplateDialogOpen(open)
        }}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t('playground.createTemplate')}</DialogTitle>
            <DialogDescription className="sr-only">
              {t('playground.templateNamePrompt')}
            </DialogDescription>
          </DialogHeader>
          <form
            className="space-y-4"
            onSubmit={(e) => {
              e.preventDefault()
              if (templateName.trim() === '') {
                toast.warning(t('templates.nameRequired'))
                return
              }
              createTemplateMutation.mutate(templateName.trim())
            }}
          >
            <div className="space-y-2">
              <Label htmlFor="pg-template-name">{t('playground.templateName')}</Label>
              <Input
                id="pg-template-name"
                value={templateName}
                onChange={(e) => setTemplateName(e.target.value)}
                placeholder={t('playground.templateNamePrompt')}
                autoFocus
              />
            </div>
            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                onClick={() => setTemplateDialogOpen(false)}
                disabled={createTemplateMutation.isPending}
              >
                {t('common.cancel')}
              </Button>
              <Button type="submit" disabled={createTemplateMutation.isPending}>
                {createTemplateMutation.isPending ? (
                  <Loader2 className="animate-spin" />
                ) : null}
                {t('common.confirm')}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  )
}

function ValidationErrors({
  errors,
}: {
  errors: { field: string; code: string; detail: string }[]
}) {
  const { t } = useI18n()
  return (
    <div className="space-y-2">
      {errors.map((err, i) => (
        <div
          key={i}
          role="alert"
          className="space-y-0.5 rounded-md border border-amber-500/50 bg-amber-500/10 p-3"
        >
          <p className="text-sm font-medium text-amber-700 dark:text-amber-400">
            {t('playground.validationErrors')}: {err.field}: {err.code}
          </p>
          {err.detail ? (
            <p className="text-xs text-muted-foreground break-words">{err.detail}</p>
          ) : null}
        </div>
      ))}
    </div>
  )
}

function valueTypeKey(vt: string): string {
  switch (vt) {
    case 'string':
      return 'string'
    case 'number':
      return 'number'
    case 'boolean':
      return 'boolean'
    case 'object':
      return 'object'
    case 'array':
      return 'array'
    default:
      return 'string'
  }
}

function multiModeKey(mode: string): string {
  switch (mode) {
    case 'first':
      return 'first'
    case 'join(, )':
      return 'joinComma'
    case 'array':
      return 'array'
    case 'count':
      return 'count'
    default:
      return 'selectMode'
  }
}
