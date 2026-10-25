import { useLayoutEffect, useRef, useState } from 'react'
import { Badge } from '@/components/ui/badge'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'

interface OverflowBadgesProps {
  items: string[]
  nameOf: (id: string) => string
}

const GAP = 4 // gap-1 in px

/**
 * Single-line badge list that fits as many badges as the available width
 * allows and collapses the rest into one "+N" badge (hover shows the hidden
 * names). Widths come from an invisible absolute copy of the full row plus a
 * "+N" probe badge, so the result is stable — it only depends on the
 * container's width, never on what is currently rendered.
 */
export function OverflowBadges({ items, nameOf }: OverflowBadgesProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const fullRef = useRef<HTMLDivElement>(null)
  const [visibleCount, setVisibleCount] = useState(items.length)

  useLayoutEffect(() => {
    const box = containerRef.current
    const full = fullRef.current
    if (!box || !full) return

    const children = Array.from(full.children) as HTMLElement[]
    // Last child is the "+N" probe badge (widest count text = full item count).
    const probeW = children[children.length - 1]?.offsetWidth ?? 0
    const widths = children.slice(0, items.length).map((el) => el.offsetWidth)

    const fits = (k: number, withCount: boolean) => {
      if (k === 0) return !withCount || probeW <= box.clientWidth
      let w = -GAP
      for (let i = 0; i < k; i++) w += widths[i] + GAP
      if (withCount) w += GAP + probeW
      return w <= box.clientWidth
    }

    const check = () => {
      let k = items.length
      if (!fits(k, false)) {
        k = items.length - 1
        while (k > 0 && !fits(k, true)) k--
      }
      setVisibleCount(k)
    }
    check()
    const ro = new ResizeObserver(check)
    ro.observe(box)
    return () => ro.disconnect()
  }, [items])

  const hidden = items.length - visibleCount
  const hiddenNames = items.slice(visibleCount).map(nameOf)

  return (
    <div ref={containerRef} className="relative min-w-0 overflow-hidden">
      {/* Invisible full copy + "+N" probe — width measurement only */}
      <div
        ref={fullRef}
        aria-hidden
        className="pointer-events-none absolute invisible flex items-center gap-1"
      >
        {items.map((id) => (
          <Badge key={id} variant="secondary" className="max-w-40">
            <span className="truncate">{nameOf(id)}</span>
          </Badge>
        ))}
        <Badge variant="secondary">+{items.length}</Badge>
      </div>
      <div className="flex items-center gap-1">
        {items.slice(0, visibleCount).map((id) => (
          <Badge key={id} variant="secondary" className="max-w-40">
            <span className="truncate">{nameOf(id)}</span>
          </Badge>
        ))}
        {hidden > 0 && (
          <Tooltip>
            <TooltipTrigger asChild>
              <Badge variant="secondary">+{hidden}</Badge>
            </TooltipTrigger>
            <TooltipContent className="max-w-sm">
              <p className="whitespace-pre-wrap break-words text-xs">
                {hiddenNames.join('\n')}
              </p>
            </TooltipContent>
          </Tooltip>
        )}
      </div>
    </div>
  )
}
