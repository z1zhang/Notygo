import { useMemo } from 'react'
import { useNavigate } from 'react-router'
import { useQuery } from '@tanstack/react-query'
import { AlertTriangle, Bell, CircleX, ListChecks, LoaderCircle, Send } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { getDashboardStats, getQueueStats } from '@/lib/api'
import { formatTime } from '@/lib/formatters'
import { lookupName, useChannelOptions, useEndpointOptions } from '@/hooks/use-lookup'
import { PageHeader } from '@/components/layout/PageHeader'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { StatusBadge } from '@/components/data-display/StatusBadge'
import { TableSkeleton } from '@/components/data-display/TableSkeleton'
import { TruncatedId } from '@/components/data-display/TruncatedId'
import { EmptyState } from '@/components/feedback/EmptyState'
import { ErrorState } from '@/components/feedback/ErrorState'
import { RefreshButton } from '@/components/feedback/RefreshButton'
import { cn } from '@/lib/utils'
import type { QueueStatsRow } from '@/lib/types'

interface ErrorRow {
  type: 'event' | 'delivery'
  id: string
  refId: string
  status: string
  error: string
  createdAt: string
}

const statMeta = [
  { key: 'totalEvents', icon: Bell, iconClass: 'bg-sky-500/10 text-sky-600 dark:text-sky-400' },
  {
    key: 'processing',
    icon: LoaderCircle,
    iconClass: 'bg-amber-500/10 text-amber-600 dark:text-amber-400',
  },
  {
    key: 'pendingDeliveries',
    icon: Send,
    iconClass: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400',
  },
  {
    key: 'failedDeliveries',
    icon: CircleX,
    iconClass: 'bg-red-500/10 text-red-600 dark:text-red-400',
  },
] as const

