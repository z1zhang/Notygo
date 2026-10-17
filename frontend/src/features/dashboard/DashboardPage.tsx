import { useMemo } from 'react'
import { useNavigate } from 'react-router'
import { useQuery } from '@tanstack/react-query'
import { AlertTriangle, Bell, CircleX, LoaderCircle, Send } from 'lucide-react'
import { useI18n } from '@/i18n'
import { listDeliveries, listEvents } from '@/lib/api'
import { formatTime } from '@/lib/formatters'
import { lookupName, useDestinationOptions, useSourceOptions } from '@/hooks/use-lookup'
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
  const { t } = useI18n()
  const navigate = useNavigate()

  const totalEventsQuery = useQuery({
    queryKey: ['dashboard', 'events', 'total'],
    queryFn: () => listEvents({ page: 1, size: 1 }),
  })
  const processingQuery = useQuery({
    queryKey: ['dashboard', 'events', 'processing'],
    queryFn: () => listEvents({ page: 1, size: 1, status: 'processing' }),
  })
  const pendingQuery = useQuery({
    queryKey: ['dashboard', 'deliveries', 'pending'],
    queryFn: () => listDeliveries({ page: 1, size: 1, status: 'pending' }),
  })
  const failedDeliveriesQuery = useQuery({
    queryKey: ['dashboard', 'deliveries', 'failed'],
    queryFn: () => listDeliveries({ page: 1, size: 1, status: 'failed' }),
  })

  const failedEventsQuery = useQuery({
    queryKey: ['dashboard', 'recent-errors', 'events'],
    queryFn: () => listEvents({ page: 1, size: 5, status: 'failed' }),
  })
  const failedDeliveriesListQuery = useQuery({
    queryKey: ['dashboard', 'recent-errors', 'deliveries'],
    queryFn: () => listDeliveries({ page: 1, size: 5, status: 'failed' }),
  })

  const sourceOptions = useSourceOptions()
  const destinationOptions = useDestinationOptions()

  const statQueries = [
    { ...statMeta[0], query: totalEventsQuery },
    { ...statMeta[1], query: processingQuery },
    { ...statMeta[2], query: pendingQuery },
    { ...statMeta[3], query: failedDeliveriesQuery },
  ]

  const errorQueries = [failedEventsQuery, failedDeliveriesListQuery]

  const errorRows = useMemo<ErrorRow[]>(() => {
    const eventErrors: ErrorRow[] = (failedEventsQuery.data?.data ?? []).map((e) => ({
      type: 'event',
      id: e.id,
      refId: e.source_id,
      status: e.status,
      error: e.error_summary ?? e.error_code ?? '',
      createdAt: e.received_at ?? '',
    }))
    const deliveryErrors: ErrorRow[] = (failedDeliveriesListQuery.data?.data ?? []).map((d) => ({
      type: 'delivery',
      id: d.id,
      refId: d.destination_id,
      status: d.status,
      error: d.last_error_summary ?? d.last_error_code ?? '',
      createdAt: d.created_at ?? '',
    }))
    return [...eventErrors, ...deliveryErrors]
      .sort((a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime())
      .slice(0, 10)
  }, [failedEventsQuery.data, failedDeliveriesListQuery.data])

  function refreshAll() {
    for (const { query } of statQueries) void query.refetch()
    for (const q of errorQueries) void q.refetch()
  }

  const errorsLoading = errorQueries.some((q) => q.isPending)
  const errorsFailed = errorQueries.find((q) => q.isError)

  return (
    <div className="space-y-4">
      <PageHeader
        title={t('nav.dashboard')}
        actions={
          <RefreshButton
            label={t('dashboard.refresh')}
            onClick={refreshAll}
            loading={
              statQueries.some(({ query }) => query.isFetching) ||
              errorQueries.some((q) => q.isFetching)
            }
          />
        }
      />

      {/* Stat tiles — all-time totals from the API (no time filter exists) */}
      <div className="grid grid-cols-2 gap-4 xl:grid-cols-4">
        {statQueries.map(({ key, query, icon: Icon, iconClass }) => (
          <div key={key} className="flex items-center gap-4 rounded-lg border px-5 py-4">
            <div className={cn('flex size-10 shrink-0 items-center justify-center rounded-lg', iconClass)}>
              <Icon className="size-5" />
            </div>
            <div className="min-w-0">
              <p className="text-sm text-muted-foreground">{t(`dashboard.${key}`)}</p>
              <p className="text-2xl font-semibold tabular-nums">
                {query.isError ? '-' : (query.data?.total ?? 0)}
              </p>
            </div>
          </div>
        ))}
      </div>

      {/* Recent errors */}
      <div className="rounded-lg border">
        <div className="flex items-center justify-between border-b px-4 py-3">
          <h2 className="text-sm font-semibold">{t('dashboard.recentErrors')}</h2>
          <RefreshButton
            variant="ghost"
            label={t('dashboard.refresh')}
            onClick={() => {
              for (const q of errorQueries) void q.refetch()
            }}
            loading={errorQueries.some((q) => q.isFetching)}
          />
        </div>

        {errorsLoading ? (
          <TableSkeleton cols={6} />
        ) : errorsFailed ? (
          <ErrorState
            error={errorsFailed.error}
            onRetry={() => {
              for (const q of errorQueries) void q.refetch()
            }}
            entity={t('dashboard.recentErrors')}
          />
        ) : errorRows.length === 0 ? (
          <EmptyState
            title={t('dashboard.noErrors')}
            icon={<AlertTriangle className="size-5" />}
          />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-20">{t('common.type')}</TableHead>
                <TableHead className="min-w-40">{t('common.id')}</TableHead>
                <TableHead className="min-w-32">{t('dashboard.sourceOrTarget')}</TableHead>
                <TableHead className="w-28">{t('common.status')}</TableHead>
                <TableHead className="min-w-48">{t('common.error')}</TableHead>
                <TableHead className="min-w-36">{t('common.time')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {errorRows.map((row) => (
                <TableRow
                  key={`${row.type}-${row.id}`}
                  className="cursor-pointer"
                  onClick={() => navigate(row.type === 'event' ? '/events' : '/deliveries')}
                >
                  <TableCell>
                    <span
                      className={cn(
                        'inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium',
                        row.type === 'event'
                          ? 'bg-amber-500/10 text-amber-700 dark:text-amber-400'
                          : 'bg-red-500/10 text-red-700 dark:text-red-400',
                      )}
                    >
                      {t(`enums.errorType.${row.type}`, row.type)}
                    </span>
                  </TableCell>
                  <TableCell>
                    <TruncatedId id={row.id} />
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {row.type === 'event'
                      ? lookupName(sourceOptions.data, row.refId)
                      : lookupName(destinationOptions.data, row.refId)}
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
