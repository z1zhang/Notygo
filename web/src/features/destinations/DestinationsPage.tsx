import { useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useFieldArray, useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { toast } from 'sonner'
import { Loader2, Pencil, Plus, Send, Trash2, X } from 'lucide-react'
import { useI18n } from '@/i18n'
import { createDestination, deleteDestination, listDestinations, updateDestination } from '@/lib/api'
import { getErrorMessage } from '@/lib/api-client'
import type { DestinationResponse } from '@/lib/types'
import { usePagination } from '@/hooks/use-pagination'
import { PageHeader } from '@/components/layout/PageHeader'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { UnitInput } from '@/components/form/UnitInput'
import { PasswordInput } from '@/components/ui/password-input'
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

const DEST_TYPES = ['dingtalk', 'feishu', 'wecom', 'email', 'http_webhook'] as const
type DestType = (typeof DEST_TYPES)[number]

const WEBHOOK_TYPES: DestType[] = ['dingtalk', 'feishu', 'wecom']

const MSG_TYPE_OPTIONS: Record<string, string[]> = {
  dingtalk: ['markdown', 'actionCard'],
  feishu: ['interactive', 'text', 'post'],
  wecom: ['markdown', 'text', 'image', 'news'],
}

const DEFAULT_MSG_TYPE: Record<string, string> = {
  dingtalk: 'markdown',
  feishu: 'interactive',
  wecom: 'markdown',
}

const HTTP_METHODS = ['POST', 'GET', 'PUT', 'PATCH'] as const

const destinationSchema = z
  .object({
    name: z.string().min(1),
    type: z.enum(DEST_TYPES),
    webhook_url: z.string(),
    msg_type: z.string(),
    smtp_host: z.string(),
    smtp_port: z.number().min(1).max(65535),
    use_tls: z.boolean(),
    from: z.string(),
    to: z.string(),
    cc: z.string(),
    url: z.string(),
    method: z.enum(HTTP_METHODS),
    headers: z.array(z.object({ key: z.string(), value: z.string() })),
    body_template: z.string(),
    secret: z.string(),
    timeout_ms: z.number().min(1000),
    enabled: z.boolean(),
  })
  // The delivery target URL is required for the corresponding types
  .superRefine((data, ctx) => {
    if (data.type === 'http_webhook' && !data.url.trim()) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, path: ['url'], message: 'urlRequired' })
    }
    if (WEBHOOK_TYPES.includes(data.type) && !data.webhook_url.trim()) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, path: ['webhook_url'], message: 'webhookUrlRequired' })
    }
  })

type DestinationForm = z.infer<typeof destinationSchema>

const defaultForm: DestinationForm = {
  name: '',
  type: 'http_webhook',
  webhook_url: '',
  msg_type: '',
  smtp_host: '',
  smtp_port: 465,
  use_tls: true,
  from: '',
  to: '',
  cc: '',
  url: '',
  method: 'POST',
  headers: [],
  body_template: '',
  secret: '',
  timeout_ms: 10000,
  enabled: true,
}

// ── Config parsing helpers (server config is a loosely-typed object) ──────

function asString(value: unknown): string {
  return typeof value === 'string' ? value : ''
}

function asNumber(value: unknown, fallback: number): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : fallback
}

/** to/cc may be an array of emails or a string — normalize to a textarea string. */
function asAddressList(value: unknown): string {
  if (Array.isArray(value)) {
    return value.filter((v): v is string => typeof v === 'string').join('\n')
  }
  return asString(value)
}

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

function splitAddressList(value: string): string[] {
  return value
    .split(/[\n,]/)
    .map((v) => v.trim())
    .filter((v) => v !== '')
}

function buildConfig(data: DestinationForm): Record<string, unknown> {
  switch (data.type) {
    case 'dingtalk':
    case 'feishu':
    case 'wecom': {
      const config: Record<string, unknown> = { webhook_url: data.webhook_url }
      if (data.msg_type !== '') config.msg_type = data.msg_type
      return config
    }
    case 'email': {
      const config: Record<string, unknown> = {
        smtp_host: data.smtp_host,
        smtp_port: data.smtp_port,
        use_tls: data.use_tls,
        from: data.from,
        to: splitAddressList(data.to),
      }
      const cc = splitAddressList(data.cc)
      if (cc.length > 0) config.cc = cc
      return config
    }
    case 'http_webhook': {
      const config: Record<string, unknown> = { url: data.url, method: data.method }
      const headers = Object.fromEntries(
        data.headers
          .filter((h) => h.key !== '')
          .map((h) => [h.key, h.value]),
      )
      if (Object.keys(headers).length > 0) config.headers = headers
      if (data.body_template !== '') config.body_template = data.body_template
      return config
    }
  }
}

