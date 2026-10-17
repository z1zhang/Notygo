import { AlertTriangle, RefreshCw } from 'lucide-react'
import { useI18n } from '@/i18n'
import { getErrorMessage } from '@/lib/api-client'
import { Button } from '@/components/ui/button'

interface ErrorStateProps {
  error: unknown
  onRetry?: () => void
  entity?: string
  className?: string
}

export function ErrorState({ error, onRetry, entity, className }: ErrorStateProps) {
  const { t } = useI18n()
  const fallback = entity
    ? t('common.failedToLoad', { entity })
    : t('common.failedToLoad', { entity: '' })
  const message = getErrorMessage(error, fallback)

  return (
    <div className={`flex flex-col items-center justify-center gap-3 px-6 py-12 ${className ?? ''}`}>
      <div className="flex size-10 items-center justify-center rounded-full bg-red-500/10 text-red-600 dark:text-red-400">
        <AlertTriangle className="size-5" />
      </div>
      <p className="text-sm font-medium">{message}</p>
      {onRetry ? (
        <Button variant="outline" size="sm" onClick={onRetry}>
          <RefreshCw />
          {t('common.retry')}
        </Button>
      ) : null}
    </div>
  )
}
