import { useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { RotateCcw, Send } from "lucide-react"
import { useTranslation } from "react-i18next"
import { bulkRetry, listDeliveries } from "@/lib/api"
import { getErrorMessage } from "@/lib/api-client"
import { formatTime } from "@/lib/formatters"
import { lookupName, useChannelOptions } from "@/hooks/use-lookup"
import { usePagination } from "@/hooks/use-pagination"
import { PageHeader } from "@/components/layout/PageHeader"
import { Button } from "@/components/ui/button"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { Pagination } from "@/components/data-display/Pagination"
import { StatusBadge } from "@/components/data-display/StatusBadge"
import { TableSkeleton } from "@/components/data-display/TableSkeleton"
import { TruncatedId } from "@/components/data-display/TruncatedId"
import { ConfirmDialog } from "@/components/feedback/ConfirmDialog"
import { RefreshButton } from "@/components/feedback/RefreshButton"
import { EmptyState } from "@/components/feedback/EmptyState"
import { ErrorState } from "@/components/feedback/ErrorState"
import { DeliveryDetailSheet } from "./DeliveryDetailSheet"

const DELIVERY_STATUSES = [
  "pending",
  "processing",
  "delivered",
  "retry_wait",
  "failed",
  "dead_letter",
] as const

/** Filter sentinel meaning "no filter" (Radix Select has no clearable state). */
const ALL = "all"

function errorCell(
  code: string | null | undefined,
  summary: string | null | undefined,
): { short: string; full: string | null } {
  const parts = [code, summary].filter(Boolean)
  if (parts.length === 0) return { short: "-", full: null }
  const full = parts.join(": ")
  return { short: summary || code || "-", full }
}

export default function DeliveriesPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { page, size, setPage, setSize } = usePagination()
  const [statusFilter, setStatusFilter] = useState<string>(ALL)
  const [destFilter, setDestFilter] = useState<string>(ALL)
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [bulkConfirmOpen, setBulkConfirmOpen] = useState(false)

  const listQuery = useQuery({
    queryKey: ["deliveries", page, size, statusFilter, destFilter],
    queryFn: () =>
      listDeliveries({
        page,
        size,
        status: statusFilter === ALL ? undefined : statusFilter,
        channel_id: destFilter === ALL ? undefined : destFilter,
      }),
  })
  const channelOptions = useChannelOptions()

  const bulkRetryMutation = useMutation({
    mutationFn: () =>
      bulkRetry({
        status: statusFilter === ALL ? "failed" : statusFilter,
        channel_id: destFilter === ALL ? undefined : destFilter,
      }),
    onSuccess: (res) => {
      toast.success(t("deliveries.bulkRetryDone", { count: res.reset_count }))
      setBulkConfirmOpen(false)
      void queryClient.invalidateQueries({ queryKey: ["deliveries"] })
    },
    onError: (e) => {
      toast.error(getErrorMessage(e, t("deliveries.bulkRetryFailed")))
    },
  })

  const rows = listQuery.data?.data ?? []
  const total = listQuery.data?.total ?? 0

  /** Bulk retry only applies to failed/retry_wait — disable when filtered elsewhere */
  const BULK_RETRYABLE = statusFilter === ALL || statusFilter === "failed" || statusFilter === "retry_wait"

  return (
    <div className="space-y-4">
      <PageHeader
        title={t("nav.deliveryLogs")}
        actions={
          <>
            <RefreshButton
              label={t("dashboard.refresh")}
              onClick={() => void listQuery.refetch()}
              loading={listQuery.isFetching}
            />
            <Tooltip>
              <TooltipTrigger asChild>
                <span>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setBulkConfirmOpen(true)}
                    disabled={bulkRetryMutation.isPending || !BULK_RETRYABLE}
                  >
                    <RotateCcw />
                    {t("deliveries.bulkRetry")}
                  </Button>
                </span>
              </TooltipTrigger>
              {!BULK_RETRYABLE ? (
                <TooltipContent>{t("deliveries.bulkRetryUnavailable")}</TooltipContent>
              ) : null}
            </Tooltip>
          </>
        }
      />

      <div className="flex flex-wrap items-center gap-3">
        <Select
          value={statusFilter}
          onValueChange={(v) => {
            setStatusFilter(v)
            setPage(1)
          }}
        >
          <SelectTrigger size="sm" className="w-40" aria-label={t("common.status")}>
            <SelectValue placeholder={t("common.status")} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL}>{t("common.all")}</SelectItem>
            {DELIVERY_STATUSES.map((status) => (
              <SelectItem key={status} value={status}>
                {t(`enums.deliveryStatus.${status}`)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select
          value={destFilter}
          onValueChange={(v) => {
            setDestFilter(v)
            setPage(1)
          }}
        >
          <SelectTrigger size="sm" className="w-40" aria-label={t("deliveries.channel")}>
            <SelectValue placeholder={t("deliveries.channel")} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL}>{t("common.all")}</SelectItem>
            {(channelOptions.data ?? []).map((dest) => (
              <SelectItem key={dest.id} value={dest.id}>
                {dest.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <div className="rounded-lg border">
        {listQuery.isPending ? (
          <TableSkeleton cols={7} />
        ) : listQuery.isError ? (
          <ErrorState
            error={listQuery.error}
            onRetry={() => void listQuery.refetch()}
            entity={t("nav.deliveryLogs")}
          />
        ) : rows.length === 0 ? (
          <EmptyState
            title={t("deliveries.emptyTitle")}
            description={t("deliveries.emptyDescription")}
            icon={<Send className="size-5" />}
          />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("common.id")}</TableHead>
                <TableHead>{t("deliveries.eventId")}</TableHead>
                <TableHead>{t("deliveries.channel")}</TableHead>
                <TableHead>{t("common.status")}</TableHead>
                <TableHead className="w-24">{t("deliveries.attemptCount")}</TableHead>
                <TableHead>{t("deliveries.nextAttempt")}</TableHead>
                <TableHead>{t("common.error")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((row) => {
                const err = errorCell(row.last_error_code, row.last_error_summary)
                return (
                  <TableRow
                    key={row.id}
                    className="cursor-pointer"
                    onClick={() => setSelectedId(row.id)}
                  >
                    <TableCell>
                      <TruncatedId id={row.id} />
                    </TableCell>
                    <TableCell>
                      <TruncatedId id={row.event_id} />
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {lookupName(channelOptions.data, row.channel_id)}
                    </TableCell>
                    <TableCell>
                      <StatusBadge kind="delivery" status={row.status} />
                    </TableCell>
                    <TableCell className="text-muted-foreground tabular-nums">{row.attempt_count}</TableCell>
                    <TableCell className="text-muted-foreground">
                      {formatTime(row.next_attempt_at)}
                    </TableCell>
                    <TableCell>
                      {err.full ? (
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <span className="block max-w-[240px] truncate text-muted-foreground">
                              {err.short}
                            </span>
                          </TooltipTrigger>
                          <TooltipContent className="max-w-md">
                            <p className="break-words whitespace-pre-wrap">{err.full}</p>
                          </TooltipContent>
                        </Tooltip>
                      ) : (
                        <span className="text-muted-foreground">-</span>
                      )}
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

      <DeliveryDetailSheet deliveryId={selectedId} onClose={() => setSelectedId(null)} />

      <ConfirmDialog
        open={bulkConfirmOpen}
        onOpenChange={(open) => {
          if (!bulkRetryMutation.isPending) setBulkConfirmOpen(open)
        }}
        title={t("deliveries.bulkRetryConfirmTitle")}
        description={t("deliveries.bulkRetryConfirmMsg")}
        confirmLabel={t("deliveries.bulkRetry")}
        destructive={false}
        onConfirm={() => bulkRetryMutation.mutate()}
        pending={bulkRetryMutation.isPending}
      />
    </div>
  )
}
