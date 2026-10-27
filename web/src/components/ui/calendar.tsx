import * as React from 'react'
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { DayPicker } from 'react-day-picker'
import { buttonVariants } from '@/components/ui/button'
import { cn } from '@/lib/utils'

export type CalendarProps = React.ComponentProps<typeof DayPicker>

/**
 * 日历(react-day-picker v9),shadcn 风格的 Tailwind 样式覆盖。
 * 导航按钮绝对定位在月份标题两侧;选区/范围样式落在 day 单元格上。
 */
function Calendar({ className, classNames, showOutsideDays = true, ...props }: CalendarProps) {
  return (
    <DayPicker
      showOutsideDays={showOutsideDays}
      className={cn('relative p-3', className)}
      classNames={{
        months: 'flex flex-col sm:flex-row gap-2',
        month: 'flex flex-col gap-2',
        month_caption: 'flex h-9 w-full items-center justify-center text-sm font-medium',
        caption_label: 'text-sm font-medium',
        nav: 'flex items-center gap-1',
        button_previous: cn(
          buttonVariants({ variant: 'outline' }),
          'absolute left-1 top-2 size-8 bg-transparent p-0 opacity-50 hover:opacity-100'
        ),
        button_next: cn(
          buttonVariants({ variant: 'outline' }),
          'absolute right-1 top-2 size-8 bg-transparent p-0 opacity-50 hover:opacity-100'
        ),
        month_grid: 'w-full border-collapse',
        weekdays: 'flex',
        weekday: 'w-9 text-center text-[0.8rem] font-normal text-muted-foreground',
        week: 'mt-1 flex w-full',
        day: 'relative flex size-9 items-center justify-center p-0 text-sm focus-within:relative focus-within:z-20',
        day_button:
          'inline-flex size-9 items-center justify-center text-sm hover:bg-accent hover:text-accent-foreground',
        selected: 'rounded-md bg-accent text-accent-foreground',
        range_start: 'rounded-l-md bg-accent text-accent-foreground',
        range_end: 'rounded-r-md bg-accent text-accent-foreground',
        range_middle: 'rounded-none bg-accent/50 text-accent-foreground',
        today: 'rounded-md bg-accent text-accent-foreground',
        outside: 'text-muted-foreground',
        disabled: 'text-muted-foreground opacity-50',
        hidden: 'invisible',
        ...classNames,
      }}
      components={{
        Chevron: ({ orientation }) =>
          orientation === 'left' ? (
            <ChevronLeft className="size-4" />
          ) : (
            <ChevronRight className="size-4" />
          ),
      }}
      {...props}
    />
  )
}

export { Calendar }
