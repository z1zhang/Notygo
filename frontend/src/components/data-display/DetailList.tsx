import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'

interface DetailListProps {
  items: { label: string; value: ReactNode; span?: 1 | 2 }[]
  className?: string
}

/**
 * Two-column description grid used in detail drawers.
 */
export function DetailList({ items, className }: DetailListProps) {
  return (
    <dl
      className={cn(
        'grid grid-cols-1 gap-x-6 gap-y-0 rounded-lg border sm:grid-cols-2',
        className,
      )}
    >
      {items.map(
        (item, i) =>
          item.value !== null &&
          item.value !== undefined && (
            <div
              key={i}
              className={cn(
                'flex flex-col gap-0.5 border-b px-4 py-2.5 last:border-b-0 sm:[&:nth-last-child(2)]:border-b-0',
                item.span === 2 && 'sm:col-span-2',
              )}
            >
              <dt className="text-xs text-muted-foreground">{item.label}</dt>
              <dd className="text-sm break-words">{item.value}</dd>
            </div>
          ),
      )}
    </dl>
  )
}

export function DetailSection({
  title,
  children,
  className,
}: {
  title: string
  children: ReactNode
  className?: string
}) {
  return (
    <section className={cn('space-y-3', className)}>
      <h3 className="text-sm font-semibold">{title}</h3>
      {children}
    </section>
  )
}
