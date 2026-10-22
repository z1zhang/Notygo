import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { RotateCcw } from "lucide-react"
import { useTranslation } from "react-i18next"
import { getEvent, listDeliveries, replayEvent } from "@/lib/api"
import { getErrorMessage } from "@/lib/api-client"
import { formatTime, stringifyValue } from "@/lib/formatters"
import type { EventRunResponse, RuleSnapshot } from "@/lib/types"
import { lookupName, useDestinationOptions, useSourceOptions } from "@/hooks/use-lookup"
import { Button } from "@/components/ui/button"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { DetailList, DetailSection } from "@/components/data-display/DetailList"
import { JsonViewer } from "@/components/data-display/JsonViewer"
import { ResizableSheet } from "@/components/data-display/ResizableSheet"
import { StatusBadge } from "@/components/data-display/StatusBadge"
import { TableSkeleton } from "@/components/data-display/TableSkeleton"
import { CopyButton } from "@/components/data-display/CopyButton"
import { TruncatedId } from "@/components/data-display/TruncatedId"
import { EmptyState } from "@/components/feedback/EmptyState"
import { ErrorState } from "@/components/feedback/ErrorState"

interface EventDetailSheetProps {
  eventId: string | null
  onClose: () => void
}

function matchedRules(run: EventRunResponse): RuleSnapshot[] {
  // Older runs may carry JSON null for these fields before completion
  return (run.rule_set_snapshot ?? []).filter((r) => (run.matched_rule_ids ?? []).includes(r.id))
}

function joinError(code?: string | null, summary?: string | null): string | null {
  const parts = [code, summary].filter(Boolean)
  return parts.length > 0 ? parts.join(": ") : null
}

