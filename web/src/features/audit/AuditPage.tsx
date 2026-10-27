import { useState } from 'react'
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { format } from 'date-fns'
import { ScrollText, X } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { listAuditLogs } from '@/lib/api'
import { formatTime } from '@/lib/formatters'
import { usePagination } from '@/hooks/use-pagination'
import { PageHeader } from '@/components/layout/PageHeader'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { DateRangePicker, type DateRangeValue } from '@/components/data-display/DateRangePicker'
import { Pagination } from '@/components/data-display/Pagination'
import { StatusBadge } from '@/components/data-display/StatusBadge'
import { TableSkeleton } from '@/components/data-display/TableSkeleton'
import { TruncatedId } from '@/components/data-display/TruncatedId'
import { EmptyState } from '@/components/feedback/EmptyState'
import { ErrorState } from '@/components/feedback/ErrorState'
import { RefreshButton } from '@/components/feedback/RefreshButton'

const ACTION_FILTERS = [
  'create',
  'update',
  'delete',
  'login',
  'replay',
  'retry',
  'bulk_retry',
] as const

const TARGET_TYPE_FILTERS = [
  'endpoint',
  'mapping',
  'rule',
  'template',
  'channel',
  'event',
  'delivery',
] as const

export default function AuditPage() {
  const { t } = useTranslation()
  const { page, size, setPage, setSize } = usePagination()
  const [action, setAction] = useState('')
  const [targetType, setTargetType] = useState('')
  const [dateRange, setDateRange] = useState<DateRangeValue>({ from: null, to: null })

  // Times are sent only when both ends are set (parity with the old UI)
  const { from: rangeFrom, to: rangeTo } = dateRange
  const hasDateRange = rangeFrom !== null && rangeTo !== null
  const startTime = hasDateRange ? format(rangeFrom, 'yyyy-MM-dd') : ''
  const endTime = hasDateRange ? format(rangeTo, 'yyyy-MM-dd') : ''

  const listQuery = useQuery({
    queryKey: ['audit', page, size, action, targetType, startTime, endTime],
    queryFn: () =>
      listAuditLogs({
        page,
        size,
        ...(action ? { action } : {}),
        ...(targetType ? { target_type: targetType } : {}),
        ...(hasDateRange ? { start_time: startTime, end_time: endTime } : {}),
      }),
    placeholderData: keepPreviousData,
  })

  function clearFilters() {
    setAction('')
    setTargetType('')
    setDateRange({ from: null, to: null })
    setPage(1)
  }

  const rows = listQuery.data?.data ?? []
  const total = listQuery.data?.total ?? 0
  const hasFilters = action !== '' || targetType !== '' || dateRange.from !== null

  return (
    <div className="space-y-4">
      <PageHeader
        title={t('nav.auditLogs')}
        actions={
          <RefreshButton
            label={t('dashboard.refresh')}
            onClick={() => void listQuery.refetch()}
            loading={listQuery.isFetching}
          />
        }
      />

      <div className="flex flex-wrap items-center gap-3">
        <Select
          value={action || 'all'}
          onValueChange={(v) => {
            setAction(v === 'all' ? '' : v)
            setPage(1)
          }}
        >
          <SelectTrigger size="sm" className="w-40" aria-label={t('audit.action')}>
            <SelectValue placeholder={t('audit.action')} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">{t('common.all')}</SelectItem>
            {ACTION_FILTERS.map((a) => (
              <SelectItem key={a} value={a}>
                {t(`audit.${a}`, { defaultValue: a })}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select
          value={targetType || 'all'}
          onValueChange={(v) => {
            setTargetType(v === 'all' ? '' : v)
            setPage(1)
          }}
        >
          <SelectTrigger size="sm" className="w-40" aria-label={t('audit.targetType')}>
            <SelectValue placeholder={t('audit.targetType')} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">{t('common.all')}</SelectItem>
            {TARGET_TYPE_FILTERS.map((tt) => (
              <SelectItem key={tt} value={tt}>
                {t(`enums.auditTargetType.${tt}`, { defaultValue: tt })}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <DateRangePicker
          value={dateRange}
          onChange={(range) => {
            setDateRange(range)
            setPage(1)
          }}
        />

        {hasFilters ? (
          <Button variant="ghost" size="sm" onClick={clearFilters}>
            <X />
            {t('audit.clearFilters')}
          </Button>
        ) : null}
      </div>

      <div className="rounded-lg border">
        {listQuery.isPending ? (
          <TableSkeleton cols={6} />
        ) : listQuery.isError ? (
          <ErrorState
            error={listQuery.error}
            onRetry={() => void listQuery.refetch()}
            entity={t('nav.auditLogs')}
          />
        ) : rows.length === 0 ? (
          <EmptyState
            title={t('audit.emptyTitle')}
            description={t('audit.emptyDescription')}
            icon={<ScrollText className="size-5" />}
          />
        ) : (
          <Table className="min-w-[600px]">
            <TableHeader>
              <TableRow className="group">
                <TableHead className="sticky left-0 z-20 min-w-36 bg-background group-hover:bg-[color-mix(in_oklab,var(--color-muted)_50%,var(--color-background))]">{t('common.time')}</TableHead>
                <TableHead className="w-28">{t('audit.operator')}</TableHead>
                <TableHead className="w-28">{t('audit.action')}</TableHead>
                <TableHead className="w-28">{t('audit.targetType')}</TableHead>
                <TableHead className="min-w-40">{t('audit.targetId')}</TableHead>
                <TableHead className="min-w-48">{t('audit.summary')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((row) => (
                <TableRow key={row.id} className="group">
                  <TableCell className="sticky left-0 z-10 bg-background text-muted-foreground group-hover:bg-[color-mix(in_oklab,var(--color-muted)_50%,var(--color-background))]">
                    {formatTime(row.created_at)}
                  </TableCell>
                  <TableCell className="text-muted-foreground">{row.operator}</TableCell>
                  <TableCell>
                    <StatusBadge kind="audit" status={row.action} />
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {t(`enums.auditTargetType.${row.target_type}`, { defaultValue: row.target_type })}
                  </TableCell>
                  <TableCell>
                    <TruncatedId id={row.target_id} />
                  </TableCell>
                  <TableCell>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <span className="block max-w-72 truncate text-sm text-muted-foreground">
                          {row.summary ?? '-'}
                        </span>
                      </TooltipTrigger>
                      <TooltipContent className="max-w-sm">
                        <p className="whitespace-pre-wrap break-words text-xs">
                          {row.summary ?? '-'}
                        </p>
                      </TooltipContent>
                    </Tooltip>
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
          sizeOptions={[10, 20, 50, 100]}
        />
      ) : null}
    </div>
  )
}
