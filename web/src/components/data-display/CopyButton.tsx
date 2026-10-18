import { useState } from 'react'
import { Check, Copy } from 'lucide-react'
import { useI18n } from '@/i18n'
import { copyToClipboard } from '@/lib/formatters'
import { cn } from '@/lib/utils'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'

interface CopyButtonProps {
  text: string
  className?: string
  /** Show a small icon-only button; default false shows icon+handled by parent */
  iconOnly?: boolean
  /** Hide the icon until the container marked `group/reveal` is hovered; default true */
  revealOnHover?: boolean
}

export function CopyButton({ text, className, iconOnly = true, revealOnHover = true }: CopyButtonProps) {
  const { t } = useI18n()
  const [copied, setCopied] = useState(false)

  async function handleCopy(e: React.MouseEvent) {
    e.stopPropagation()
    // blur immediately: keeping focus would re-open the tooltip when the
    // surrounding dialog/sheet is reopened (Radix restores focus on open)
    ;(e.currentTarget as HTMLButtonElement).blur()
    try {
      await copyToClipboard(text)
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    } catch {
      // clipboard unavailable
    }
  }

  if (iconOnly) {
    return (
      <Tooltip>
        <TooltipTrigger asChild>
          <button
            type="button"
            aria-label={t('common.copy')}
            onClick={handleCopy}
            className={cn(
              'inline-flex size-6 shrink-0 items-center justify-center rounded-sm text-muted-foreground transition-[color,opacity] hover:text-foreground',
              revealOnHover && 'opacity-0 group-hover/reveal:opacity-100 focus-visible:opacity-100',
              className,
            )}
          >
            {copied ? <Check className="size-3.5 text-emerald-500" /> : <Copy className="size-3.5" />}
          </button>
        </TooltipTrigger>
        <TooltipContent>
          <p>{copied ? t('common.copied') : t('common.copy')}</p>
        </TooltipContent>
      </Tooltip>
    )
  }

  return null
}
