import { ChevronsUpDown, X } from 'lucide-react'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Checkbox } from '@/components/ui/checkbox'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import type { LookupItem } from '@/hooks/use-lookup'

interface MultiSelectProps {
  options: LookupItem[] | undefined
  value: string[]
  onChange: (value: string[]) => void
  /** Shown in the trigger when nothing is selected; also the popover empty state. */
  placeholder: string
  /** Accessible label for the badge remove button. */
  removeLabel?: string
  className?: string
}

/**
 * Multi-select built from Popover + checkbox list (shadcn Select is single-only).
 * Selected items render as removable badges inside the trigger area.
 * Selected ids missing from the options list are kept (shown as raw id).
 */
export function MultiSelect({
  options,
  value,
  onChange,
  placeholder,
  removeLabel,
  className,
}: MultiSelectProps) {
  const nameOf = (id: string) => options?.find((o) => o.id === id)?.name ?? id

  function toggle(id: string) {
    onChange(value.includes(id) ? value.filter((v) => v !== id) : [...value, id])
  }

  return (
    <Popover>
      <PopoverTrigger asChild>
        <div
          role="combobox"
          tabIndex={0}
          className={cn(
            'flex min-h-9 w-full cursor-pointer flex-wrap items-center gap-1 rounded-md border border-input bg-transparent px-3 py-1.5 text-sm shadow-xs outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 dark:bg-input/30',
            className,
          )}
        >
          {value.length === 0 ? (
            <span className="text-muted-foreground">{placeholder}</span>
          ) : (
            value.map((id) => (
              <Badge key={id} variant="secondary" className="gap-0.5 pr-1">
                <span className="max-w-40 truncate">{nameOf(id)}</span>
                <button
                  type="button"
                  className="rounded-full p-0.5 transition-colors hover:bg-foreground/15"
                  aria-label={removeLabel ?? nameOf(id)}
                  onClick={(e) => {
                    // Keep the popover open when removing a badge from the trigger
                    e.preventDefault()
                    e.stopPropagation()
                    onChange(value.filter((v) => v !== id))
                  }}
                >
                  <X className="size-3" />
                </button>
              </Badge>
            ))
          )}
          <ChevronsUpDown className="ml-auto size-4 shrink-0 text-muted-foreground" />
        </div>
      </PopoverTrigger>
      <PopoverContent className="w-(--radix-popover-trigger-width) p-1" align="start">
        <div className="max-h-60 overflow-y-auto">
          {(options ?? []).length === 0 ? (
            <p className="px-2 py-4 text-center text-sm text-muted-foreground">{placeholder}</p>
          ) : (
            (options ?? []).map((opt) => (
              <label
                key={opt.id}
                className="flex cursor-pointer items-center gap-2 rounded-sm px-2 py-1.5 text-sm outline-hidden hover:bg-accent hover:text-accent-foreground"
              >
                <Checkbox
                  checked={value.includes(opt.id)}
                  onCheckedChange={() => toggle(opt.id)}
                  aria-label={opt.name}
                />
                <span className="truncate">{opt.name}</span>
              </label>
            ))
          )}
        </div>
      </PopoverContent>
    </Popover>
  )
}
