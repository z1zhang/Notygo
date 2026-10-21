import { useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { ChevronDown, ChevronRight, RotateCcw } from "lucide-react"
import { useTranslation } from "react-i18next"
import { getDelivery, retryDelivery } from "@/lib/api"
import { getErrorMessage } from "@/lib/api-client"
import { formatTime } from "@/lib/formatters"
import type { DeliveryAttemptResponse } from "@/lib/types"
import { lookupName, useDestinationOptions } from "@/hooks/use-lookup"
import { Button } from "@/components/ui/button"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { DetailList, DetailSection } from "@/components/data-display/DetailList"
import { JsonViewer } from "@/components/data-display/JsonViewer"
import { ResizableSheet } from "@/components/data-display/ResizableSheet"
import { StatusBadge } from "@/components/data-display/StatusBadge"
import { TableSkeleton } from "@/components/data-display/TableSkeleton"
import { TruncatedId } from "@/components/data-display/TruncatedId"
import { ConfirmDialog } from "@/components/feedback/ConfirmDialog"
import { EmptyState } from "@/components/feedback/EmptyState"
import { ErrorState } from "@/components/feedback/ErrorState"
import { cn } from "@/lib/utils"
import { RenderedBody } from "./RenderedBody"

interface DeliveryDetailSheetProps {
  deliveryId: string | null
  onClose: () => void
}

function joinError(code?: string | null, summary?: string | null): string | null {
  const parts = [code, summary].filter(Boolean)
  return parts.length > 0 ? parts.join(": ") : null
}

/** The backend only accepts retries for these statuses (not dead_letter). */
const RETRYABLE_STATUSES = ["failed", "retry_wait"]

function AttemptResultBadge({ attempt }: { attempt: DeliveryAttemptResponse }) {
  const { t } = useTranslation()
  if (attempt.error_code) {
    return (
      <span className="inline-flex items-center rounded-md bg-red-500/10 px-2 py-0.5 text-xs font-medium whitespace-nowrap text-red-700 dark:text-red-400">
        {t("enums.deliveryStatus.failed")}
      </span>
    )
  }
  return (
    <span className="inline-flex items-center rounded-md bg-emerald-500/10 px-2 py-0.5 text-xs font-medium whitespace-nowrap text-emerald-700 dark:text-emerald-400">
      {t("deliveries.ok")}
    </span>
  )
}

function AttemptRow({
  attempt,
  expanded,
  onToggle,
}: {
  attempt: DeliveryAttemptResponse
  expanded: boolean
  onToggle: () => void
}) {
  const { t } = useTranslation()
  const error = joinError(attempt.error_code, attempt.error_summary)

  return (
    <>
      <TableRow className="cursor-pointer" onClick={onToggle}>
        <TableCell>
          <span className="flex items-center gap-1">
            <Button variant="ghost" size="icon-xs" aria-label={t("events.detail")}>
              {expanded ? <ChevronDown /> : <ChevronRight />}
            </Button>
            <span className="tabular-nums">{attempt.attempt_no}</span>
          </span>
        </TableCell>
        <TableCell>
          <AttemptResultBadge attempt={attempt} />
        </TableCell>
        <TableCell className="tabular-nums">{attempt.response_status ?? "-"}</TableCell>
        <TableCell className="tabular-nums">
          {attempt.duration_ms != null ? `${attempt.duration_ms}ms` : "-"}
        </TableCell>
        <TableCell className="text-muted-foreground">{formatTime(attempt.started_at)}</TableCell>
      </TableRow>
      {expanded ? (
        <TableRow className="hover:bg-transparent">
          <TableCell colSpan={5} className="bg-muted/30">
            <div className="space-y-3 p-2">
              <div className="space-y-1.5">
                <p className="text-xs font-medium text-muted-foreground">
                  {t("deliveries.requestSummary")}
                </p>
                <JsonViewer value={attempt.request_summary} maxHeight={300} />
              </div>
              {error ? <p className="text-sm break-words text-destructive">{error}</p> : null}
              <div className="space-y-1.5">
                <p className="text-xs font-medium text-muted-foreground">
                  {t("deliveries.responseBody")}
                </p>
                <JsonViewer value={attempt.response_summary} maxHeight={300} />
              </div>
            </div>
          </TableCell>
        </TableRow>
      ) : null}
    </>
  )
}

export function DeliveryDetailSheet({ deliveryId, onClose }: DeliveryDetailSheetProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const destinationOptions = useDestinationOptions()
  const [confirmRetry, setConfirmRetry] = useState(false)
  const [expandedAttemptId, setExpandedAttemptId] = useState<string | null>(null)

  const detailQuery = useQuery({
    queryKey: ["deliveries", "detail", deliveryId],
    queryFn: () => getDelivery(deliveryId as string),
    enabled: deliveryId !== null,
  })

  const retryMutation = useMutation({
    mutationFn: (id: string) => retryDelivery(id),
    onSuccess: () => {
      toast.success(t("deliveries.retryInitiated"))
      setConfirmRetry(false)
      void queryClient.invalidateQueries({ queryKey: ["deliveries"] })
      void detailQuery.refetch()
    },
    onError: (e) => {
      toast.error(getErrorMessage(e, t("deliveries.retryFailed")))
    },
  })

  const delivery = detailQuery.data?.delivery
  const attempts = detailQuery.data?.delivery_attempts ?? []
  const retryable = delivery ? RETRYABLE_STATUSES.includes(delivery.status) : false

  return (
    <>
      <ResizableSheet
        open={deliveryId !== null}
        onOpenChange={(open) => {
          if (!open) onClose()
        }}
        title={t("deliveries.deliveryDetail")}
        description={deliveryId ?? undefined}
        footer={
          delivery ? (
            <div className="flex justify-end">
              <Button
                variant="outline"
                disabled={!retryable || retryMutation.isPending}
                className={cn(
                  "border-amber-500/60 text-amber-700 hover:bg-amber-500/10 hover:text-amber-700",
                  "dark:border-amber-500/40 dark:text-amber-400",
                  !retryable && "text-muted-foreground",
                )}
                onClick={() => setConfirmRetry(true)}
              >
                <RotateCcw />
                {t("deliveries.retry")}
              </Button>
            </div>
          ) : undefined
        }
      >
        {detailQuery.isPending ? (
          <TableSkeleton rows={8} cols={4} />
        ) : detailQuery.isError ? (
          <ErrorState
            error={detailQuery.error}
            onRetry={() => void detailQuery.refetch()}
            entity={t("nav.deliveryLogs")}
          />
        ) : delivery ? (
          <div className="space-y-6">
            <DetailList
              items={[
                {
                  label: t("common.id"),
                  value: <span className="break-all text-muted-foreground">{delivery.id}</span>,
                  span: 2,
                },
                {
                  label: t("common.status"),
                  value: <StatusBadge kind="delivery" status={delivery.status} />,
                },
                {
                  label: t("deliveries.destination"),
                  value: lookupName(destinationOptions.data, delivery.destination_id),
                },
                {
                  label: t("deliveries.eventId"),
                  value: <TruncatedId id={delivery.event_id} />,
                },
                {
                  label: t("deliveries.ruleId"),
                  value: delivery.rule_id ? <TruncatedId id={delivery.rule_id} /> : "-",
                },
                { label: t("deliveries.attemptCount"), value: delivery.attempt_count ?? 0 },
                {
                  label: t("deliveries.maxAttempts"),
                  value: delivery.max_attempts ? String(delivery.max_attempts) : "-",
                },
                { label: t("deliveries.nextAttempt"), value: formatTime(delivery.next_attempt_at) },
                { label: t("deliveries.createdAt"), value: formatTime(delivery.created_at) },
                ...(joinError(delivery.last_error_code, delivery.last_error_summary)
                  ? [
                      {
                        label: t("deliveries.lastError"),
                        value: (
                          <span className="text-destructive break-words">
                            {joinError(delivery.last_error_code, delivery.last_error_summary)}
                          </span>
                        ),
                        span: 2 as const,
                      },
                    ]
                  : []),
              ]}
            />

            <DetailSection title={t("deliveries.renderedTitle")}>
              <pre className="rounded-md border bg-muted/40 p-3 text-sm break-words whitespace-pre-wrap font-sans">
                {delivery.rendered_title || "-"}
              </pre>
            </DetailSection>

            <DetailSection title={t("deliveries.renderedBody")}>
              <RenderedBody body={delivery.rendered_body} format={delivery.rendered_format} />
            </DetailSection>

            <DetailSection title={t("deliveries.deliveryAttempts")}>
              {attempts.length > 0 ? (
                <div className="rounded-lg border">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead className="w-20">{t("deliveries.attemptNo")}</TableHead>
                        <TableHead>{t("deliveries.result")}</TableHead>
                        <TableHead>{t("deliveries.httpStatus")}</TableHead>
                        <TableHead>{t("deliveries.duration")}</TableHead>
                        <TableHead>{t("events.startedAt")}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {attempts.map((attempt) => (
                        <AttemptRow
                          key={attempt.id}
                          attempt={attempt}
                          expanded={expandedAttemptId === attempt.id}
                          onToggle={() =>
                            setExpandedAttemptId((cur) => (cur === attempt.id ? null : attempt.id))
                          }
                        />
                      ))}
                    </TableBody>
                  </Table>
                </div>
              ) : (
                <EmptyState title={t("deliveries.noAttempts")} />
              )}
            </DetailSection>
          </div>
        ) : null}
      </ResizableSheet>

      <ConfirmDialog
        open={confirmRetry}
        onOpenChange={(open) => {
          if (!retryMutation.isPending) setConfirmRetry(open)
        }}
        title={t("deliveries.retryConfirmTitle")}
        description={t("deliveries.retryConfirmMsg")}
        confirmLabel={t("deliveries.retry")}
        destructive={false}
        onConfirm={() => delivery && retryMutation.mutate(delivery.id)}
        pending={retryMutation.isPending}
      />
    </>
  )
}
