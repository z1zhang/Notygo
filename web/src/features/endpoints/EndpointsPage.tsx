import { useMemo, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { toast } from 'sonner'
import { Loader2, Pencil, Plus, Trash2, Webhook } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { createEndpoint, deleteEndpoint, listEndpoints, updateEndpoint } from '@/lib/api'
import { getErrorMessage } from '@/lib/api-client'
import { copyToClipboard } from '@/lib/formatters'
import type { EndpointResponse } from '@/lib/types'
import { useMappingOptions, lookupName } from '@/hooks/use-lookup'
import { usePagination } from '@/hooks/use-pagination'
import { PageHeader } from '@/components/layout/PageHeader'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { UnitInput } from '@/components/form/UnitInput'
import { PasswordInput } from '@/components/ui/password-input'
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
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { Pagination } from '@/components/data-display/Pagination'
import { CopyButton } from '@/components/data-display/CopyButton'
import { EmptyState } from '@/components/feedback/EmptyState'
import { ErrorState } from '@/components/feedback/ErrorState'
import { TableSkeleton } from '@/components/data-display/TableSkeleton'
import { ConfirmDialog } from '@/components/feedback/ConfirmDialog'
import { RefreshButton } from '@/components/feedback/RefreshButton'

const AUTH_TYPES = ['none', 'bearer', 'hmac'] as const

const endpointSchema = z.object({
  name: z.string().min(1),
  auth_type: z.enum(AUTH_TYPES),
  secret: z.string(),
  maxBodyKb: z.number().min(0),
  rate: z.number().min(0),
  burst: z.number().min(0),
  mapping_id: z.string(),
  enabled: z.boolean(),
})

type EndpointForm = z.infer<typeof endpointSchema>

const defaultForm: EndpointForm = {
  name: '',
  auth_type: 'none',
  secret: '',
  maxBodyKb: 1024,
  rate: 0,
  burst: 0,
  mapping_id: '',
  enabled: true,
}

function parseRateLimit(rateLimit: string | null | undefined): { rate: number; burst: number } {
  if (!rateLimit) return { rate: 0, burst: 0 }
  try {
    const parsed = JSON.parse(rateLimit) as { rate?: number; burst?: number }
    return { rate: parsed.rate ?? 0, burst: parsed.burst ?? 0 }
  } catch {
    return { rate: 0, burst: 0 }
  }
}

function generateSecret(): string {
  const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789'
  const bytes = new Uint8Array(32)
  crypto.getRandomValues(bytes)
  return Array.from(bytes, (b) => chars[b % chars.length]).join('')
}

export default function EndpointsPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { page, size, setPage, setSize } = usePagination()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<EndpointResponse | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<EndpointResponse | null>(null)
  // Server returns the DECRYPTED secret on edit — only re-submit when changed.
  const originalSecretRef = useRef('')

  const listQuery = useQuery({
    queryKey: ['endpoints', page, size],
    queryFn: () => listEndpoints({ page, size }),
  })
  const mappingOptions = useMappingOptions()

  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: ['endpoints'] })
    void queryClient.invalidateQueries({ queryKey: ['lookup', 'endpoints'] })
  }

  const saveMutation = useMutation({
    mutationFn: async (data: EndpointForm) => {
      const payload: Record<string, unknown> = {
        name: data.name,
        auth_type: data.auth_type,
        max_body_bytes: Math.round(data.maxBodyKb) * 1024,
        enabled: data.enabled,
        mapping_id: data.mapping_id || null,
      }
      // Only re-submit the secret when it actually changed (blank = keep existing)
      if (data.auth_type !== 'none' && data.secret !== '' && data.secret !== originalSecretRef.current) {
        payload.secret = data.secret
      }
      if (data.rate > 0 || data.burst > 0) {
        payload.rate_limit = JSON.stringify({ rate: data.rate, burst: data.burst })
      }
      if (editing) {
        return updateEndpoint(editing.id, payload)
      }
      return createEndpoint(payload)
    },
    onSuccess: () => {
      toast.success(editing ? t('endpoints.endpointUpdated') : t('endpoints.endpointCreated'))
      setDialogOpen(false)
      invalidate()
    },
    onError: (e) => {
      toast.error(getErrorMessage(e, t('endpoints.failedToSaveEndpoint')))
    },
  })

  const toggleMutation = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      // Only send {enabled} — minimal patch, keeps the secret untouched
      updateEndpoint(id, { enabled }),
    onSuccess: (_data, variables) => {
      toast.success(
        variables.enabled ? t('endpoints.endpointEnabled') : t('endpoints.endpointDisabled'),
      )
      invalidate()
    },
    onError: (e) => {
      toast.error(getErrorMessage(e, t('endpoints.failedToToggleEndpoint')))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteEndpoint(id),
    onSuccess: () => {
      toast.success(t('endpoints.endpointDeleted'))
      setDeleteTarget(null)
      invalidate()
    },
    onError: (e) => {
      toast.error(getErrorMessage(e, t('common.failedToSave', { entity: t('nav.endpoints') })))
    },
  })

  const form = useForm<EndpointForm>({
    resolver: zodResolver(endpointSchema),
    defaultValues: defaultForm,
  })
  const authType = form.watch('auth_type')

  function openCreate() {
    setEditing(null)
    originalSecretRef.current = ''
    form.reset(defaultForm)
    setDialogOpen(true)
  }

  function openEdit(row: EndpointResponse) {
    setEditing(row)
    // Server returns the decrypted secret; "configured"/"not_configured" are
    // fallback markers (decryption failure / nothing configured) — not values.
    const secret =
      row.secret === 'configured' || row.secret === 'not_configured' ? '' : (row.secret ?? '')
    originalSecretRef.current = secret
    form.reset({
      name: row.name,
      auth_type: (AUTH_TYPES.includes(row.auth_type as (typeof AUTH_TYPES)[number])
        ? row.auth_type
        : 'none') as EndpointForm['auth_type'],
      secret,
      maxBodyKb: Math.round((row.max_body_bytes ?? 1048576) / 1024),
      ...parseRateLimit(row.rate_limit),
      mapping_id: row.mapping_id ?? '',
      enabled: row.enabled,
    })
    setDialogOpen(true)
  }

  const rows = listQuery.data?.data ?? []
  const total = listQuery.data?.total ?? 0

  const webhookUrl = useMemo(
    () => (slug: string) => `${window.location.origin}/endpoint/${slug}`,
    [],
  )

  return (
    <div className="space-y-4">
      <PageHeader
        title={t('nav.endpoints')}
        description={t('endpoints.description')}
        actions={
          <>
            <RefreshButton
              label={t('dashboard.refresh')}
              onClick={() => void listQuery.refetch()}
              loading={listQuery.isFetching}
            />
            <Button size="sm" onClick={openCreate}>
              <Plus />
              {t('endpoints.newEndpoint')}
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
            entity={t('nav.endpoints')}
          />
        ) : rows.length === 0 ? (
          <EmptyState
            title={t('endpoints.emptyTitle')}
            description={t('endpoints.emptyDescription')}
            icon={<Webhook className="size-5" />}
            action={
              <Button size="sm" variant="outline" onClick={openCreate}>
                <Plus />
                {t('endpoints.newEndpoint')}
              </Button>
            }
          />
        ) : (
          <Table className="min-w-[560px]">
            <TableHeader>
              <TableRow className="group">
                <TableHead className="sticky left-0 z-20 bg-background group-hover:bg-[color-mix(in_oklab,var(--color-muted)_50%,var(--color-background))]">{t('common.name')}</TableHead>
                <TableHead>{t('endpoints.slug')}</TableHead>
                <TableHead>{t('endpoints.authType')}</TableHead>
                <TableHead className="w-full min-w-24">{t('endpoints.mapping')}</TableHead>
                <TableHead className="w-20">{t('common.enabled')}</TableHead>
                <TableHead className="sticky right-0 z-20 min-w-[108px] bg-background text-right group-hover:bg-[color-mix(in_oklab,var(--color-muted)_50%,var(--color-background))]">{t('common.actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((row) => (
                <TableRow key={row.id} className="group">
                  <TableCell className="sticky left-0 z-10 bg-background text-muted-foreground group-hover:bg-[color-mix(in_oklab,var(--color-muted)_50%,var(--color-background))]">{row.name}</TableCell>
                  <TableCell>
                    <span className="group/reveal inline-flex items-center gap-1">
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <button
                            type="button"
                            className="font-mono text-sm text-muted-foreground transition-colors hover:text-foreground"
                            onClick={async () => {
                              try {
                                await copyToClipboard(webhookUrl(row.slug))
                                toast.success(t('common.copied'))
                              } catch {
                                toast.error(t('common.copyFailed'))
                              }
                            }}
                          >
                            {row.slug}
                          </button>
                        </TooltipTrigger>
                        <TooltipContent>
                          <p className="font-mono text-xs">{webhookUrl(row.slug)}</p>
                        </TooltipContent>
                      </Tooltip>
                      <CopyButton text={webhookUrl(row.slug)} />
                    </span>
                  </TableCell>
                  <TableCell>
                    <span className="text-muted-foreground">
                      {t(`enums.authType.${row.auth_type}`, { defaultValue: row.auth_type })}
                    </span>
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {lookupName(mappingOptions.data, row.mapping_id)}
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
                  <TableCell className="sticky right-0 z-10 min-w-[108px] bg-background text-right group-hover:bg-[color-mix(in_oklab,var(--color-muted)_50%,var(--color-background))]">
                    <div className="flex justify-end gap-1">
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        className="size-7"
                        onClick={() => openEdit(row)}
                        aria-label={t('common.edit')}
                      >
                        <Pencil />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        className="size-7 text-destructive hover:text-destructive"
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
        <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-xl">
          <DialogHeader>
            <DialogTitle>
              {editing ? t('endpoints.editEndpoint') : t('endpoints.newEndpoint')}
            </DialogTitle>
            <DialogDescription className="sr-only">
              {editing ? t('endpoints.editEndpoint') : t('endpoints.newEndpoint')}
            </DialogDescription>
          </DialogHeader>

          <form
            className="space-y-4"
            onSubmit={form.handleSubmit((values) => saveMutation.mutate(values))}
          >
            <div className="grid grid-cols-[1fr_auto] gap-4">
              <div className="space-y-2">
                <Label htmlFor="endpoint-name">{t('common.name')}</Label>
                <Input
                  id="endpoint-name"
                  placeholder={t('common.name')}
                  {...form.register('name')}
                />
                {form.formState.errors.name ? (
                  <p className="text-xs text-destructive">{t('endpoints.nameRequired')}</p>
                ) : null}
              </div>
              <div className="space-y-2">
                <Label htmlFor="endpoint-enabled">{t('common.enabled')}</Label>
                <div className="flex h-9 items-center">
                  <Switch
                    id="endpoint-enabled"
                    checked={form.watch('enabled')}
                    onCheckedChange={(checked) => form.setValue('enabled', checked)}
                  />
                </div>
              </div>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label>{t('endpoints.authType')}</Label>
                <Select
                  value={authType}
                  onValueChange={(v) => form.setValue('auth_type', v as EndpointForm['auth_type'])}
                >
                  <SelectTrigger>
                    <SelectValue placeholder={t('endpoints.selectAuthType')} />
                  </SelectTrigger>
                  <SelectContent>
                    {AUTH_TYPES.map((type) => (
                      <SelectItem key={type} value={type}>
                        {t(`enums.authType.${type}`)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-2">
                <Label htmlFor="endpoint-max-body">{t('endpoints.maxBodyBytes')}</Label>
                <UnitInput
                  id="endpoint-max-body"
                  type="number"
                  min={0}
                  unit={t('endpoints.maxBodyUnit')}
                  {...form.register('maxBodyKb', { valueAsNumber: true })}
                />
              </div>
            </div>

            {authType !== 'none' ? (
              <div className="space-y-2">
                <Label htmlFor="endpoint-secret">{t('endpoints.secretToken')}</Label>
                <div className="flex gap-2">
                  <PasswordInput
                    id="endpoint-secret"
                    containerClassName="flex-1"
                    autoComplete="new-password"
                    placeholder={
                      editing
                        ? t('endpoints.secretEditPlaceholder')
                        : t('endpoints.secretPlaceholder')
                    }
                    {...form.register('secret')}
                  />
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => form.setValue('secret', generateSecret())}
                  >
                    {t('endpoints.generateSecret')}
                  </Button>
                </div>
              </div>
            ) : null}

            <div className="space-y-2">
              <Label>{t('endpoints.rateLimit')}</Label>
              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-1.5">
                  <Label htmlFor="endpoint-rate" className="text-xs text-muted-foreground">
                    {t('endpoints.rateLimitRate')}
                  </Label>
                  <Input
                    id="endpoint-rate"
                    type="number"
                    min={0}
                    placeholder={t('endpoints.rateLimitRatePlaceholder')}
                    {...form.register('rate', { valueAsNumber: true })}
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="endpoint-burst" className="text-xs text-muted-foreground">
                    {t('endpoints.rateLimitBurst')}
                  </Label>
                  <Input
                    id="endpoint-burst"
                    type="number"
                    min={0}
                    placeholder={t('endpoints.rateLimitBurstPlaceholder')}
                    {...form.register('burst', { valueAsNumber: true })}
                  />
                </div>
              </div>
              <p className="text-xs text-muted-foreground">{t('endpoints.rateLimitHint')}</p>
            </div>

            <div className="space-y-2">
              <Label>{t('endpoints.mapping')}</Label>
              <Select
                value={form.watch('mapping_id') || 'none'}
                onValueChange={(v) => form.setValue('mapping_id', v === 'none' ? '' : v)}
              >
                <SelectTrigger>
                  <SelectValue placeholder={t('endpoints.selectMapping')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="none">-</SelectItem>
                  {(mappingOptions.data ?? []).map((m) => (
                    <SelectItem key={m.id} value={m.id}>
                      {m.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
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
          entity: t('nav.endpoints'),
          name: deleteTarget?.name ?? '',
        })}
        confirmLabel={t('common.delete')}
        onConfirm={() => deleteTarget && deleteMutation.mutate(deleteTarget.id)}
        pending={deleteMutation.isPending}
      />
    </div>
  )
}
