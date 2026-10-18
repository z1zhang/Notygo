import { RefreshCw } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

interface RefreshButtonProps {
  label: string
  onClick: () => void
  /** Spin the icon while a refetch is in flight. */
  loading?: boolean
  variant?: 'outline' | 'ghost'
}

export function RefreshButton({ label, onClick, loading, variant = 'outline' }: RefreshButtonProps) {
  return (
    <Button variant={variant} size="sm" onClick={onClick} aria-label={label}>
      <RefreshCw className={cn(loading && 'animate-spin')} />
      {label}
    </Button>
  )
}
