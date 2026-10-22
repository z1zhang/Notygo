import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useFieldArray, useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { toast } from 'sonner'
import { ListFilter, Loader2, Pencil, Plus, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { type TranslationKey } from '@/i18n'
import { createRule, deleteRule, listRules, updateRule } from '@/lib/api'
import { getErrorMessage } from '@/lib/api-client'
import type { Condition, ConditionGroup, RuleResponse } from '@/lib/types'
import { lookupName, useDestinationOptions, useTemplateOptions } from '@/hooks/use-lookup'
import { usePagination } from '@/hooks/use-pagination'
import { PageHeader } from '@/components/layout/PageHeader'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
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
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { Pagination } from '@/components/data-display/Pagination'
import { EmptyState } from '@/components/feedback/EmptyState'
import { ErrorState } from '@/components/feedback/ErrorState'
import { TableSkeleton } from '@/components/data-display/TableSkeleton'
import { ConfirmDialog } from '@/components/feedback/ConfirmDialog'
import { RefreshButton } from '@/components/feedback/RefreshButton'
import { cn } from '@/lib/utils'
import { MultiSelect } from '@/components/form/MultiSelect'
import { UnitInput } from '@/components/form/UnitInput'

const OPERATOR_GROUPS: { labelKey: TranslationKey; items: { value: string; labelKey: TranslationKey }[] }[] = [
  {
    labelKey: 'rules.equality',
    items: [
      { value: 'eq', labelKey: 'rules.equals' },
      { value: 'neq', labelKey: 'rules.notEquals' },
    ],
  },
  {
    labelKey: 'rules.stringGroup',
    items: [
      { value: 'contains', labelKey: 'rules.contains' },
      { value: 'not_contains', labelKey: 'rules.notContains' },
      { value: 'regex', labelKey: 'rules.regexMatch' },
    ],
  },
  {
    labelKey: 'rules.numericGroup',
    items: [
      { value: 'gt', labelKey: 'rules.greaterThan' },
      { value: 'lt', labelKey: 'rules.lessThan' },
      { value: 'gte', labelKey: 'rules.greaterOrEqual' },
      { value: 'lte', labelKey: 'rules.lessOrEqual' },
    ],
  },
  {
    labelKey: 'rules.existenceGroup',
    items: [
      { value: 'exists', labelKey: 'rules.exists' },
      { value: 'not_exists', labelKey: 'rules.notExists' },
      { value: 'not_empty', labelKey: 'rules.notEmpty' },
      { value: 'empty', labelKey: 'rules.empty' },
    ],
  },
]

const NO_VALUE_OPERATORS = ['exists', 'not_exists', 'not_empty', 'empty']

const ruleSchema = z.object({
  name: z.string().min(1),
  priority: z.number().min(0),
  enabled: z.boolean(),
  mode: z.enum(['and', 'or']),
  conditions: z.array(
    z.object({
      field: z.string(),
      operator: z.string(),
      value: z.string(),
    }),
  ),
  template_id: z.string(),
  destination_ids: z.array(z.string()),
  dedupeEnabled: z.boolean(),
  dedupeWindow: z.number().min(1).max(86400),
})

type RuleForm = z.infer<typeof ruleSchema>

const defaultForm: RuleForm = {
  name: '',
  priority: 0,
  enabled: true,
  mode: 'and',
  conditions: [],
  template_id: '',
  destination_ids: [],
  dedupeEnabled: false,
  dedupeWindow: 300,
}

/** row.conditions is a ConditionGroup; tolerate a legacy flat array of conditions. */
function parseConditions(
  conditions: ConditionGroup | Condition[] | null | undefined,
): { mode: 'and' | 'or'; conditions: Condition[] } {
  if (Array.isArray(conditions)) {
    return { mode: 'and', conditions }
  }
  if (conditions && typeof conditions === 'object') {
    return {
      mode: conditions.operator === 'or' ? 'or' : 'and',
      conditions: conditions.conditions ?? [],
    }
  }
  return { mode: 'and', conditions: [] }
}

export default function RulesPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { page, size, setPage, setSize } = usePagination()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<RuleResponse | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<RuleResponse | null>(null)

  const listQuery = useQuery({
    queryKey: ['rules', page, size],
    queryFn: () => listRules({ page, size }),
  })
  const templateOptions = useTemplateOptions()
  const destinationOptions = useDestinationOptions()

  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: ['rules'] })
    void queryClient.invalidateQueries({ queryKey: ['lookup', 'rules'] })
  }

  // Backend PUT /rules/{id} decodes the FULL rule object (a {enabled} patch
  // fails with 400) — the toggle must reconstruct the entire rule from the row.
  const toggleMutation = useMutation({
    mutationFn: ({ row, enabled }: { row: RuleResponse; enabled: boolean }) =>
      updateRule(row.id, {
        name: row.name,
        enabled,
        priority: row.priority,
        conditions: row.conditions ?? { operator: 'and', conditions: [] },
        template_id: row.template_id ?? null,
        destination_ids: row.destination_ids ?? [],
        dedupe_config: row.dedupe_config ?? null,
      }),
    onSuccess: (_data, variables) => {
      toast.success(
        variables.enabled ? t('rules.ruleEnabled') : t('rules.ruleDisabled'),
      )
      invalidate()
    },
    onError: (e) => {
      toast.error(getErrorMessage(e, t('rules.failedToToggleRule')))
    },
  })

  const saveMutation = useMutation({
    mutationFn: async (data: RuleForm) => {
      const conditions = data.conditions
        .filter((c) => c.field !== '')
        .map((c) => ({
          field: c.field,
          operator: c.operator,
          value: NO_VALUE_OPERATORS.includes(c.operator) ? '' : c.value,
        }))
      const payload = {
        name: data.name,
        priority: data.priority,
        enabled: data.enabled,
        conditions: { operator: data.mode, conditions },
        template_id: data.template_id || null,
        destination_ids: data.destination_ids,
        dedupe_config: data.dedupeEnabled ? { window_seconds: data.dedupeWindow } : null,
      }
      if (editing) {
        return updateRule(editing.id, payload)
      }
      return createRule(payload)
    },
    onSuccess: () => {
      toast.success(editing ? t('rules.ruleUpdated') : t('rules.ruleCreated'))
      setDialogOpen(false)
      invalidate()
    },
    onError: (e) => {
      toast.error(getErrorMessage(e, t('rules.failedToSaveRule')))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteRule(id),
    onSuccess: () => {
      toast.success(t('rules.ruleDeleted'))
      setDeleteTarget(null)
      invalidate()
    },
    onError: (e) => {
      toast.error(getErrorMessage(e, t('common.failedToSave', { entity: t('nav.rules') })))
    },
  })

  const form = useForm<RuleForm>({
    resolver: zodResolver(ruleSchema),
    defaultValues: defaultForm,
  })
  const { fields, append, remove } = useFieldArray({ control: form.control, name: 'conditions' })
  const mode = form.watch('mode')
  const conditionsWatch = form.watch('conditions')
  const dedupeEnabled = form.watch('dedupeEnabled')

  function openCreate() {
    setEditing(null)
    form.reset(defaultForm)
    setDialogOpen(true)
  }

  function openEdit(row: RuleResponse) {
    setEditing(row)
    const parsed = parseConditions(row.conditions as ConditionGroup | Condition[] | null | undefined)
    form.reset({
      name: row.name,
      priority: row.priority ?? 0,
      enabled: row.enabled,
      mode: parsed.mode,
      conditions: parsed.conditions.map((c) => ({
        field: c.field ?? '',
        operator: c.operator ?? 'eq',
        value: c.value ?? '',
      })),
      template_id: row.template_id ?? '',
      destination_ids: row.destination_ids ?? [],
      dedupeEnabled: Boolean(row.dedupe_config && (row.dedupe_config.window_seconds ?? 0) > 0),
      dedupeWindow: row.dedupe_config?.window_seconds && row.dedupe_config.window_seconds > 0
        ? row.dedupe_config.window_seconds
        : 300,
    })
    setDialogOpen(true)
  }

  const rows = listQuery.data?.data ?? []
  const total = listQuery.data?.total ?? 0

  function valuePlaceholder(operator: string): string {
    if (operator === 'regex') return t('rules.regexPlaceholder')
    if (['gt', 'lt', 'gte', 'lte'].includes(operator)) return t('rules.numericPlaceholder')
    return t('rules.comparisonPlaceholder')
  }

  return (
    <div className="space-y-4">
      <PageHeader
        title={t('nav.rules')}
        description={t('rules.description')}
        actions={
          <>
            <RefreshButton
              label={t('dashboard.refresh')}
              onClick={() => void listQuery.refetch()}
              loading={listQuery.isFetching}
            />
            <Button size="sm" onClick={openCreate}>
              <Plus />
              {t('rules.newRule')}
            </Button>
          </>
        }
      />

      <div className="rounded-lg border">
        {listQuery.isPending ? (
          <TableSkeleton cols={6} />
        ) : listQuery.isError ? (
          <ErrorState
            error={listQuery.error}
            onRetry={() => void listQuery.refetch()}
            entity={t('nav.rules')}
          />
        ) : rows.length === 0 ? (
          <EmptyState
            title={t('rules.emptyTitle')}
            description={t('rules.emptyDescription')}
            icon={<ListFilter className="size-5" />}
            action={
              <Button size="sm" variant="outline" onClick={openCreate}>
                <Plus />
                {t('rules.newRule')}
              </Button>
            }
          />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('common.name')}</TableHead>
                <TableHead className="w-20">{t('rules.priority')}</TableHead>
                <TableHead className="w-20">{t('common.enabled')}</TableHead>
                <TableHead>{t('rules.template')}</TableHead>
                <TableHead>{t('rules.destinations')}</TableHead>
                <TableHead className="w-24 text-right">{t('common.actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((row) => {
                const destIds = row.destination_ids ?? []
                const shown = destIds.slice(0, 3)
                const rest = destIds.length - shown.length
                return (
                  <TableRow key={row.id}>
                    <TableCell className="text-muted-foreground">{row.name}</TableCell>
                    <TableCell className="text-muted-foreground">{row.priority}</TableCell>
                    <TableCell>
                      <Switch
                        checked={row.enabled}
                        onCheckedChange={(checked) =>
                          toggleMutation.mutate({ row, enabled: checked })
                        }
                        aria-label={t('common.enabled')}
                      />
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {lookupName(templateOptions.data, row.template_id)}
                    </TableCell>
                    <TableCell>
                      {destIds.length === 0 ? (
                        <span className="text-muted-foreground">-</span>
                      ) : (
                        <div className="flex flex-wrap gap-1">
                          {shown.map((id) => (
                            <Badge key={id} variant="secondary" className="max-w-40">
                              <span className="truncate">
                                {lookupName(destinationOptions.data, id)}
                              </span>
                            </Badge>
                          ))}
                          {rest > 0 ? (
                            <Badge variant="outline">+{rest}</Badge>
                          ) : null}
                        </div>
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
              {editing ? t('rules.editRule') : t('rules.newRule')}
            </DialogTitle>
            <DialogDescription className="sr-only">
              {editing ? t('rules.editRule') : t('rules.newRule')}
            </DialogDescription>
          </DialogHeader>

          <form
            className="space-y-4"
            onSubmit={form.handleSubmit((values) => saveMutation.mutate(values))}
          >
            <div className="grid grid-cols-[1fr_120px_auto] gap-4">
              <div className="space-y-2">
                <Label htmlFor="rule-name">{t('common.name')}</Label>
                <Input
                  id="rule-name"
                  placeholder={t('rules.ruleName')}
                  {...form.register('name')}
                />
                {form.formState.errors.name ? (
                  <p className="text-xs text-destructive">{t('rules.nameRequired')}</p>
                ) : null}
              </div>

              <div className="space-y-2">
                <Label htmlFor="rule-priority">{t('rules.priority')}</Label>
                <Input
                  id="rule-priority"
                  type="number"
                  min={0}
                  {...form.register('priority', { valueAsNumber: true })}
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="rule-enabled">{t('common.enabled')}</Label>
                <div className="flex h-9 items-center">
                  <Switch
                    id="rule-enabled"
                    checked={form.watch('enabled')}
                    onCheckedChange={(checked) => form.setValue('enabled', checked)}
                  />
                </div>
              </div>
            </div>

            <div className="space-y-3">
              <div className="flex flex-wrap items-center gap-3">
                <Label className="shrink-0">{t('rules.conditions')}</Label>
                <div className="relative inline-flex h-8 items-center rounded-md border border-input bg-transparent p-0.5 text-sm">
                  {/* Sliding highlight — width is half the track, drawer-slides between the two options */}
                  <span
                    aria-hidden
                    className={cn(
                      'absolute inset-y-0.5 left-0.5 w-[calc(50%-2px)] rounded-sm bg-primary transition-transform duration-200 ease-in-out',
                      mode === 'or' && 'translate-x-full',
                    )}
                  />
                  {(['and', 'or'] as const).map((m) => (
                    <Tooltip key={m}>
                      <TooltipTrigger asChild>
                        <button
                          type="button"
                          className={cn(
                            'relative z-10 min-w-0 flex-1 truncate rounded-sm px-3 py-1 text-center text-sm transition-colors',
                            mode === m
                              ? 'text-primary-foreground'
                              : 'text-muted-foreground hover:text-foreground',
                          )}
                          onClick={() => form.setValue('mode', m)}
                        >
                      {t(m === 'and' ? 'rules.andShort' : 'rules.orShort')}
                        </button>
                      </TooltipTrigger>
                      <TooltipContent>
                        {t(m === 'and' ? 'rules.andMode' : 'rules.orMode')}
                      </TooltipContent>
                    </Tooltip>
                  ))}
                </div>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="ml-auto"
                  onClick={() => append({ field: '', operator: 'eq', value: '' })}
                >
                  <Plus />
                  {t('rules.addCondition')}
                </Button>
              </div>

              {fields.length === 0 ? (
                <p className="text-sm text-muted-foreground">{t('rules.noConditions')}</p>
              ) : null}

              {fields.map((field, index) => {
                const operator = conditionsWatch[index]?.operator ?? 'eq'
                const hasValue = !NO_VALUE_OPERATORS.includes(operator)
                return (
                  <div key={field.id} className="space-y-3 rounded-lg border p-3">
                    <div className="flex items-center justify-between">
                      <Label className="text-xs text-muted-foreground">
                        {t('rules.conditionNo', { n: index + 1 })}
                      </Label>
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        className="text-destructive hover:text-destructive"
                        onClick={() => remove(index)}
                      >
                        {t('common.remove')}
                      </Button>
                    </div>

                    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                      <div className="space-y-1.5">
                        <Label htmlFor={`condition-field-${index}`} className="text-xs">
                          {t('rules.field')}
                        </Label>
                        <Input
                          id={`condition-field-${index}`}
                          placeholder={t('rules.fieldPlaceholder')}
                          {...form.register(`conditions.${index}.field`)}
                        />
                      </div>
                      <div className="space-y-1.5">
                        <Label className="text-xs">{t('rules.operator')}</Label>
                        <Select
                          value={operator}
                          onValueChange={(v) =>
                            form.setValue(`conditions.${index}.operator`, v)
                          }
                        >
                          <SelectTrigger className="w-full">
                            <SelectValue placeholder={t('rules.selectOperator')} />
                          </SelectTrigger>
                          <SelectContent>
                            {OPERATOR_GROUPS.map((group) => (
                              <SelectGroup key={group.labelKey}>
                                <SelectLabel>{t(group.labelKey)}</SelectLabel>
                                {group.items.map((item) => (
                                  <SelectItem key={item.value} value={item.value}>
                                    {t(item.labelKey)}
                                  </SelectItem>
                                ))}
                              </SelectGroup>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>
                    </div>

                    {hasValue ? (
                      <div className="space-y-1.5">
                        <Label htmlFor={`condition-value-${index}`} className="text-xs">
                          {t('rules.value')}
                        </Label>
                        <Input
                          id={`condition-value-${index}`}
                          placeholder={valuePlaceholder(operator)}
                          {...form.register(`conditions.${index}.value`)}
                        />
                      </div>
                    ) : null}
                  </div>
                )
              })}
            </div>

            <div className="space-y-2">
              <Label htmlFor="rule-template">{t('rules.template')}</Label>
              <Select
                value={form.watch('template_id') || 'none'}
                onValueChange={(v) => form.setValue('template_id', v === 'none' ? '' : v)}
              >
                <SelectTrigger className="w-full">
                  <SelectValue placeholder={t('rules.selectTemplate')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="none">-</SelectItem>
                  {(templateOptions.data ?? []).map((tpl) => (
                    <SelectItem key={tpl.id} value={tpl.id}>
                      {tpl.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <Label>{t('rules.destinations')}</Label>
              <MultiSelect
                options={destinationOptions.data}
                value={form.watch('destination_ids')}
                onChange={(value) => form.setValue('destination_ids', value)}
                placeholder={t('rules.selectDestinations')}
                removeLabel={t('common.remove')}
              />
            </div>

            <div className="grid grid-cols-[auto_1fr] gap-4">
              <div className="space-y-2">
                <Label htmlFor="rule-dedupe">{t('rules.dedupeEnabled')}</Label>
                <div className="flex h-9 items-center">
                  <Switch
                    id="rule-dedupe"
                    checked={dedupeEnabled}
                    onCheckedChange={(checked) => form.setValue('dedupeEnabled', checked)}
                  />
                </div>
              </div>
              <div className="space-y-2">
                <Label htmlFor="rule-dedupe-window">{t('rules.windowSec')}</Label>
                <UnitInput
                  id="rule-dedupe-window"
                  type="number"
                  min={1}
                  max={86400}
                  disabled={!dedupeEnabled}
                  placeholder={t('rules.windowPlaceholder')}
                  unit={t('rules.secUnit')}
                  {...form.register('dedupeWindow', { valueAsNumber: true })}
                />
              </div>
            </div>

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
          entity: t('nav.rules'),
          name: deleteTarget?.name ?? '',
        })}
        confirmLabel={t('common.delete')}
        onConfirm={() => deleteTarget && deleteMutation.mutate(deleteTarget.id)}
        pending={deleteMutation.isPending}
      />
    </div>
  )
}
