import { toast } from 'sonner'
import { useI18n } from '@/i18n'
import { copyToClipboard } from '@/lib/formatters'
import { cn } from '@/lib/utils'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { CopyButton } from './CopyButton'

/**
 * Full ID with CSS truncation, tooltip, and click-to-copy.
 */
export function TruncatedId({ id, className }: { id: string; className?: string }) {
  const { t } = useI18n()

  async function handleCopy(e: React.MouseEvent) {
    e.stopPropagation()
    ;(e.currentTarget as HTMLButtonElement).blur()
    try {
      await copyToClipboard(id)
      toast.success(t('common.copied'))
    } catch {
      toast.error(t('common.copyFailed'))
    }
  }

  return (
    <span className={cn('group/reveal inline-flex min-w-0 items-center gap-0.5', className)}>
      <Tooltip>
        <TooltipTrigger asChild>
          <button
            type="button"
            onClick={handleCopy}
            className="min-w-0 truncate text-left text-sm text-muted-foreground transition-colors hover:text-foreground"
          >
            {id}
          </button>
        </TooltipTrigger>
        <TooltipContent className="max-w-md">
          <p className="break-all">{id}</p>
        </TooltipContent>
      </Tooltip>
      <CopyButton text={id} />
    </span>
  )
}
