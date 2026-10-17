import { useI18n } from '@/i18n'
import { cn } from '@/lib/utils'

type StatusTone = 'info' | 'success' | 'warning' | 'danger' | 'neutral'

const toneClasses: Record<StatusTone, string> = {
  info: 'bg-sky-500/10 text-sky-700 dark:text-sky-400',
  success: 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-400',
  warning: 'bg-amber-500/10 text-amber-700 dark:text-amber-400',
  danger: 'bg-red-500/10 text-red-700 dark:text-red-400',
  neutral: 'bg-muted text-muted-foreground',
}

const dotClasses: Record<StatusTone, string> = {
  info: 'bg-sky-500',
  success: 'bg-emerald-500',
  warning: 'bg-amber-500',
  danger: 'bg-red-500',
  neutral: 'bg-muted-foreground/50',
}

const eventStatusTone: Record<string, StatusTone> = {
  received: 'info',
  processing: 'warning',
  completed: 'success',
  deduped: 'neutral',
  failed: 'danger',
}

const deliveryStatusTone: Record<string, StatusTone> = {
  pending: 'info',
  processing: 'warning',
  delivered: 'success',
  retry_wait: 'warning',
  failed: 'danger',
  dead_letter: 'danger',
}

const auditActionTone: Record<string, StatusTone> = {
  create: 'success',
  update: 'info',
  delete: 'danger',
  login: 'neutral',
  replay: 'warning',
  retry: 'warning',
  bulk_retry: 'warning',
}

export type StatusKind = 'event' | 'delivery' | 'audit'

export function StatusBadge({
  kind,
  status,
  className,
}: {
  kind: StatusKind
  status: string
  className?: string
}) {
  const { t } = useI18n()

  const toneMap =
    kind === 'event' ? eventStatusTone : kind === 'delivery' ? deliveryStatusTone : auditActionTone
  const tone = toneMap[status] ?? 'info'

  const labelKey =
    kind === 'event'
      ? `enums.eventStatus.${status}`
      : kind === 'delivery'
        ? `enums.deliveryStatus.${status}`
        : `enums.auditAction.${status}`
  const label = t(labelKey, status)

  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 rounded-md px-2 py-0.5 text-xs font-medium whitespace-nowrap',
        toneClasses[tone],
        className,
      )}
    >
      <span className={cn('size-1.5 rounded-full', dotClasses[tone])} aria-hidden />
      {label}
    </span>
  )
}
