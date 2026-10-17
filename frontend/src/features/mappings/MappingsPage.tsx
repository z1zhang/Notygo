import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { toast } from 'sonner'
import { Braces, Check, ChevronDown, Loader2, Pencil, Plus, Search, Sparkles, Trash2 } from 'lucide-react'
import { useI18n } from '@/i18n'
import { createMapping, deleteMapping, listMappings, previewParse, updateMapping } from '@/lib/api'
import { getErrorMessage } from '@/lib/api-client'
import type { MappingResponse, SuggestedField, StructureInfo } from '@/lib/types'
import { lookupName, useRuleOptions } from '@/hooks/use-lookup'
import { usePagination } from '@/hooks/use-pagination'
import { PageHeader } from '@/components/layout/PageHeader'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'
import { Switch } from '@/components/ui/switch'
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
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Pagination } from '@/components/data-display/Pagination'
import { EmptyState } from '@/components/feedback/EmptyState'
import { ErrorState } from '@/components/feedback/ErrorState'
import { TableSkeleton } from '@/components/data-display/TableSkeleton'
import { ConfirmDialog } from '@/components/feedback/ConfirmDialog'
import { RefreshButton } from '@/components/feedback/RefreshButton'
import { MultiSelect } from '@/components/form/MultiSelect'
import { cn } from '@/lib/utils'

// Must match the backend mapper's canonical value_type set (internal/mapping/mapper.go)
const VALUE_TYPES = ['string', 'number', 'boolean', 'object', 'array'] as const

/** Legacy frontend values sent before the list was aligned with the backend. */
function normalizeValueType(vt: string | null | undefined): string {
  if (vt === 'bool') return 'boolean'
  if (vt === 'json') return 'object'
  return vt || 'string'
}
const MULTI_MODES = ['first', 'join(, )', 'array', 'count'] as const
const TRANSFORMS = ['to_lower', 'to_upper', 'trim'] as const

const mappingSchema = z.object({
  name: z.string().min(1),
  continue_on_match: z.boolean(),
})

type MappingForm = z.infer<typeof mappingSchema>

interface FieldRow {
  source_path: string
  target_key: string
  value_type: string
  default_value: string
  required: boolean
  transform: string
  multi_result_mode: string
  description: string
}

type FieldErrors = Record<number, { source_path?: boolean; target_key?: boolean }>

function createEmptyField(): FieldRow {
  return {
    source_path: '',
    target_key: '',
    value_type: 'string',
    default_value: '',
    required: false,
    transform: '',
    multi_result_mode: 'first',
    description: '',
  }
}

/** 'first' is the default multi-result mode — equivalent to null/empty. */
function suggestionKey(sf: SuggestedField): string {
  const mode = sf.multi_result_mode && sf.multi_result_mode !== 'first' ? sf.multi_result_mode : ''
  return sf.source_path + mode
}

function valueTypeBadgeVariant(valueType: string): 'secondary' | 'outline' | 'default' | 'destructive' {
  switch (valueType) {
    case 'string':
      return 'secondary'
    case 'number':
      return 'outline'
    case 'boolean':
      return 'default'
    case 'array':
      return 'destructive'
    default:
      return 'secondary'
  }
}

