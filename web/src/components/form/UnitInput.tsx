import type { ComponentProps } from 'react'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'

/**
 * Number input with a fixed unit suffix rendered inside the field
 * (e.g. 秒 / KB / 毫秒). Mirrors the label-above + suffix pattern used by
 * the rules dedupe window field.
 */
export function UnitInput({ unit, className, ...props }: ComponentProps<typeof Input> & { unit: string }) {
  return (
    <div className="relative">
      <Input className={cn('pr-12', className)} {...props} />
      <span className="pointer-events-none absolute inset-y-0 right-3 flex items-center text-sm text-muted-foreground">
        {unit}
      </span>
    </div>
  )
}
