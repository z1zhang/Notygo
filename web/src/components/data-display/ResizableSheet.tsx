import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'

interface ResizableSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description?: string
  children: ReactNode
  footer?: ReactNode
  initialWidth?: number
  minWidth?: number
  maxWidth?: number
}

/**
 * Right-side detail drawer whose width can be dragged via a handle
 * on the left border (parity with the old useResizableDrawer).
 */
export function ResizableSheet({
  open,
  onOpenChange,
  title,
  description,
  children,
  footer,
  initialWidth = 800,
  minWidth = 400,
  maxWidth = 1200,
}: ResizableSheetProps) {
  const [width, setWidth] = useState(initialWidth)
  const dragState = useRef<{ startX: number; startWidth: number } | null>(null)

  const onPointerMove = useCallback((e: PointerEvent) => {
    const drag = dragState.current
    if (!drag) return
    const delta = drag.startX - e.clientX
    const next = Math.min(Math.max(drag.startWidth + delta, minWidth), maxWidth)
    // clamp to viewport
    setWidth(Math.min(next, window.innerWidth - 16))
  }, [minWidth, maxWidth])

  const onPointerUp = useCallback(() => {
    dragState.current = null
    document.body.style.userSelect = ''
    document.body.style.cursor = ''
  }, [])

  function startDrag(e: React.PointerEvent) {
    e.preventDefault()
    dragState.current = { startX: e.clientX, startWidth: width }
    document.body.style.userSelect = 'none'
    document.body.style.cursor = 'col-resize'
    window.addEventListener('pointermove', onPointerMove)
    window.addEventListener('pointerup', onPointerUp)
  }

  // Clean up listeners when the sheet closes
  useEffect(() => {
    if (!open) {
      window.removeEventListener('pointermove', onPointerMove)
      window.removeEventListener('pointerup', onPointerUp)
      dragState.current = null
      document.body.style.userSelect = ''
      document.body.style.cursor = ''
    }
  }, [open, onPointerMove, onPointerUp])

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        side="right"
        className="flex w-full flex-col gap-0 p-0 sm:max-w-none"
        style={{ width: `min(${width}px, 100vw - 16px)` }}
        // disable Radix auto-focus on open: otherwise the last-focused element
        // (e.g. a copy button) regains focus and its tooltip pops open
        onOpenAutoFocus={(e) => e.preventDefault()}
      >
        {/* Drag handle on the left edge */}
        <div
          onPointerDown={startDrag}
          role="separator"
          aria-orientation="vertical"
          className="absolute inset-y-0 left-0 z-10 w-1.5 -translate-x-1/2 cursor-col-resize touch-none hover:bg-primary/30"
        />
        <SheetHeader className="border-b pr-10">
          <SheetTitle className="text-base">{title}</SheetTitle>
          {description ? <SheetDescription>{description}</SheetDescription> : null}
        </SheetHeader>
        <div className="min-h-0 flex-1 overflow-y-auto p-4">{children}</div>
        {footer ? <div className="border-t p-4">{footer}</div> : null}
      </SheetContent>
    </Sheet>
  )
}
