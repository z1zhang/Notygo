import { useMemo } from 'react'
import { CopyButton } from './CopyButton'
import { prettyJson } from '@/lib/formatters'
import { cn } from '@/lib/utils'

interface JsonViewerProps {
  /** Raw JSON text or any value; invalid JSON degrades to plain text */
  value: string | null | undefined
  className?: string
  /** Max height with internal scroll; default 240px */
  maxHeight?: number
}

/**
 * Read-only JSON viewer: monospace, pretty-printed when valid, copyable,
 * internal scroll for long content, plain-text fallback.
 */
export function JsonViewer({ value, className, maxHeight = 240 }: JsonViewerProps) {
  const display = useMemo(() => {
    if (value === null || value === undefined || value === '') return null
    if (typeof value === 'string') return prettyJson(value)
    try {
      return JSON.stringify(value, null, 2)
    } catch {
      return String(value)
    }
  }, [value])

  if (display === null) {
    return <span className="text-sm text-muted-foreground">-</span>
  }

  return (
    <div
      className={cn(
        'relative rounded-md border bg-muted/40 dark:bg-muted/20',
        'group/json overflow-hidden',
        className,
      )}
    >
      <div className="absolute top-1.5 right-1.5 opacity-0 transition-opacity group-hover/json:opacity-100">
        <CopyButton text={display} revealOnHover={false} />
      </div>
      <pre
        className="overflow-auto p-3 font-mono text-xs leading-relaxed whitespace-pre-wrap break-all"
        style={{ maxHeight }}
      >
        {display}
      </pre>
    </div>
  )
}