export default function DashboardPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()

  // Single atomic stats request: counts + recent failures in one round trip
  const statsQuery = useQuery({
    queryKey: ['dashboard', 'stats'],
    queryFn: () => getDashboardStats(),
  })

  // River queue health: river_job counts by queue and state
  const queueQuery = useQuery({
    queryKey: ['dashboard', 'queue-stats'],
    queryFn: () => getQueueStats(),
  })

  const endpointOptions = useEndpointOptions()
  const channelOptions = useChannelOptions()

  const stats = statsQuery.data
  const statsValues = [
    stats?.total_events ?? 0,
    stats?.processing_events ?? 0,
    stats?.pending_deliveries ?? 0,
    stats?.failed_deliveries ?? 0,
  ]
  const statQueries = statMeta.map((meta, i) => ({ ...meta, value: statsValues[i] }))

  const errorRows = useMemo<ErrorRow[]>(() => {
    const eventErrors: ErrorRow[] = (stats?.recent_failed_events ?? []).map((e) => ({
      type: 'event',
      id: e.id,
      refId: e.endpoint_id,
      status: e.status,
      error: e.error_summary ?? e.error_code ?? '',
      createdAt: e.received_at ?? '',
    }))
    const deliveryErrors: ErrorRow[] = (stats?.recent_failed_deliveries ?? []).map((d) => ({
      type: 'delivery',
      id: d.id,
      refId: d.channel_id,
      status: d.status,
      error: d.last_error_summary ?? d.last_error_code ?? '',
      createdAt: d.created_at ?? '',
    }))
    return [...eventErrors, ...deliveryErrors]
      .sort((a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime())
      .slice(0, 10)
  }, [stats])

  function refreshAll() {
    void statsQuery.refetch()
    void queueQuery.refetch()
  }

  // Group river_job rows by queue for the queue-health card
  const queueGroups = useMemo(() => {
    const rows = queueQuery.data?.queues ?? []
    const byQueue = new Map<string, QueueStatsRow[]>()
    for (const row of rows) {
      const list = byQueue.get(row.queue) ?? []
      list.push(row)
      byQueue.set(row.queue, list)
    }
    return [...byQueue.entries()].map(([queue, states]) => ({
      queue,
      states: states.filter((s) => s.count > 0),
    }))
  }, [queueQuery.data])

  // Stats scope: terminal-state counts roll off after River's retention
  // window, so the card is labeled "last 24h" etc.
  const queueWindow = useMemo(() => {
    const s = queueQuery.data?.retention_seconds
    if (!s || s <= 0) return ''
    if (s < 3600) {
      return t('dashboard.queueWindowMinutes', { n: Math.max(1, Math.round(s / 60)) })
    }
    return t('dashboard.queueWindowHours', { n: Math.max(1, Math.round(s / 3600)) })
  }, [queueQuery.data?.retention_seconds, t])

  const errorsLoading = statsQuery.isPending
  const errorsFailed = statsQuery.isError ? statsQuery : undefined

  return (
    <div className="space-y-4">
      <PageHeader
        title={t('nav.dashboard')}
        actions={
          <RefreshButton
            label={t('dashboard.refresh')}
            onClick={refreshAll}
            loading={statsQuery.isFetching}
          />
        }
      />

      {/* Stat tiles — all-time totals from the API (no time filter exists) */}
      <div className="grid grid-cols-2 gap-4 xl:grid-cols-4">
        {statQueries.map(({ key, value, icon: Icon, iconClass }) => (
          <div key={key} className="flex items-center gap-4 rounded-lg border px-5 py-4">
            <div className={cn('flex size-10 shrink-0 items-center justify-center rounded-lg', iconClass)}>
              <Icon className="size-5" />
            </div>
            <div className="min-w-0">
              <p className="text-sm text-muted-foreground">{t(`dashboard.${key}`)}</p>
              <p className="text-2xl font-semibold tabular-nums">
                {statsQuery.isError ? '-' : value}
              </p>
            </div>
          </div>
        ))}
      </div>

      {/* Queue health (River job counts) */}
      <div className="rounded-lg border">
        <div className="flex items-center justify-between border-b px-4 py-3">
          <h2 className="flex items-center gap-2 text-sm font-semibold">
            <ListChecks className="size-4 text-muted-foreground" />
            {t('dashboard.queueStats')}
            {queueWindow && (
              <span className="text-xs font-normal text-muted-foreground">{queueWindow}</span>
            )}
          </h2>
          <RefreshButton
            variant="ghost"
            label={t('dashboard.refresh')}
            onClick={() => void queueQuery.refetch()}
            loading={queueQuery.isFetching}
          />
        </div>
        <div className="space-y-2 px-4 py-3">
          {queueQuery.isPending ? (
            <p className="text-sm text-muted-foreground">…</p>
          ) : queueGroups.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t('dashboard.queueIdle')}</p>
          ) : (
            queueGroups.map(({ queue, states }) => (
              <div key={queue} className="flex flex-wrap items-center gap-2">
                <span className="min-w-20 text-sm text-muted-foreground">
                  {t(`dashboard.queueName.${queue}`, { defaultValue: queue })}
                </span>
                {states.map((s) => (
                  <span
                    key={s.state}
                    className="inline-flex items-center gap-1.5 rounded-md bg-muted px-2 py-0.5 text-xs text-muted-foreground"
                  >
                    {t(`enums.queueState.${s.state}`, { defaultValue: s.state })}
                    <span className="text-foreground tabular-nums">{s.count}</span>
                  </span>
                ))}
              </div>
            ))
          )}
        </div>
      </div>

      {/* Recent errors */}
      <div className="rounded-lg border">
        <div className="flex items-center justify-between border-b px-4 py-3">
          <h2 className="text-sm font-semibold">{t('dashboard.recentErrors')}</h2>
          <RefreshButton
            variant="ghost"
            label={t('dashboard.refresh')}
            onClick={() => void statsQuery.refetch()}
            loading={statsQuery.isFetching}
          />
        </div>

        {errorsLoading ? (
          <TableSkeleton cols={6} />
        ) : errorsFailed ? (
          <ErrorState
            error={errorsFailed.error}
            onRetry={() => void statsQuery.refetch()}
            entity={t('dashboard.recentErrors')}
          />
        ) : errorRows.length === 0 ? (
          <EmptyState
            title={t('dashboard.noErrors')}
            icon={<AlertTriangle className="size-5" />}
          />
        ) : (
          <Table className="min-w-[560px]">
            <TableHeader>
              <TableRow className="group">
                <TableHead className="sticky left-0 z-20 w-20 bg-background group-hover:bg-[color-mix(in_oklab,var(--color-muted)_50%,var(--color-background))]">{t('common.type')}</TableHead>
                <TableHead className="min-w-40">{t('common.id')}</TableHead>
                <TableHead className="min-w-32">{t('dashboard.endpointOrTarget')}</TableHead>
                <TableHead className="w-28">{t('common.status')}</TableHead>
                <TableHead className="min-w-48">{t('common.error')}</TableHead>
                <TableHead className="min-w-36">{t('common.time')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {errorRows.map((row) => (
                <TableRow
                  key={`${row.type}-${row.id}`}
                  className="group cursor-pointer"
                  onClick={() => navigate(row.type === 'event' ? '/events' : '/deliveries')}
                >
                  <TableCell className="sticky left-0 z-10 bg-background group-hover:bg-[color-mix(in_oklab,var(--color-muted)_50%,var(--color-background))]">
                    <span
                      className={cn(
                        'inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium',
                        row.type === 'event'
                          ? 'bg-amber-500/10 text-amber-700 dark:text-amber-400'
                          : 'bg-red-500/10 text-red-700 dark:text-red-400',
                      )}
                    >
                      {t(`enums.errorType.${row.type}`, { defaultValue: row.type })}
                    </span>
                  </TableCell>
                  <TableCell>
                    <TruncatedId id={row.id} />
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {row.type === 'event'
                      ? lookupName(endpointOptions.data, row.refId)
                      : lookupName(channelOptions.data, row.refId)}
                  </TableCell>
                  <TableCell>
                    <StatusBadge kind={row.type} status={row.status} />
                  </TableCell>
                  <TableCell>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <span className="block max-w-64 truncate text-sm text-muted-foreground">
                          {row.error || '-'}
                        </span>
                      </TooltipTrigger>
                      <TooltipContent className="max-w-sm">
                        <p className="whitespace-pre-wrap break-words text-xs">
                          {row.error || '-'}
                        </p>
                      </TooltipContent>
                    </Tooltip>
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {formatTime(row.createdAt)}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </div>
    </div>
  )
}
