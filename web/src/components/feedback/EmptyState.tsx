import type { ReactNode } from 'react'
import { Inbox } from 'lucide-react'

interface EmptyStateProps {
  title: string
  description?: string
  icon?: ReactNode
  action?: ReactNode
  /** Draw a rounded border so the empty state aligns with bordered tables in detail sheets */
  bordered?: boolean
  className?: string
}

export function EmptyState({ title, description, icon, action, bordered, className }: EmptyStateProps) {
  return (
    <div
      className={`flex flex-col items-center justify-center gap-2 px-6 ${
        bordered ? 'rounded-lg border py-8' : 'py-12'
      } ${className ?? ''}`}
    >
      <div className="flex size-10 items-center justify-center rounded-full bg-muted text-muted-foreground">
        {icon ?? <Inbox className="size-5" />}
      </div>
      <p className="text-sm font-medium">{title}</p>
      {description ? (
        <p className="max-w-sm text-center text-sm text-muted-foreground">{description}</p>
      ) : null}
      {action}
    </div>
  )
}
