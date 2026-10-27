"use client"

import * as React from "react"
import { cn } from "@/lib/utils"

function Table({ className, ...props }: React.ComponentProps<"table">) {
  const wrapRef = React.useRef<HTMLDivElement>(null)
  // Frozen-pane shadows: while columns are clipped past a pinned edge column,
  // a soft gradient fades in at that column's inner boundary (the pinned
  // column appears to float above the content sliding underneath). Positions
  // are measured at runtime from the pinned columns' widths, so pages only
  // mark cells sticky; nothing else is coupled here.
  const [shadows, setShadows] = React.useState({
    left: false,
    right: false,
    leftEdge: 0,
    rightEdge: 0,
  })

  const updateShadows = React.useCallback(() => {
    const wrap = wrapRef.current
    const scroller = wrap?.firstElementChild as HTMLElement | null
    const table = scroller?.firstElementChild as HTMLTableElement | null
    if (!wrap || !scroller || !table) return
    const row = table.tBodies[0]?.rows[0] ?? table.tHead?.rows[0]
    if (!row || row.cells.length === 0) {
      setShadows((s) => (s.left || s.right ? { ...s, left: false, right: false } : s))
      return
    }
    const scrollerRect = scroller.getBoundingClientRect()
    const first = row.cells[0]
    const last = row.cells[row.cells.length - 1]
    const firstRect = first.getBoundingClientRect()
    const lastRect = last.getBoundingClientRect()
    const clipped = scroller.scrollWidth > scroller.clientWidth + 1
    const next = {
      // left shadow: first column is pinned AND content is scrolled under it
      left: clipped && scroller.scrollLeft > 0 && firstRect.left >= scrollerRect.left - 1,
      // right shadow: last column is pinned AND content remains hidden to the right
      right:
        clipped &&
        scroller.scrollLeft + scroller.clientWidth < scroller.scrollWidth - 1 &&
        lastRect.right <= scrollerRect.right + 1,
      leftEdge: firstRect.width,
      rightEdge: lastRect.width,
    }
    setShadows((s) =>
      s.left === next.left && s.right === next.right && s.leftEdge === next.leftEdge && s.rightEdge === next.rightEdge
        ? s
        : next
    )
  }, [])

  React.useEffect(() => {
    const wrap = wrapRef.current
    const scroller = wrap?.firstElementChild as HTMLElement | null
    if (!wrap || !scroller) return
    updateShadows()
    scroller.addEventListener("scroll", updateShadows, { passive: true })
    const ro = new ResizeObserver(updateShadows)
    ro.observe(wrap)
    const table = scroller.firstElementChild as HTMLElement | null
    ro.observe(table ?? scroller)
    return () => {
      scroller.removeEventListener("scroll", updateShadows)
      ro.disconnect()
    }
  }, [updateShadows])

  return (
    // overflow-hidden + rounded-lg clips the opaque sticky-cell backgrounds at
    // the corners so the page wrapper's rounded border stays visible.
    <div ref={wrapRef} data-slot="table-container" className="relative w-full overflow-hidden rounded-lg">
      <div className="w-full overflow-x-auto overscroll-x-contain">
        <table
          data-slot="table"
          className={cn(
            "w-full caption-bottom text-sm",
            // Round the four corner cells so opaque sticky backgrounds don't
            // paint over the page wrapper's rounded border (own-radius bg
            // clipping — independent of any overflow-clip semantics).
            "[&_thead_th:first-child]:rounded-tl-lg",
            "[&_thead_th:last-child]:rounded-tr-lg",
            "[&_tbody_tr:last-child_td:first-child]:rounded-bl-lg",
            "[&_tbody_tr:last-child_td:last-child]:rounded-br-lg",
            className
          )}
          {...props}
        />
      </div>
      {/* Boundary shadows sit above the sticky cells (z-30, no pointer events)
          and only render while columns are actually clipped. */}
      {shadows.left ? (
        <div
          aria-hidden
          style={{ left: shadows.leftEdge }}
          className="pointer-events-none absolute inset-y-0 z-30 w-4 bg-gradient-to-r from-foreground/15 to-transparent"
        />
      ) : null}
      {shadows.right ? (
        <div
          aria-hidden
          style={{ right: shadows.rightEdge }}
          className="pointer-events-none absolute inset-y-0 z-30 w-4 bg-gradient-to-l from-foreground/15 to-transparent"
        />
      ) : null}
    </div>
  )
}

function TableHeader({ className, ...props }: React.ComponentProps<"thead">) {
  return (
    <thead
      data-slot="table-header"
      className={cn("[&_tr]:border-b", className)}
      {...props}
    />
  )
}

function TableBody({ className, ...props }: React.ComponentProps<"tbody">) {
  return (
    <tbody
      data-slot="table-body"
      className={cn("[&_tr:last-child]:border-0", className)}
      {...props}
    />
  )
}

function TableFooter({ className, ...props }: React.ComponentProps<"tfoot">) {
  return (
    <tfoot
      data-slot="table-footer"
      className={cn(
        "border-t bg-muted/50 font-medium [&>tr]:last:border-b-0",
        className
      )}
      {...props}
    />
  )
}

function TableRow({ className, ...props }: React.ComponentProps<"tr">) {
  return (
    <tr
      data-slot="table-row"
      className={cn(
        "border-b transition-colors hover:bg-muted/50 has-aria-expanded:bg-muted/50 data-[state=selected]:bg-muted",
        className
      )}
      {...props}
    />
  )
}

function TableHead({ className, ...props }: React.ComponentProps<"th">) {
  return (
    <th
      data-slot="table-head"
      className={cn(
        "h-10 px-3 text-left align-middle font-medium whitespace-nowrap text-foreground [&:has([role=checkbox])]:pr-0 [&>[role=checkbox]]:translate-y-[2px]",
        className
      )}
      {...props}
    />
  )
}

function TableCell({ className, ...props }: React.ComponentProps<"td">) {
  return (
    <td
      data-slot="table-cell"
      className={cn(
        "px-3 py-2 align-middle whitespace-nowrap [&:has([role=checkbox])]:pr-0 [&>[role=checkbox]]:translate-y-[2px]",
        className
      )}
      {...props}
    />
  )
}

function TableCaption({
  className,
  ...props
}: React.ComponentProps<"caption">) {
  return (
    <caption
      data-slot="table-caption"
      className={cn("mt-4 text-sm text-muted-foreground", className)}
      {...props}
    />
  )
}

export {
  Table,
  TableHeader,
  TableBody,
  TableFooter,
  TableHead,
  TableRow,
  TableCell,
  TableCaption,
}
