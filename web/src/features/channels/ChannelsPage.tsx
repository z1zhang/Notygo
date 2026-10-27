import { useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { toast } from 'sonner'
import { Loader2, Pencil, Plus, Send, Trash2, Zap } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { createChannel, deleteChannel, listChannels, testChannel, updateChannel } from '@/lib/api'
import { getErrorMessage } from '@/lib/api-client'
import type { ChannelResponse } from '@/lib/types'
import { usePagination } from '@/hooks/use-pagination'
import { PageHeader } from '@/components/layout/PageHeader'
import { Badge } from '@/components/ui/badge'
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
import { Pagination } from '@/components/data-display/Pagination'
import { EmptyState } from '@/components/feedback/EmptyState'
import { ErrorState } from '@/components/feedback/ErrorState'
import { TableSkeleton } from '@/components/data-display/TableSkeleton'
import { ConfirmDialog } from '@/components/feedback/ConfirmDialog'
import { RefreshButton } from '@/components/feedback/RefreshButton'
import {
  CHANNEL_TYPES,
  channelModule,
  defaultForm,
  channelResolver,
  uniqueModules,
  type ChannelForm,
} from './modules'

export default function ChannelsPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { page, size, setPage, setSize } = usePagination()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<ChannelResponse | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<ChannelResponse | null>(null)
  // Server returns the DECRYPTED secret on edit — only re-submit when changed.
  const originalSecretRef = useRef('')

  const listQuery = useQuery({
    queryKey: ['channels', page, size],
    queryFn: () => listChannels({ page, size }),
  })

  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: ['channels'] })
    void queryClient.invalidateQueries({ queryKey: ['lookup', 'channels'] })
  }

  // Channel update is partial — the toggle can safely send {enabled} only.
  // Revert the switch on error by re-fetching the list.
  const toggleMutation = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      updateChannel(id, { enabled }),
    onSuccess: (_data, variables) => {
      toast.success(
        variables.enabled
          ? t('channels.channelEnabled')
          : t('channels.channelDisabled'),
      )
      invalidate()
    },
    onError: (e) => {
      toast.error(getErrorMessage(e, t('channels.failedToToggleChannel')))
      void queryClient.invalidateQueries({ queryKey: ['channels'] })
    },
  })

  const saveMutation = useMutation({
    mutationFn: async (data: ChannelForm) => {
      const payload: Record<string, unknown> = {
        name: data.name,
        type: data.type,
        config: channelModule(data.type).toConfig(data),
        timeout_ms: data.timeout_ms,
        max_attempts: data.max_attempts,
        enabled: data.enabled,
      }
      if (data.secret !== '' && data.secret !== originalSecretRef.current) {
        payload.secret = data.secret
      }
      if (editing) {
        return updateChannel(editing.id, payload)
      }
      return createChannel(payload)
    },
    onSuccess: () => {
      toast.success(
        editing ? t('channels.channelUpdated') : t('channels.channelCreated'),
      )
      setDialogOpen(false)
      invalidate()
    },
    onError: (e) => {
      toast.error(getErrorMessage(e, t('channels.failedToSaveChannel')))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteChannel(id),
    onSuccess: () => {
      toast.success(t('channels.channelDeleted'))
      setDeleteTarget(null)
      invalidate()
    },
    onError: (e) => {
      toast.error(getErrorMessage(e, t('common.failedToSave', { entity: t('nav.channels') })))
    },
  })

  // Test-send goes through the same sender path as real deliveries
  const [testingId, setTestingId] = useState<string | null>(null)
  const testMutation = useMutation({
    mutationFn: (id: string) => {
      setTestingId(id)
      return testChannel(id)
    },
    onSuccess: () => {
      toast.success(t('channels.testSuccess'))
    },
    onError: (e) => {
      toast.error(getErrorMessage(e, t('channels.testFailed')))
    },
    onSettled: () => {
      setTestingId(null)
    },
  })

  const form = useForm<ChannelForm>({
    resolver: channelResolver,
    defaultValues: defaultForm,
  })
  const type = form.watch('type')
  const ActiveFields = channelModule(type).Fields

  function openCreate() {
    setEditing(null)
    originalSecretRef.current = ''
    form.reset(defaultForm)
    setDialogOpen(true)
  }

  function openEdit(row: ChannelResponse) {
    setEditing(row)
    originalSecretRef.current = row.secret ?? ''
    const config = (row.config ?? {}) as Record<string, unknown>
    const rowType = (CHANNEL_TYPES.includes(row.type as (typeof CHANNEL_TYPES)[number])
      ? row.type
      : 'http') as ChannelForm['type']
    const module = channelModule(rowType)
    form.reset({
      name: row.name,
      type: rowType,
      secret: row.secret ?? '',
      timeout_ms: typeof row.timeout_ms === 'number' && Number.isFinite(row.timeout_ms)
        ? row.timeout_ms
        : 10000,
      max_attempts: typeof row.max_attempts === 'number' && row.max_attempts >= 1
        ? row.max_attempts
        : 5,
      enabled: row.enabled,
      ...allChannelDefaults(),
      ...module.fromConfig(config, rowType),
    })
    setDialogOpen(true)
  }

  // Defaults of every module — the base for a clean form state; only the
  // active channel's fromConfig values are layered on top.
  function allChannelDefaults(): Record<string, unknown> {
    return Object.assign({}, ...uniqueModules().map((m) => m.defaults))
  }

  // Switching type resets the type-specific config fields. Module defaults are
  // keyed dynamically, so the setter is loosened like the body-field editor.
  const setAnyValue = (path: string, value: unknown) =>
    (form.setValue as (p: string, v: unknown) => void)(path, value)

  function handleTypeChange(next: string) {
    const nextType = next as ChannelForm['type']
    form.setValue('type', nextType)
    for (const [key, value] of Object.entries(allChannelDefaults())) {
      setAnyValue(key, value)
    }
    const module = channelModule(nextType)
    if (module.defaultMsgType) {
      setAnyValue('msg_type', module.defaultMsgType[nextType] ?? '')
    }
  }

  const rows = listQuery.data?.data ?? []
  const total = listQuery.data?.total ?? 0

  return (
    <div className="space-y-4">
      <PageHeader
        title={t('nav.channels')}
        description={t('channels.description')}
        actions={
          <>
            <RefreshButton
              label={t('dashboard.refresh')}
              onClick={() => void listQuery.refetch()}
              loading={listQuery.isFetching}
            />
            <Button size="sm" onClick={openCreate}>
              <Plus />
              {t('channels.newChannel')}
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
            entity={t('nav.channels')}
          />
        ) : rows.length === 0 ? (
          <EmptyState
            title={t('channels.emptyTitle')}
            description={t('channels.emptyDescription')}
            icon={<Send className="size-5" />}
            action={
              <Button size="sm" variant="outline" onClick={openCreate}>
                <Plus />
                {t('channels.newChannel')}
              </Button>
            }
          />
        ) : (
          <Table className="min-w-[420px]">
            <TableHeader>
              <TableRow className="group">
                <TableHead className="sticky left-0 z-20 bg-background group-hover:bg-[color-mix(in_oklab,var(--color-muted)_50%,var(--color-background))]">{t('common.name')}</TableHead>
                <TableHead className="w-full min-w-32">{t('common.type')}</TableHead>
                <TableHead className="w-20">{t('common.enabled')}</TableHead>
                <TableHead className="sticky right-0 z-20 min-w-[108px] bg-background text-right group-hover:bg-[color-mix(in_oklab,var(--color-muted)_50%,var(--color-background))]">{t('common.actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((row) => (
                <TableRow key={row.id} className="group">
                  <TableCell className="sticky left-0 z-10 bg-background text-muted-foreground group-hover:bg-[color-mix(in_oklab,var(--color-muted)_50%,var(--color-background))]">{row.name}</TableCell>
                  <TableCell>
                    <Badge variant="secondary">
                      {t(`enums.channelType.${row.type}`, { defaultValue: row.type })}
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
                  <TableCell className="sticky right-0 z-10 min-w-[108px] bg-background text-right group-hover:bg-[color-mix(in_oklab,var(--color-muted)_50%,var(--color-background))]">
                    <div className="flex justify-end gap-1">
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        className="size-7"
                        disabled={testMutation.isPending}
                        onClick={() => testMutation.mutate(row.id)}
                        aria-label={t('channels.test')}
                      >
                        {testingId === row.id ? (
                          <Loader2 className="animate-spin" />
                        ) : (
                          <Zap />
                        )}
                      </Button>
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
        <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>
              {editing ? t('channels.editChannel') : t('channels.newChannel')}
            </DialogTitle>
            <DialogDescription className="sr-only">
              {editing ? t('channels.editChannel') : t('channels.newChannel')}
            </DialogDescription>
          </DialogHeader>

          <form
            className="space-y-4"
            onSubmit={form.handleSubmit((values) => saveMutation.mutate(values))}
          >
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-[minmax(0,1fr)_176px_60px]">
              <div className="space-y-2">
                <Label htmlFor="channel-name">{t('common.name')}</Label>
                <Input
                  id="channel-name"
                  placeholder={t('channels.channelName')}
                  {...form.register('name')}
                />
                {form.formState.errors.name ? (
                  <p className="text-xs text-destructive">{t('channels.nameRequired')}</p>
                ) : null}
              </div>

              <div className="space-y-2">
                <Label>{t('common.type')}</Label>
                <Select value={type} onValueChange={handleTypeChange}>
                  <SelectTrigger className="w-full">
                    <SelectValue placeholder={t('channels.selectType')} />
                  </SelectTrigger>
                  <SelectContent>
                    {CHANNEL_TYPES.map((channelType) => (
                      <SelectItem key={channelType} value={channelType}>
                        {t(`enums.channelType.${channelType}`)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-2">
                <Label htmlFor="channel-enabled">{t('common.enabled')}</Label>
                <div className="flex h-9 items-center">
                  <Switch
                    id="channel-enabled"
                    checked={form.watch('enabled')}
                    onCheckedChange={(checked) => form.setValue('enabled', checked)}
                  />
                </div>
              </div>
            </div>

            {/* Type-specific config: rendered by the channel module */}
            {ActiveFields ? <ActiveFields form={form as never} /> : null}

            {/* Common config */}
            <div className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="channel-secret">{t('channels.secret')}</Label>
                <PasswordInput
                  id="channel-secret"
                  autoComplete="new-password"
                  placeholder={
                    editing
                      ? t('channels.secretEditPlaceholder')
                      : t('channels.secretPlaceholder')
                  }
                  {...form.register('secret')}
                />
              </div>

              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div className="space-y-2">
                  <Label htmlFor="channel-timeout">{t('channels.timeoutMs')}</Label>
                  <UnitInput
                    id="channel-timeout"
                    type="number"
                    min={1000}
                    step={1000}
                    unit={t('channels.msUnit')}
                    {...form.register('timeout_ms', { valueAsNumber: true })}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="channel-max-attempts">{t('channels.maxAttempts')}</Label>
                  <Input
                    id="channel-max-attempts"
                    type="number"
                    min={1}
                    max={100}
                    {...form.register('max_attempts', { valueAsNumber: true })}
                  />
                </div>
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
          entity: t('nav.channels'),
          name: deleteTarget?.name ?? '',
        })}
        confirmLabel={t('common.delete')}
        onConfirm={() => deleteTarget && deleteMutation.mutate(deleteTarget.id)}
        pending={deleteMutation.isPending}
      />
    </div>
  )
}
