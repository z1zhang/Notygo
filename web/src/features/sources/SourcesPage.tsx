import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { toast } from 'sonner'
import { Loader2, Pencil, Plus, Trash2, Webhook } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { createSource, deleteSource, listSources, updateSource } from '@/lib/api'
import { getErrorMessage } from '@/lib/api-client'
import { copyToClipboard } from '@/lib/formatters'
import type { SourceResponse } from '@/lib/types'
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

const sourceSchema = z.object({
  name: z.string().min(1),
  auth_type: z.enum(AUTH_TYPES),
  secret: z.string(),
  maxBodyKb: z.number().min(0),
  rate: z.number().min(0),
  burst: z.number().min(0),
  mapping_id: z.string(),
  enabled: z.boolean(),
})

type SourceForm = z.infer<typeof sourceSchema>

const defaultForm: SourceForm = {
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

export default function SourcesPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { page, size, setPage, setSize } = usePagination()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<SourceResponse | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<SourceResponse | null>(null)

  const listQuery = useQuery({
    queryKey: ['sources', page, size],
    queryFn: () => listSources({ page, size }),
  })
  const mappingOptions = useMappingOptions()

  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: ['sources'] })
    void queryClient.invalidateQueries({ queryKey: ['lookup', 'sources'] })
  }

  const saveMutation = useMutation({
    mutationFn: async (data: SourceForm) => {
      const payload: Record<string, unknown> = {
        name: data.name,
        auth_type: data.auth_type,
        max_body_bytes: Math.round(data.maxBodyKb) * 1024,
        enabled: data.enabled,
        mapping_id: data.mapping_id || null,
      }
      // Never re-submit the server-side "configured"/"not_configured" marker
      if (data.auth_type !== 'none' && data.secret !== '') {
        payload.secret_ref = data.secret
      }
      if (data.rate > 0 || data.burst > 0) {
        payload.rate_limit = JSON.stringify({ rate: data.rate, burst: data.burst })
      }
      if (editing) {
        return updateSource(editing.id, payload)
      }
      return createSource(payload)
    },
    onSuccess: () => {
      toast.success(editing ? t('sources.sourceUpdated') : t('sources.sourceCreated'))
      setDialogOpen(false)
      invalidate()
    },
    onError: (e) => {
      toast.error(getErrorMessage(e, t('sources.failedToSaveSource')))
    },
  })

  const toggleMutation = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      // Only send {enabled} — avoids re-posting the masked secret marker
      updateSource(id, { enabled }),
    onSuccess: (_data, variables) => {
      toast.success(
        variables.enabled ? t('sources.sourceEnabled') : t('sources.sourceDisabled'),
      )
      invalidate()
    },
    onError: (e) => {
      toast.error(getErrorMessage(e, t('sources.failedToToggleSource')))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteSource(id),
    onSuccess: () => {
      toast.success(t('sources.sourceDeleted'))
      setDeleteTarget(null)
      invalidate()
    },
    onError: (e) => {
      toast.error(getErrorMessage(e, t('common.failedToSave', { entity: t('nav.sources') })))
    },
  })

  const form = useForm<SourceForm>({
    resolver: zodResolver(sourceSchema),
    defaultValues: defaultForm,
  })
  const authType = form.watch('auth_type')

  function openCreate() {
    setEditing(null)
    form.reset(defaultForm)
    setDialogOpen(true)
  }

  function openEdit(row: SourceResponse) {
    setEditing(row)
    form.reset({
      name: row.name,
      auth_type: (AUTH_TYPES.includes(row.auth_type as (typeof AUTH_TYPES)[number])
        ? row.auth_type
        : 'none') as SourceForm['auth_type'],
      secret: '', // server sends a marker; keep blank = keep existing
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
    () => (slug: string) => `${window.location.origin}/ingest/${slug}`,
    [],
  )

  return (
    <div className="space-y-4">
      <PageHeader
        title={t('nav.sources')}
        description={t('sources.description')}
        actions={
          <>
            <RefreshButton
              label={t('dashboard.refresh')}
              onClick={() => void listQuery.refetch()}
              loading={listQuery.isFetching}
            />
            <Button size="sm" onClick={openCreate}>
              <Plus />
              {t('sources.newSource')}
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
            entity={t('nav.sources')}
          />
        ) : rows.length === 0 ? (
          <EmptyState
            title={t('sources.emptyTitle')}
            description={t('sources.emptyDescription')}
            icon={<Webhook className="size-5" />}
            action={
              <Button size="sm" variant="outline" onClick={openCreate}>
                <Plus />
                {t('sources.newSource')}
              </Button>
            }
          />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('common.name')}</TableHead>
                <TableHead>{t('sources.slug')}</TableHead>
                <TableHead>{t('sources.authType')}</TableHead>
                <TableHead>{t('sources.mapping')}</TableHead>
                <TableHead>{t('common.enabled')}</TableHead>
                <TableHead className="w-24 text-right">{t('common.actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((row) => (
                <TableRow key={row.id}>
                  <TableCell className="text-muted-foreground">{row.name}</TableCell>
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
        <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-xl">
          <DialogHeader>
            <DialogTitle>
              {editing ? t('sources.editSource') : t('sources.newSource')}
            </DialogTitle>
            <DialogDescription className="sr-only">
              {editing ? t('sources.editSource') : t('sources.newSource')}
            </DialogDescription>
          </DialogHeader>

          <form
            className="space-y-4"
            onSubmit={form.handleSubmit((values) => saveMutation.mutate(values))}
          >
            <div className="grid grid-cols-[1fr_auto] gap-4">
              <div className="space-y-2">
                <Label htmlFor="source-name">{t('common.name')}</Label>
                <Input
                  id="source-name"
                  placeholder={t('common.name')}
                  {...form.register('name')}
                />
                {form.formState.errors.name ? (
                  <p className="text-xs text-destructive">{t('sources.nameRequired')}</p>
                ) : null}
              </div>
              <div className="space-y-2">
                <Label htmlFor="source-enabled">{t('common.enabled')}</Label>
                <div className="flex h-9 items-center">
                  <Switch
                    id="source-enabled"
                    checked={form.watch('enabled')}
                    onCheckedChange={(checked) => form.setValue('enabled', checked)}
                  />
                </div>
              </div>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label>{t('sources.authType')}</Label>
                <Select
                  value={authType}
                  onValueChange={(v) => form.setValue('auth_type', v as SourceForm['auth_type'])}
                >
                  <SelectTrigger>
                    <SelectValue placeholder={t('sources.selectAuthType')} />
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
                <Label htmlFor="source-max-body">{t('sources.maxBodyBytes')}</Label>
                <UnitInput
                  id="source-max-body"
                  type="number"
                  min={0}
                  unit={t('sources.maxBodyUnit')}
                  {...form.register('maxBodyKb', { valueAsNumber: true })}
                />
              </div>
            </div>

            {authType !== 'none' ? (
              <div className="space-y-2">
                <Label htmlFor="source-secret">{t('sources.secretToken')}</Label>
                <div className="flex gap-2">
                  <PasswordInput
                    id="source-secret"
                    containerClassName="flex-1"
                    autoComplete="new-password"
                    placeholder={
                      editing
                        ? t('sources.secretEditPlaceholder')
                        : t('sources.secretPlaceholder')
                    }
                    {...form.register('secret')}
                  />
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => form.setValue('secret', generateSecret())}
                  >
                    {t('sources.generateSecret')}
                  </Button>
                </div>
              </div>
            ) : null}

            <div className="space-y-2">
              <Label>{t('sources.rateLimit')}</Label>
              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-1.5">
                  <Label htmlFor="source-rate" className="text-xs text-muted-foreground">
                    {t('sources.rateLimitRate')}
                  </Label>
                  <Input
                    id="source-rate"
                    type="number"
                    min={0}
                    placeholder={t('sources.rateLimitRatePlaceholder')}
                    {...form.register('rate', { valueAsNumber: true })}
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="source-burst" className="text-xs text-muted-foreground">
                    {t('sources.rateLimitBurst')}
                  </Label>
                  <Input
                    id="source-burst"
                    type="number"
                    min={0}
                    placeholder={t('sources.rateLimitBurstPlaceholder')}
                    {...form.register('burst', { valueAsNumber: true })}
                  />
                </div>
              </div>
              <p className="text-xs text-muted-foreground">{t('sources.rateLimitHint')}</p>
            </div>

            <div className="space-y-2">
              <Label>{t('sources.mapping')}</Label>
              <Select
                value={form.watch('mapping_id') || 'none'}
                onValueChange={(v) => form.setValue('mapping_id', v === 'none' ? '' : v)}
              >
                <SelectTrigger>
                  <SelectValue placeholder={t('sources.selectMapping')} />
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
          entity: t('nav.sources'),
          name: deleteTarget?.name ?? '',
        })}
        confirmLabel={t('common.delete')}
        onConfirm={() => deleteTarget && deleteMutation.mutate(deleteTarget.id)}
        pending={deleteMutation.isPending}
      />
    </div>
  )
}
