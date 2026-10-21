import { ChevronLeft, ChevronRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { cn } from '@/lib/utils'

interface PaginationProps {
  total: number
  page: number
  size: number
  onPageChange: (page: number) => void
  onSizeChange: (size: number) => void
  sizeOptions?: number[]
  className?: string
}

export function Pagination({
  total,
  page,
  size,
  onPageChange,
  onSizeChange,
  sizeOptions = [10, 20, 50],
  className,
}: PaginationProps) {
  const { t } = useTranslation()
  const totalPages = Math.max(1, Math.ceil(total / size))
  const page2 = Math.min(page, totalPages)
  const start = total === 0 ? 0 : (page2 - 1) * size + 1
  const end = Math.min(total, page2 * size)

  return (
    <div className={cn('flex flex-wrap items-center justify-between gap-3 py-2', className)}>
      <div className="text-sm text-muted-foreground">
        {t('pagination.range', { start, end, total, count: total })}
      </div>
      <div className="flex items-center gap-3">
        <Select
          value={String(size)}
          onValueChange={(v) => {
            onSizeChange(Number(v))
            onPageChange(1)
          }}
        >
          <SelectTrigger size="sm" className="w-24" aria-label={t('pagination.size')}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {sizeOptions.map((s) => (
              <SelectItem key={s} value={String(s)}>
                {s} / {t('pagination.pageUnit')}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <div className="flex items-center gap-1">
          <Button
            variant="outline"
            size="icon-sm"
            disabled={page2 <= 1}
            onClick={() => onPageChange(page2 - 1)}
            aria-label={t('pagination.prev')}
          >
            <ChevronLeft />
          </Button>
          <span className="min-w-16 text-center text-sm text-muted-foreground tabular-nums">
            {page2} / {totalPages}
          </span>
          <Button
            variant="outline"
            size="icon-sm"
            disabled={page2 >= totalPages}
            onClick={() => onPageChange(page2 + 1)}
            aria-label={t('pagination.next')}
          >
            <ChevronRight />
          </Button>
        </div>
      </div>
    </div>
  )
}
