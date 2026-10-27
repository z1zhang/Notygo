import { Badge } from '@/components/ui/badge'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'

interface OverflowBadgesProps {
  items: string[]
  nameOf: (id: string) => string
  /** 最多显示的徽章数,超出折叠成 +N,默认 3 */
  max?: number
}

/** 徽章列表:最多显示 max 个,其余折叠为一个 +N 徽章(hover 展开全部名称)。 */
export function OverflowBadges({ items, nameOf, max = 3 }: OverflowBadgesProps) {
  const visible = items.slice(0, max)
  const hidden = items.slice(max)
  const hiddenNames = hidden.map(nameOf)

  return (
    <div className="flex items-center gap-1">
      {visible.map((id) => (
        <Badge key={id} variant="secondary" className="max-w-40">
          <span className="truncate">{nameOf(id)}</span>
        </Badge>
      ))}
      {hidden.length > 0 && (
        <Tooltip>
          <TooltipTrigger asChild>
            <Badge variant="secondary">+{hidden.length}</Badge>
          </TooltipTrigger>
          <TooltipContent className="max-w-sm">
            <p className="whitespace-pre-wrap break-words text-xs">
              {hiddenNames.join('\n')}
            </p>
          </TooltipContent>
        </Tooltip>
      )}
    </div>
  )
}