export default function DestinationsPage() {
  const { t } = useI18n()
  const queryClient = useQueryClient()
  const { page, size, setPage, setSize } = usePagination()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<DestinationResponse | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<DestinationResponse | null>(null)
  // Server returns the DECRYPTED secret on edit — only re-submit when changed.
  const originalSecretRef = useRef('')

  const listQuery = useQuery({
    queryKey: ['destinations', page, size],
    queryFn: () => listDestinations({ page, size }),
  })

  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: ['destinations'] })
    void queryClient.invalidateQueries({ queryKey: ['lookup', 'destinations'] })
  }

  // Destination update is partial — the toggle can safely send {enabled} only.
  // Revert the switch on error by re-fetching the list.
  const toggleMutation = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      updateDestination(id, { enabled }),
    onSuccess: (_data, variables) => {
      toast.success(
        variables.enabled
          ? t('destinations.destinationEnabled')
          : t('destinations.destinationDisabled'),
      )
      invalidate()
    },
    onError: (e) => {
      toast.error(getErrorMessage(e, t('destinations.failedToToggleDestination')))
      void queryClient.invalidateQueries({ queryKey: ['destinations'] })
    },
  })

  const saveMutation = useMutation({
    mutationFn: async (data: DestinationForm) => {
      const payload: Record<string, unknown> = {
        name: data.name,
        type: data.type,
        config: buildConfig(data),
        timeout_ms: data.timeout_ms,
        enabled: data.enabled,
      }
      if (data.secret !== '' && data.secret !== originalSecretRef.current) {
        payload.secret_ref = data.secret
      }
      if (editing) {
        return updateDestination(editing.id, payload)
      }
      return createDestination(payload)
    },
    onSuccess: () => {
      toast.success(
        editing ? t('destinations.destinationUpdated') : t('destinations.destinationCreated'),
      )
      setDialogOpen(false)
      invalidate()
    },
    onError: (e) => {
      toast.error(getErrorMessage(e, t('destinations.failedToSaveDestination')))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteDestination(id),
    onSuccess: () => {
      toast.success(t('destinations.destinationDeleted'))
      setDeleteTarget(null)
      invalidate()
    },
    onError: (e) => {
      toast.error(getErrorMessage(e, t('common.failedToSave', { entity: t('nav.destinations') })))
    },
  })

  const form = useForm<DestinationForm>({
    resolver: zodResolver(destinationSchema),
    defaultValues: defaultForm,
  })
  const { fields: headerFields, append: appendHeader, remove: removeHeader } = useFieldArray({
    control: form.control,
    name: 'headers',
  })
  const type = form.watch('type')

  function openCreate() {
    setEditing(null)
    originalSecretRef.current = ''
    form.reset(defaultForm)
    setDialogOpen(true)
  }

  function openEdit(row: DestinationResponse) {
    setEditing(row)
    originalSecretRef.current = row.secret_ref ?? ''
    const config = (row.config ?? {}) as Record<string, unknown>
    const rowType = (DEST_TYPES.includes(row.type as DestType)
      ? row.type
      : 'http_webhook') as DestType
    const msgOptions = MSG_TYPE_OPTIONS[rowType] ?? []
    form.reset({
      name: row.name,
      type: rowType,
      webhook_url: asString(config.webhook_url),
      msg_type: msgOptions.includes(asString(config.msg_type))
        ? asString(config.msg_type)
        : (DEFAULT_MSG_TYPE[rowType] ?? ''),
      smtp_host: asString(config.smtp_host),
      smtp_port: asNumber(config.smtp_port, 465),
      use_tls: typeof config.use_tls === 'boolean' ? config.use_tls : true,
      from: asString(config.from),
      to: asAddressList(config.to),
      cc: asAddressList(config.cc),
      url: asString(config.url),
      method: (HTTP_METHODS.includes(asString(config.method) as (typeof HTTP_METHODS)[number])
        ? asString(config.method)
        : 'POST') as DestinationForm['method'],
      headers: asHeaderRows(config.headers),
      body_template: asString(config.body_template),
      secret: row.secret_ref ?? '',
      timeout_ms: asNumber(row.timeout_ms, 10000),
      enabled: row.enabled,
    })
    setDialogOpen(true)
  }

  // Switching type resets the type-specific config fields.
  function handleTypeChange(next: string) {
    const nextType = next as DestinationForm['type']
    form.setValue('type', nextType)
    form.setValue('msg_type', DEFAULT_MSG_TYPE[nextType] ?? '')
    form.setValue('webhook_url', '')
    form.setValue('smtp_host', '')
    form.setValue('smtp_port', 465)
    form.setValue('use_tls', true)
    form.setValue('from', '')
    form.setValue('to', '')
    form.setValue('cc', '')
    form.setValue('url', '')
    form.setValue('method', 'POST')
    form.setValue('headers', [])
    form.setValue('body_template', '')
  }

  const rows = listQuery.data?.data ?? []
  const total = listQuery.data?.total ?? 0
  const msgTypeOptions = MSG_TYPE_OPTIONS[type] ?? []

  return (
    <div className="space-y-4">
      <PageHeader
        title={t('nav.destinations')}
        description={t('destinations.description')}
        actions={
          <>
            <RefreshButton
              label={t('dashboard.refresh')}
              onClick={() => void listQuery.refetch()}
              loading={listQuery.isFetching}
            />
            <Button size="sm" onClick={openCreate}>
              <Plus />
              {t('destinations.newDestination')}
            </Button>
          </>
        }
      />

      <div className="rounded-lg border">
        {listQuery.isPending ? (
          <TableSkeleton cols={4} />
        ) : listQuery.isError ? (
          <ErrorState
            error={listQuery.error}
            onRetry={() => void listQuery.refetch()}
            entity={t('nav.destinations')}
          />
        ) : rows.length === 0 ? (
          <EmptyState
            title={t('destinations.emptyTitle')}
            description={t('destinations.emptyDescription')}
            icon={<Send className="size-5" />}
            action={
              <Button size="sm" variant="outline" onClick={openCreate}>
                <Plus />
                {t('destinations.newDestination')}
              </Button>
            }
          />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('common.name')}</TableHead>
                <TableHead className="w-32">{t('common.type')}</TableHead>
                <TableHead className="w-20">{t('common.enabled')}</TableHead>
                <TableHead className="w-24 text-right">{t('common.actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((row) => (
                <TableRow key={row.id}>
                  <TableCell className="text-muted-foreground">{row.name}</TableCell>
                  <TableCell>
                    <Badge variant="secondary">
                      {t(`enums.destType.${row.type}`, row.type)}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    <Switch
                      checked={row.enabled}
                      onCheckedChange={(checked) =>
                        toggleMutation.mutate({ id: row.id, enabled: checked })
                      }
                      aria-label={t('common.enabled')}
                    />
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
              ))}
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
              {editing ? t('destinations.editDestination') : t('destinations.newDestination')}
            </DialogTitle>
            <DialogDescription className="sr-only">
              {editing ? t('destinations.editDestination') : t('destinations.newDestination')}
            </DialogDescription>
          </DialogHeader>

          <form
            className="space-y-4"
            onSubmit={form.handleSubmit((values) => saveMutation.mutate(values))}
          >
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-[1fr_1fr_auto]">
              <div className="space-y-2">
                <Label htmlFor="destination-name">{t('common.name')}</Label>
                <Input
                  id="destination-name"
                  placeholder={t('destinations.destinationName')}
                  {...form.register('name')}
                />
                {form.formState.errors.name ? (
                  <p className="text-xs text-destructive">{t('destinations.nameRequired')}</p>
                ) : null}
              </div>

              <div className="space-y-2">
                <Label>{t('common.type')}</Label>
                <Select value={type} onValueChange={handleTypeChange}>
                  <SelectTrigger className="w-full">
                    <SelectValue placeholder={t('destinations.selectType')} />
                  </SelectTrigger>
                  <SelectContent>
                    {DEST_TYPES.map((destType) => (
                      <SelectItem key={destType} value={destType}>
                        {t(`enums.destType.${destType}`)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-2">
                <Label htmlFor="destination-enabled">{t('common.enabled')}</Label>
                <div className="flex h-9 items-center">
                  <Switch
                    id="destination-enabled"
                    checked={form.watch('enabled')}
                    onCheckedChange={(checked) => form.setValue('enabled', checked)}
                  />
                </div>
              </div>
            </div>

            {/* Type-specific config */}
            {WEBHOOK_TYPES.includes(type) ? (
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div className="space-y-2">
                  <Label htmlFor="destination-webhook-url">{t('destinations.webhookUrl')}</Label>
                  <Input
                    id="destination-webhook-url"
                    placeholder="https://..."
                    {...form.register('webhook_url')}
                  />
                  {form.formState.errors.webhook_url ? (
                    <p className="text-xs text-destructive">{t('destinations.webhookUrlRequired')}</p>
                  ) : null}
                </div>
                <div className="space-y-2">
                  <Label>{t('destinations.msgType')}</Label>
                  <Select
                    value={form.watch('msg_type')}
                    onValueChange={(v) => form.setValue('msg_type', v)}
                  >
                    <SelectTrigger className="w-full">
                      <SelectValue placeholder={t('destinations.msgType')} />
                    </SelectTrigger>
                    <SelectContent>
                      {msgTypeOptions.map((option) => (
                        <SelectItem key={option} value={option}>
                          {t(`enums.msgType.${option}`, option)}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </div>
            ) : null}

            {type === 'email' ? (
              <div className="space-y-4">
                <div className="grid grid-cols-1 gap-4 sm:grid-cols-[1fr_130px_130px]">
                  <div className="space-y-2">
                    <Label htmlFor="destination-smtp-host">{t('destinations.smtpHost')}</Label>
                    <Input
                      id="destination-smtp-host"
                      placeholder="smtp.example.com"
                      {...form.register('smtp_host')}
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="destination-smtp-port">{t('destinations.smtpPort')}</Label>
                    <Input
                      id="destination-smtp-port"
                      type="number"
                      min={1}
                      max={65535}
                      {...form.register('smtp_port', { valueAsNumber: true })}
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="destination-use-tls">{t('destinations.useTls')}</Label>
                    <div className="flex h-9 items-center">
                      <Switch
                        id="destination-use-tls"
                        checked={form.watch('use_tls')}
                        onCheckedChange={(checked) => form.setValue('use_tls', checked)}
                      />
                    </div>
                  </div>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="destination-from">{t('destinations.fromAddress')}</Label>
                  <Input
                    id="destination-from"
                    placeholder="noreply@example.com"
                    {...form.register('from')}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="destination-to">{t('destinations.toAddress')}</Label>
                  <Textarea
                    id="destination-to"
                    rows={3}
                    placeholder={t('destinations.toPlaceholder')}
                    {...form.register('to')}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="destination-cc">{t('destinations.ccAddress')}</Label>
                  <Textarea
                    id="destination-cc"
                    rows={2}
                    placeholder={t('destinations.ccPlaceholder')}
                    {...form.register('cc')}
                  />
                </div>
              </div>
            ) : null}

            {type === 'http_webhook' ? (
              <div className="space-y-4">
                <div className="grid grid-cols-[1fr_140px] gap-4">
                  <div className="space-y-2">
                    <Label htmlFor="destination-url">{t('destinations.targetUrl')}</Label>
                    <Input
                      id="destination-url"
                      placeholder="https://..."
                      {...form.register('url')}
                    />
                    {form.formState.errors.url ? (
                      <p className="text-xs text-destructive">{t('destinations.urlRequired')}</p>
                    ) : null}
                  </div>
                  <div className="space-y-2">
                    <Label>{t('destinations.httpMethod')}</Label>
                    <Select
                      value={form.watch('method')}
                      onValueChange={(v) => form.setValue('method', v as DestinationForm['method'])}
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
                    <p className="text-xs text-muted-foreground">
                      {t('destinations.noHeaders')}
                    </p>
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
                  <Label htmlFor="destination-body-template">
                    {t('destinations.bodyTemplate')}
                  </Label>
                  <Textarea
                    id="destination-body-template"
                    rows={4}
                    {...form.register('body_template')}
                  />
                </div>
              </div>
            ) : null}

            {/* Common config */}
            <Separator />
            <div className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="destination-secret">{t('destinations.secret')}</Label>
                <PasswordInput
                  id="destination-secret"
                  autoComplete="new-password"
                  placeholder={
                    editing
                      ? t('destinations.secretEditPlaceholder')
                      : t('destinations.secretPlaceholder')
                  }
                  {...form.register('secret')}
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="destination-timeout">{t('destinations.timeoutMs')}</Label>
                <UnitInput
                  id="destination-timeout"
                  type="number"
                  min={1000}
                  step={1000}
                  unit={t('destinations.msUnit')}
                  {...form.register('timeout_ms', { valueAsNumber: true })}
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
          entity: t('nav.destinations'),
          name: deleteTarget?.name ?? '',
        })}
        confirmLabel={t('common.delete')}
        onConfirm={() => deleteTarget && deleteMutation.mutate(deleteTarget.id)}
        pending={deleteMutation.isPending}
      />
    </div>
  )
}
