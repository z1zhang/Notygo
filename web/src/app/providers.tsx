import type { ReactNode } from 'react'
import { ThemeProvider } from 'next-themes'
import { QueryClientProvider } from '@tanstack/react-query'
import { queryClient } from './query-client'
import { I18nProvider } from '@/i18n'
import { Toaster } from '@/components/ui/sonner'
import { TooltipProvider } from '@/components/ui/tooltip'

export function Providers({ children }: { children: ReactNode }) {
  return (
    <ThemeProvider
      attribute="class"
      defaultTheme="system"
      storageKey="zpush-theme"
      enableSystem
      disableTransitionOnChange
    >
      <I18nProvider>
        <QueryClientProvider client={queryClient}>
          <TooltipProvider delayDuration={300}>{children}</TooltipProvider>
          <Toaster richColors position="top-center" />
        </QueryClientProvider>
      </I18nProvider>
    </ThemeProvider>
  )
}