export default function MappingsPage() {
  const { t } = useI18n()
  const queryClient = useQueryClient()
  const { page, size, setPage, setSize } = usePagination()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<MappingResponse | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<MappingResponse | null>(null)

  // Fields + rule_ids managed as plain state (spec allows either RHF arrays or useState)
  const [ruleIds, setRuleIds] = useState<string[]>([])
  const [fields, setFields] = useState<FieldRow[]>([])
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({})

  // Smart Import state
  const [smartOpen, setSmartOpen] = useState(false)
  const [smartJson, setSmartJson] = useState('')
  const [structure, setStructure] = useState<StructureInfo | null>(null)
  const [suggestions, setSuggestions] = useState<SuggestedField[]>([])

  const listQuery = useQuery({
    queryKey: ['mappings', page, size],
    queryFn: () => listMappings({ page, size }),
  })
  const ruleOptions = useRuleOptions()

  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: ['mappings'] })
    void queryClient.invalidateQueries({ queryKey: ['lookup', 'mappings'] })
  }

  const form = useForm<MappingForm>({
    resolver: zodResolver(mappingSchema),
    defaultValues: { name: '', continue_on_match: false },
  })

  function buildPayloadFields() {
    return fields
      .filter((f) => f.source_path || f.target_key)
      .map((f) => ({
        source_path: f.source_path,
        target_key: f.target_key,
        value_type: f.value_type || 'string',
        default_value: f.default_value || null,
        required: f.required,
        transform: f.transform || null,
        multi_result_mode: f.multi_result_mode || null,
        description: f.description || null,
      }))
  }

  const saveMutation = useMutation({
    mutationFn: (values: MappingForm) => {
      const payload: Record<string, unknown> = {
        name: values.name,
        rule_ids: ruleIds,
        continue_on_match: values.continue_on_match,
        fields: buildPayloadFields(),
      }
      if (editing) {
        return updateMapping(editing.id, payload)
      }
      return createMapping(payload)
    },
    onSuccess: () => {
      toast.success(editing ? t('mappings.mappingUpdated') : t('mappings.mappingCreated'))
      setDialogOpen(false)
      invalidate()
    },
    onError: (e) => {
      toast.error(getErrorMessage(e, t('mappings.failedToSaveMapping')))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteMapping(id),
    onSuccess: () => {
      toast.success(t('mappings.mappingDeleted'))
      setDeleteTarget(null)
      invalidate()
    },
    onError: (e) => {
      toast.error(getErrorMessage(e, t('common.failedToSave', { entity: t('nav.mappings') })))
    },
  })

  const analyzeMutation = useMutation({
    mutationFn: (payload: string) => previewParse(payload),
    onSuccess: (res) => {
      setStructure(res.structure)
      setSuggestions(res.suggested_fields ?? [])
    },
    onError: (e) => {
      toast.error(getErrorMessage(e, t('common.error')))
    },
  })

  function resetSmartImport() {
    setSmartOpen(false)
    setSmartJson('')
    setStructure(null)
    setSuggestions([])
  }

  function openCreate() {
    setEditing(null)
    form.reset({ name: '', continue_on_match: false })
    setRuleIds([])
    setFields([])
    setFieldErrors({})
    resetSmartImport()
    setDialogOpen(true)
  }

  function openEdit(row: MappingResponse) {
    setEditing(row)
    form.reset({
      name: row.name ?? '',
      continue_on_match: row.continue_on_match ?? false,
    })
    setRuleIds((row.rule_ids ?? []) as string[])
    setFields(
      (row.fields ?? []).map((f) => ({
        source_path: f.source_path,
        target_key: f.target_key,
        value_type: normalizeValueType(f.value_type),
        default_value: f.default_value ?? '',
        required: f.required,
        transform: f.transform ?? '',
        multi_result_mode: f.multi_result_mode ?? 'first',
        description: f.description ?? '',
      })),
    )
    setFieldErrors({})
    resetSmartImport()
    setDialogOpen(true)
  }

  function updateField(index: number, patch: Partial<FieldRow>) {
    setFields((prev) => prev.map((f, i) => (i === index ? { ...f, ...patch } : f)))
    setFieldErrors((prev) => {
      if (!prev[index]) return prev
      const next = { ...prev }
      if (patch.source_path !== undefined || patch.target_key !== undefined) {
        const entry = { ...next[index] }
        if (patch.source_path !== undefined) delete entry.source_path
        if (patch.target_key !== undefined) delete entry.target_key
        if (entry.source_path || entry.target_key) {
          next[index] = entry
        } else {
          delete next[index]
        }
      }
      return next
    })
  }

  function addField() {
    setFields((prev) => [...prev, createEmptyField()])
  }

  function removeField(index: number) {
    setFields((prev) => prev.filter((_, i) => i !== index))
    setFieldErrors((prev) => {
      const next: FieldErrors = {}
      for (const [key, value] of Object.entries(prev)) {
        const i = Number(key)
        if (i < index) next[i] = value
        else if (i > index) next[i - 1] = value
      }
      return next
    })
  }

  function addFromSuggestion(sf: SuggestedField) {
    setFields((prev) => [
      ...prev,
      {
        source_path: sf.source_path,
        target_key: sf.target_key,
        value_type: normalizeValueType(sf.value_type),
        default_value: '',
        required: false,
        transform: '',
        multi_result_mode: sf.multi_result_mode || 'first',
        description: '',
      },
    ])
  }

  function addAllSuggestions() {
    const existing = new Set(fields.map((f) => {
      const mode = f.multi_result_mode && f.multi_result_mode !== 'first' ? f.multi_result_mode : ''
      return f.source_path + mode
    }))
    const toAdd = suggestions.filter((sf) => !existing.has(suggestionKey(sf)))
    if (toAdd.length === 0) return
    setFields((prev) => [
      ...prev,
      ...toAdd.map((sf) => ({
        source_path: sf.source_path,
        target_key: sf.target_key,
        value_type: normalizeValueType(sf.value_type),
        default_value: '',
        required: false,
        transform: '',
        multi_result_mode: sf.multi_result_mode || 'first',
        description: '',
      })),
    ])
  }

  function handleAnalyze() {
    const raw = smartJson.trim()
    if (!raw) return
    try {
      JSON.parse(raw)
    } catch {
      toast.error(t('templates.invalidJson'))
      return
    }
    analyzeMutation.mutate(raw)
  }

  function onSubmit(values: MappingForm) {
    // Rows that are partially filled must be completed before saving
    const errs: FieldErrors = {}
    fields.forEach((f, i) => {
      const hasSource = f.source_path.trim() !== ''
      const hasTarget = f.target_key.trim() !== ''
      if ((hasSource || hasTarget) && (!hasSource || !hasTarget)) {
        errs[i] = { source_path: !hasSource || undefined, target_key: !hasTarget || undefined }
      }
    })
    if (Object.keys(errs).length > 0) {
      setFieldErrors(errs)
      return
    }
    setFieldErrors({})
    saveMutation.mutate(values)
  }

  // Track which source_path+mode combos are already added as fields
  const addedKeys = useMemo(() => {
    const s = new Set<string>()
    for (const f of fields) {
      const mode = f.multi_result_mode && f.multi_result_mode !== 'first' ? f.multi_result_mode : ''
      s.add(f.source_path + mode)
    }
    return s
  }, [fields])

  const rows = listQuery.data?.data ?? []
  const total = listQuery.data?.total ?? 0

  return (
    <div className="space-y-4">
      <PageHeader
        title={t('nav.mappings')}
        description={t('mappings.description')}
        actions={
          <>
            <RefreshButton
              label={t('dashboard.refresh')}
              onClick={() => void listQuery.refetch()}
              loading={listQuery.isFetching}
            />
            <Button size="sm" onClick={openCreate}>
              <Plus />
              {t('mappings.newMapping')}
            </Button>
          </>
        }
      />

      <div className="rounded-lg border">
        {listQuery.isPending ? (
          <TableSkeleton cols={5} />
        ) : listQuery.isError ? (
          <ErrorState
            error={listQuery.error}
            onRetry={() => void listQuery.refetch()}
            entity={t('nav.mappings')}
          />
        ) : rows.length === 0 ? (
          <EmptyState
            title={t('mappings.emptyTitle')}
            description={t('mappings.emptyDescription')}
            icon={<Braces className="size-5" />}
            action={
              <Button size="sm" variant="outline" onClick={openCreate}>
                <Plus />
                {t('mappings.newMapping')}
              </Button>
            }
          />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('common.name')}</TableHead>
                <TableHead className="w-24">{t('mappings.revision')}</TableHead>
                <TableHead className="w-24">{t('mappings.fieldsCount')}</TableHead>
                <TableHead className="min-w-40">{t('mappings.rules')}</TableHead>
                <TableHead className="w-24 text-right">{t('common.actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((row) => {
                const ruleIdList = (row.rule_ids ?? []) as string[]
                return (
                  <TableRow key={row.id}>
                    <TableCell className="text-muted-foreground">{row.name}</TableCell>
                    <TableCell className="text-muted-foreground">{row.revision}</TableCell>
                    <TableCell>
                      <Badge variant="secondary">{row.fields?.length ?? 0}</Badge>
                    </TableCell>
                    <TableCell>
                      {ruleIdList.length > 0 ? (
                        <span className="inline-flex flex-wrap items-center gap-1">
                          {ruleIdList.slice(0, 3).map((rId) => (
                            <Badge key={rId} variant="outline" className="max-w-40">
                              <span className="truncate">{lookupName(ruleOptions.data, rId)}</span>
                            </Badge>
                          ))}
                          {ruleIdList.length > 3 ? (
                            <Badge variant="secondary">+{ruleIdList.length - 3}</Badge>
                          ) : null}
                        </span>
                      ) : (
                        <span className="text-muted-foreground">-</span>
                      )}
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-1">
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          onClick={() => openEdit(row)}
                          aria-label={t('common.edit')}
                        >
                          <Pencil />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          className="text-destructive hover:text-destructive"
                          onClick={() => setDeleteTarget(row)}
                          aria-label={t('common.delete')}
                        >
                          <Trash2 />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        )}
      </div>

      {!listQuery.isPending && !listQuery.isError && total > 0 ? (
        <Pagination
          total={total}
          page={page}
          size={size}
          onPageChange={setPage}
          onSizeChange={setSize}
        />
      ) : null}

      {/* Create / edit dialog */}
      <Dialog
        open={dialogOpen}
        onOpenChange={(open) => {
          if (!saveMutation.isPending) setDialogOpen(open)
        }}
      >
        <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>
              {editing ? t('mappings.editMapping') : t('mappings.newMapping')}
            </DialogTitle>
            <DialogDescription className="sr-only">
              {editing ? t('mappings.editMapping') : t('mappings.newMapping')}
            </DialogDescription>
          </DialogHeader>

          <form
            className="space-y-4"
            onSubmit={form.handleSubmit((values) => onSubmit(values))}
          >
            <div className="space-y-2">
              <Label htmlFor="mapping-name">{t('common.name')}</Label>
              <Input
                id="mapping-name"
                placeholder={t('mappings.mappingName')}
                {...form.register('name')}
              />
              {form.formState.errors.name ? (
                <p className="text-xs text-destructive">{t('mappings.nameRequired')}</p>
              ) : null}
            </div>

            <div className="space-y-2">
              <Label>{t('mappings.rules')}</Label>
              <div className="flex flex-wrap items-center gap-2">
                <MultiSelect
                  className="min-w-40 flex-1"
                  options={ruleOptions.data}
                  value={ruleIds}
                  onChange={setRuleIds}
                  placeholder={t('mappings.selectRules')}
                  removeLabel={t('common.remove')}
                />
                <div className="flex shrink-0 items-center gap-2 rounded-lg border px-3 min-h-9">
                  <Label
                    htmlFor="mapping-continue"
                    className="text-muted-foreground"
                  >
                    {t('mappings.continueOnMatch')}
                  </Label>
                  <Switch
                    id="mapping-continue"
                    checked={form.watch('continue_on_match')}
                    onCheckedChange={(checked) => form.setValue('continue_on_match', checked)}
                  />
                </div>
              </div>
            </div>

            {/* Smart Import (collapsible) */}
            <div className="rounded-lg border">
              <button
                type="button"
                className="flex w-full items-center gap-2 px-3 py-2 text-sm font-medium transition-colors hover:bg-accent/50"
                onClick={() => setSmartOpen((v) => !v)}
                aria-expanded={smartOpen}
              >
                <Sparkles className="size-4 text-muted-foreground" />
                {t('mappings.smartImport')}
                {suggestions.length > 0 ? (
                  <Badge variant="secondary">
                    {suggestions.length} {t('mappings.fieldsFound')}
                  </Badge>
                ) : null}
                <ChevronDown
                  className={cn(
                    'ml-auto size-4 text-muted-foreground transition-transform',
                    smartOpen && 'rotate-180',
                  )}
                />
              </button>
              {smartOpen ? (
                <div className="space-y-3 border-t p-3">
                  <Textarea
                    rows={6}
                    spellCheck={false}
                    className="field-sizing-fixed min-w-0 resize-y"
                    placeholder={t('mappings.pasteJsonPlaceholder')}
                    value={smartJson}
                    onChange={(e) => setSmartJson(e.target.value)}
                  />
                  <div className="flex gap-2">
                    <Button
                      type="button"
                      size="sm"
                      disabled={!smartJson.trim() || analyzeMutation.isPending}
                      onClick={handleAnalyze}
                    >
                      {analyzeMutation.isPending ? (
                        <Loader2 className="animate-spin" />
                      ) : (
                        <Search />
                      )}
                      {t('mappings.analyzeJson')}
                    </Button>
                    {suggestions.length > 0 ? (
                      <Button type="button" variant="outline" size="sm" onClick={addAllSuggestions}>
                        <Plus />
                        {t('mappings.addAllFields')}
                      </Button>
                    ) : null}
                  </div>

                  {suggestions.length > 0 ? (
                    <div className="space-y-2">
                      <div className="flex items-center justify-between">
                        <span className="text-sm font-medium">{t('mappings.suggestedFields')}</span>
                        {structure ? (
                          <span className="flex items-center gap-1.5">
                            <Badge variant="outline">
                              {structure.root_is_array
                                ? t('playground.rootArray')
                                : t('playground.rootObject')}
                            </Badge>
                            <Badge variant="secondary" title={t('playground.itemCount')}>
                              {structure.item_count}
                            </Badge>
                          </span>
                        ) : null}
                      </div>
                      <div className="max-h-60 overflow-y-auto rounded-md border">
                        {suggestions.map((sf) => {
                          const added = addedKeys.has(suggestionKey(sf))
                          return (
                            <div
                              key={sf.source_path + (sf.multi_result_mode || '')}
                              className={cn(
                                'flex items-center gap-2 border-b px-2.5 py-1.5 text-sm last:border-b-0',
                                added && 'opacity-50',
                              )}
                            >
                              <span
                                className="w-40 shrink-0 truncate font-medium text-primary"
                                title={sf.source_path}
                              >
                                {sf.source_path}
                              </span>
                              <Badge variant={valueTypeBadgeVariant(sf.value_type)}>
                                {sf.value_type}
                              </Badge>
                              <span
                                className="min-w-0 flex-1 truncate text-xs text-muted-foreground"
                                title={sf.sample_value}
                              >
                                {sf.sample_value}
                              </span>
                              {added ? (
                                <span className="flex size-8 shrink-0 items-center justify-center">
                                  <Check className="size-4 text-primary" />
                                </span>
                              ) : (
                                <Button
                                  type="button"
                                  variant="ghost"
                                  size="icon-sm"
                                  className="shrink-0"
                                  onClick={() => addFromSuggestion(sf)}
                                  aria-label={t('mappings.addField')}
                                >
                                  <Plus />
                                </Button>
                              )}
                            </div>
                          )
                        })}
                      </div>
                    </div>
                  ) : null}
                </div>
              ) : null}
            </div>

            <Separator />

            <div className="flex items-center justify-between">
              <Label>{t('mappings.fields')}</Label>
              <Button type="button" variant="outline" size="sm" onClick={addField}>
                <Plus />
                {t('mappings.addField')}
              </Button>
            </div>

            {fields.map((field, index) => {
              const errors = fieldErrors[index]
              return (
                <div key={index} className="space-y-3 rounded-lg border p-3">
                  <div className="flex items-center justify-between">
                    <span className="text-sm font-medium">
                      {t('mappings.fieldNo', { n: index + 1 })}
                    </span>
                    <div className="flex items-center gap-3">
                      <Label
                        htmlFor={`field-required-${index}`}
                        className="flex items-center gap-2 font-normal text-muted-foreground"
                      >
                        {t('mappings.required')}
                        <Switch
                          id={`field-required-${index}`}
                          checked={field.required}
                          onCheckedChange={(checked) => updateField(index, { required: checked })}
                        />
                      </Label>
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        className="text-destructive hover:text-destructive"
                        onClick={() => removeField(index)}
                      >
                        <Trash2 />
                        {t('common.remove')}
                      </Button>
                    </div>
                  </div>

                  <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                    <div className="space-y-2">
                      <Label htmlFor={`field-source-${index}`}>{t('mappings.sourcePath')}</Label>
                      <Input
                        id={`field-source-${index}`}
                        placeholder={t('mappings.sourcePathPlaceholder')}
                        aria-invalid={errors?.source_path === true}
                        value={field.source_path}
                        onChange={(e) => updateField(index, { source_path: e.target.value })}
                      />
                      {errors?.source_path ? (
                        <p className="text-xs text-destructive">{t('mappings.sourcePathRequired')}</p>
                      ) : null}
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor={`field-target-${index}`}>{t('mappings.targetKey')}</Label>
                      <Input
                        id={`field-target-${index}`}
                        placeholder={t('mappings.targetKeyPlaceholder')}
                        aria-invalid={errors?.target_key === true}
                        value={field.target_key}
                        onChange={(e) => updateField(index, { target_key: e.target.value })}
                      />
                      {errors?.target_key ? (
                        <p className="text-xs text-destructive">{t('mappings.targetKeyRequired')}</p>
                      ) : null}
                    </div>
                  </div>

                  <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
                    <div className="space-y-2">
                      <Label>{t('mappings.valueType')}</Label>
                      <Select
                        value={field.value_type}
                        onValueChange={(v) => updateField(index, { value_type: v })}
                      >
                        <SelectTrigger>
                          <SelectValue placeholder={t('mappings.selectType')} />
                        </SelectTrigger>
                        <SelectContent>
                          {VALUE_TYPES.map((vt) => (
                            <SelectItem key={vt} value={vt}>
                              {t(`mappings.${vt}`)}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="space-y-2">
                      <Label>{t('mappings.multiResult')}</Label>
                      <Select
                        value={field.multi_result_mode || 'first'}
                        onValueChange={(v) => updateField(index, { multi_result_mode: v })}
                      >
                        <SelectTrigger>
                          <SelectValue placeholder={t('mappings.selectMode')} />
                        </SelectTrigger>
                        <SelectContent>
                          {MULTI_MODES.map((mode) => (
                            <SelectItem key={mode} value={mode}>
                              {t(
                                mode === 'join(, )' ? 'mappings.joinComma' : `mappings.${mode}`,
                              )}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="space-y-2">
                      <Label>{t('mappings.transform')}</Label>
                      <Select
                        value={field.transform || 'none'}
                        onValueChange={(v) => updateField(index, { transform: v === 'none' ? '' : v })}
                      >
                        <SelectTrigger>
                          <SelectValue placeholder={t('mappings.selectTransform')} />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="none">-</SelectItem>
                          {TRANSFORMS.map((tr) => (
                            <SelectItem key={tr} value={tr}>
                              {t(
                                tr === 'to_lower'
                                  ? 'mappings.lower'
                                  : tr === 'to_upper'
                                    ? 'mappings.upper'
                                    : 'mappings.trim',
                              )}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                  </div>

                  <div className="grid grid-cols-1 gap-3 sm:grid-cols-[1fr_2fr]">
                    <div className="space-y-2">
                      <Label htmlFor={`field-default-${index}`}>{t('mappings.default')}</Label>
                      <Input
                        id={`field-default-${index}`}
                        placeholder={t('mappings.optionalDefault')}
                        value={field.default_value}
                        onChange={(e) => updateField(index, { default_value: e.target.value })}
                      />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor={`field-desc-${index}`}>{t('mappings.fieldDesc')}</Label>
                      <Input
                        id={`field-desc-${index}`}
                        placeholder={t('mappings.fieldDescription')}
                        value={field.description}
                        onChange={(e) => updateField(index, { description: e.target.value })}
                      />
                    </div>
                  </div>
                </div>
              )
            })}

            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                onClick={() => setDialogOpen(false)}
                disabled={saveMutation.isPending}
              >
                {t('common.cancel')}
              </Button>
              <Button type="submit" disabled={saveMutation.isPending}>
                {saveMutation.isPending ? <Loader2 className="animate-spin" /> : null}
                {t('common.save')}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => {
          if (!deleteMutation.isPending) setDeleteTarget(open ? deleteTarget : null)
        }}
        title={t('common.confirmDelete')}
        description={t('common.confirmDeleteMsg', {
          entity: t('nav.mappings'),
          name: deleteTarget?.name ?? '',
        })}
        confirmLabel={t('common.delete')}
        onConfirm={() => deleteTarget && deleteMutation.mutate(deleteTarget.id)}
        pending={deleteMutation.isPending}
      />
    </div>
  )
}