export function EventDetailSheet({ eventId, onClose }: EventDetailSheetProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const sourceOptions = useSourceOptions()
  const destinationOptions = useDestinationOptions()

  const detailQuery = useQuery({
    queryKey: ["events", "detail", eventId],
    queryFn: () => getEvent(eventId as string),
    enabled: eventId !== null,
  })

  const deliveriesQuery = useQuery({
    queryKey: ["deliveries", "related", eventId],
    queryFn: () => listDeliveries({ page: 1, size: 50, event_id: eventId as string }),
    enabled: eventId !== null,
  })

  const replayMutation = useMutation({
    mutationFn: (id: string) => replayEvent(id),
    onSuccess: () => {
      toast.success(t("events.replayInitiated"))
      void queryClient.invalidateQueries({ queryKey: ["events"] })
    },
    onError: (e) => {
      toast.error(getErrorMessage(e, t("events.replayFailed")))
    },
  })

  const data = detailQuery.data
  const event = data?.event
  const run = data?.latest_run ?? null

  return (
    <ResizableSheet
      open={eventId !== null}
      onOpenChange={(open) => {
        if (!open) onClose()
      }}
      title={t("events.eventDetail")}
      description={eventId ?? undefined}
      footer={
        event ? (
          <div className="flex justify-end">
            <Button
              variant="outline"
              disabled={replayMutation.isPending}
              className="border-amber-500/60 text-amber-700 hover:bg-amber-500/10 hover:text-amber-700 dark:border-amber-500/40 dark:text-amber-400"
              onClick={() => replayMutation.mutate(event.id)}
            >
              <RotateCcw />
              {t("events.replay")}
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
          entity={t("nav.eventLogs")}
        />
      ) : event ? (
        <div className="space-y-6">
          <DetailList
            items={[
              {
                label: t("common.id"),
                value: <span className="break-all text-muted-foreground">{event.id}</span>,
                span: 2,
              },
              { label: t("common.status"), value: <StatusBadge kind="event" status={event.status} /> },
              {
                label: t("events.source"),
                value: lookupName(sourceOptions.data, event.source_id),
              },
              { label: t("events.receivedAt"), value: formatTime(event.received_at) },
              { label: t("events.contentType"), value: event.content_type || "-" },
              ...(event.error_summary
                ? [
                    {
                      label: t("common.error"),
                      value: (
                        <span className="text-destructive break-words">
                          {joinError(event.error_code, event.error_summary)}
                        </span>
                      ),
                      span: 2 as const,
                    },
                  ]
                : []),
            ]}
          />

          <DetailSection title={t("events.rawPayload")}>
            <JsonViewer value={event.payload_raw} maxHeight={300} />
          </DetailSection>

          <DetailSection title={t("events.parsedFields")}>
            {run && Object.keys(run.parsed_fields).length > 0 ? (
              <div className="rounded-lg border">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className="w-40">{t("events.key")}</TableHead>
                      <TableHead>{t("events.value")}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {Object.entries(run.parsed_fields).map(([key, value]) => {
                      const text = stringifyValue(value)
                      return (
                        <TableRow key={key}>
                          <TableCell className="text-muted-foreground break-all">{key}</TableCell>
                          <TableCell>
                            <span className="group/reveal flex items-start gap-1">
                              <span className="min-w-0 flex-1 text-sm text-muted-foreground break-all">
                                {text}
                              </span>
                              <CopyButton text={text} className="size-5" />
                            </span>
                          </TableCell>
                        </TableRow>
                      )
                    })}
                  </TableBody>
                </Table>
              </div>
            ) : (
              <EmptyState title={t("events.noParsedFields")} />
            )}
          </DetailSection>

          {run ? (
            <DetailSection title={t("events.eventRun")}>
              <DetailList
                items={[
                  { label: t("events.runType"), value: run.run_type },
                  { label: t("common.status"), value: <StatusBadge kind="event" status={run.status} /> },
                  { label: t("events.startedAt"), value: formatTime(run.started_at) },
                  { label: t("events.finishedAt"), value: formatTime(run.finished_at) },
                ]}
              />
              {joinError(run.error_code, run.error_summary) ? (
                <p className="text-sm break-words text-destructive">
                  {joinError(run.error_code, run.error_summary)}
                </p>
              ) : null}
              {run.validation_result && Object.keys(run.validation_result).length > 0 ? (
                <div className="rounded-lg border">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead className="w-40">{t("events.field")}</TableHead>
                        <TableHead>{t("events.detail")}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {Object.entries(run.validation_result).map(([field, detail]) => (
                        <TableRow key={field}>
                          <TableCell className="text-muted-foreground break-all">{field}</TableCell>
                          <TableCell className="text-destructive break-words">
                            {stringifyValue(detail)}
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
              ) : null}
            </DetailSection>
          ) : null}

          {run ? (
            <DetailSection title={t("events.matchedRules")}>
              {matchedRules(run).length > 0 ? (
                <div className="rounded-lg border">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t("common.name")}</TableHead>
                        <TableHead className="w-24">{t("rules.priority")}</TableHead>
                        <TableHead>{t("events.destination")}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {matchedRules(run).map((rule) => (
                        <TableRow key={rule.id}>
                          <TableCell className="text-muted-foreground">{rule.name}</TableCell>
                          <TableCell className="tabular-nums text-muted-foreground">{rule.priority}</TableCell>
                          <TableCell className="text-muted-foreground">
                            {rule.destinations
                              .map((id) => lookupName(destinationOptions.data, id))
                              .join(", ")}
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
              ) : (
                <EmptyState title={t("events.noMatchedRules")} />
              )}
            </DetailSection>
          ) : null}

          <DetailSection title={t("events.relatedDeliveries")}>
            {deliveriesQuery.isPending ? (
              <TableSkeleton rows={3} cols={5} />
            ) : deliveriesQuery.isError ? (
              <ErrorState
                error={deliveriesQuery.error}
                onRetry={() => void deliveriesQuery.refetch()}
                entity={t("nav.deliveryLogs")}
              />
            ) : (deliveriesQuery.data?.data ?? []).length > 0 ? (
              <div className="rounded-lg border">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t("common.id")}</TableHead>
                      <TableHead>{t("events.destination")}</TableHead>
                      <TableHead>{t("common.status")}</TableHead>
                      <TableHead className="w-24">{t("events.attempts")}</TableHead>
                      <TableHead>{t("events.createdAt")}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {(deliveriesQuery.data?.data ?? []).map((delivery) => (
                      <TableRow key={delivery.id}>
                        <TableCell>
                          <TruncatedId id={delivery.id} />
                        </TableCell>
                        <TableCell className="text-muted-foreground">
                          {lookupName(destinationOptions.data, delivery.destination_id)}
                        </TableCell>
                        <TableCell>
                          <StatusBadge kind="delivery" status={delivery.status} />
                        </TableCell>
                        <TableCell className="tabular-nums text-muted-foreground">{delivery.attempt_count}</TableCell>
                        <TableCell className="text-muted-foreground">
                          {formatTime(delivery.created_at)}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            ) : (
              <EmptyState title={t("events.noDeliveries")} />
            )}
          </DetailSection>
        </div>
      ) : null}
    </ResizableSheet>
  )
}
