import { useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { Webhook } from "lucide-react"
import { useTranslation } from "react-i18next"
import { listEvents } from "@/lib/api"
import { lookupName, useEndpointOptions } from "@/hooks/use-lookup"
import { usePagination } from "@/hooks/use-pagination"
import { PageHeader } from "@/components/layout/PageHeader"
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
import { EmptyState } from "@/components/feedback/EmptyState"
import { RefreshButton } from "@/components/feedback/RefreshButton"
import { ErrorState } from "@/components/feedback/ErrorState"
import { formatTime } from "@/lib/formatters"
import { EventDetailSheet } from "./EventDetailSheet"

const EVENT_STATUSES = ["received", "processing", "completed", "deduped", "failed"] as const

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

export default function EventsPage() {
  const { t } = useTranslation()
  const { page, size, setPage, setSize } = usePagination()
  const [statusFilter, setStatusFilter] = useState<string>(ALL)
  const [endpointFilter, setEndpointFilter] = useState<string>(ALL)
  const [selectedId, setSelectedId] = useState<string | null>(null)

  const listQuery = useQuery({
    queryKey: ["events", page, size, statusFilter, endpointFilter],
    queryFn: () =>
      listEvents({
        page,
        size,
        status: statusFilter === ALL ? undefined : statusFilter,
        endpoint_id: endpointFilter === ALL ? undefined : endpointFilter,
      }),
  })
  const endpointOptions = useEndpointOptions()

  const rows = listQuery.data?.data ?? []
  const total = listQuery.data?.total ?? 0

  return (
    <div className="space-y-4">
      <PageHeader
        title={t("nav.eventLogs")}
        actions={
          <RefreshButton
            label={t("dashboard.refresh")}
            onClick={() => void listQuery.refetch()}
            loading={listQuery.isFetching}
          />
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
            {EVENT_STATUSES.map((status) => (
              <SelectItem key={status} value={status}>
                {t(`enums.eventStatus.${status}`)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select
          value={endpointFilter}
          onValueChange={(v) => {
            setEndpointFilter(v)
            setPage(1)
          }}
        >
          <SelectTrigger size="sm" className="w-40" aria-label={t("events.endpoint")}>
            <SelectValue placeholder={t("events.endpoint")} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL}>{t("common.all")}</SelectItem>
            {(endpointOptions.data ?? []).map((endpoint) => (
              <SelectItem key={endpoint.id} value={endpoint.id}>
                {endpoint.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <div className="rounded-lg border">
        {listQuery.isPending ? (
          <TableSkeleton cols={5} />
        ) : listQuery.isError ? (
          <ErrorState
            error={listQuery.error}
            onRetry={() => void listQuery.refetch()}
            entity={t("nav.eventLogs")}
          />
        ) : rows.length === 0 ? (
          <EmptyState
            title={t("events.emptyTitle")}
            description={t("events.emptyDescription")}
            icon={<Webhook className="size-5" />}
          />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("common.id")}</TableHead>
                <TableHead>{t("events.endpoint")}</TableHead>
                <TableHead>{t("common.status")}</TableHead>
                <TableHead>{t("events.receivedAt")}</TableHead>
                <TableHead>{t("common.error")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((row) => {
                const err = errorCell(row.error_code, row.error_summary)
                return (
                  <TableRow
                    key={row.id}
                    className="cursor-pointer"
                    onClick={() => setSelectedId(row.id)}
                  >
                    <TableCell>
                      <TruncatedId id={row.id} />
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {lookupName(endpointOptions.data, row.endpoint_id)}
                    </TableCell>
                    <TableCell>
                      <StatusBadge kind="event" status={row.status} />
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {formatTime(row.received_at)}
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

      <EventDetailSheet eventId={selectedId} onClose={() => setSelectedId(null)} />
    </div>
  )
}
