import { format } from 'date-fns'
import { CalendarIcon } from 'lucide-react'
import { enUS, zhCN } from 'react-day-picker/locale'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Calendar } from '@/components/ui/calendar'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'

/** 起止均为 null 表示未选;from 有值、to 为 null 表示选了起点还没选终点。 */
export interface DateRangeValue {
  from: Date | null
  to: Date | null
}

interface DateRangePickerProps {
  value: DateRangeValue
  onChange: (range: DateRangeValue) => void
}

/** 日期范围选择:Popover + 日历,风格与筛选区的 Select 一致(outline 触发器)。 */
export function DateRangePicker({ value, onChange }: DateRangePickerProps) {
  const { t, i18n } = useTranslation()
  const locale = i18n.language.startsWith('zh') ? zhCN : enUS

  const label = value.from
    ? `${format(value.from, 'yyyy-MM-dd')} ~ ${value.to ? format(value.to, 'yyyy-MM-dd') : '…'}`
    : null

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          size="sm"
          className="h-8 w-64 justify-start font-normal"
          aria-label={t('audit.dateRange')}
        >
          <CalendarIcon className="size-4 shrink-0 text-muted-foreground" />
          <span className={label ? '' : 'text-muted-foreground'}>
            {label ?? t('audit.dateRangePlaceholder')}
          </span>
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-auto p-0" align="start">
        <Calendar
          mode="range"
          numberOfMonths={1}
          locale={locale}
          selected={{ from: value.from ?? undefined, to: value.to ?? undefined }}
          onSelect={(range) =>
            onChange({ from: range?.from ?? null, to: range?.to ?? null })
          }
        />
      </PopoverContent>
    </Popover>
  )
}
