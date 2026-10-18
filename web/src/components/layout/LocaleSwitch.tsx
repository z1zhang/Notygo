import { Languages } from 'lucide-react'
import { useI18n, type Locale } from '@/i18n'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'

export function LocaleSwitch() {
  const { locale, setLocale } = useI18n()

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="sm" className="gap-1.5">
          <Languages className="size-4" />
          <span className="text-sm">{locale === 'zh' ? '中文' : 'EN'}</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem
          onClick={() => setLocale('zh' as Locale)}
          data-active={locale === 'zh'}
        >
          中文
        </DropdownMenuItem>
        <DropdownMenuItem
          onClick={() => setLocale('en' as Locale)}
          data-active={locale === 'en'}
        >
          English
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
