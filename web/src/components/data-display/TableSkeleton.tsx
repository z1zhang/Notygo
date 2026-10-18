import { Skeleton } from '@/components/ui/skeleton'

export function TableSkeleton({ rows = 6, cols = 5 }: { rows?: number; cols?: number }) {
  return (
    <div className="space-y-2 p-1">
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="flex items-center gap-4 px-3">
          {Array.from({ length: cols }).map((_, j) => (
            <Skeleton
              key={j}
              className="h-8 flex-1"
              style={{ opacity: 1 - i * 0.12 }}
            />
          ))}
        </div>
      ))}
    </div>
  )
}
