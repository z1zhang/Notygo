import type { ReactNode } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTheme } from '@/app/theme'
import { Cpu, Monitor, Moon, Server, Sun } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { getHealthz, getSystemInfo } from '@/lib/api'
import { PageHeader } from '@/components/layout/PageHeader'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { ErrorState } from '@/components/feedback/ErrorState'
import { RefreshButton } from '@/components/feedback/RefreshButton'
import { cn } from '@/lib/utils'

type ThemeChoice = 'light' | 'dark' | 'system'

export default function SettingsPage() {
  const { t } = useTranslation()
  const { theme, setTheme } = useTheme()

  const systemQuery = useQuery({
    queryKey: ['system', 'info'],
    queryFn: getSystemInfo,
  })
  const healthQuery = useQuery({
    queryKey: ['system', 'healthz'],
    queryFn: getHealthz,
  })

  const healthy = healthQuery.data?.status === 'ok'
  const systemFailed = systemQuery.isError && healthQuery.isError

  function recheck() {
    void systemQuery.refetch()
    void healthQuery.refetch()
  }

  const themeChoices: { value: ThemeChoice; label: string; icon: typeof Sun }[] = [
    { value: 'light', label: t('settings.themeLight'), icon: Sun },
    { value: 'dark', label: t('settings.themeDark'), icon: Moon },
    { value: 'system', label: t('settings.themeSystem'), icon: Monitor },
  ]
  const currentTheme: ThemeChoice =
    theme === 'light' || theme === 'dark' ? theme : 'system'

  const info = systemQuery.data

  /** 3600 → "1 小时"; 90 → "1.5 分钟"; 30 → "30 秒" (unit via i18n) */
  const fmtDur = (seconds: number): string => {
    if (seconds >= 3600) return `${seconds % 3600 === 0 ? seconds / 3600 : (seconds / 3600).toFixed(1)} ${t('settings.unitHours')}`
    if (seconds >= 60) return `${seconds % 60 === 0 ? seconds / 60 : (seconds / 60).toFixed(1)} ${t('settings.unitMinutes')}`
    return `${seconds} ${t('settings.unitSeconds')}`
  }

  return (
    <div className="space-y-4">
      <PageHeader
        title={t('nav.settings')}
        actions={
          <RefreshButton
            label={t('settings.recheck')}
            onClick={recheck}
            loading={systemQuery.isFetching || healthQuery.isFetching}
          />
        }
      />

      {/* System info — real data from GET /api/v1/system and /healthz */}
      <section className="rounded-lg border">
        <SectionHeader icon={<Server className="size-4" />} title={t('settings.systemInfo')} />
        <div className="p-4">
          {systemFailed ? (
            <ErrorState
              error={systemQuery.error ?? healthQuery.error}
              onRetry={recheck}
              entity={t('settings.systemInfo')}
            />
          ) : (
            <dl className="grid grid-cols-1 gap-x-6 sm:grid-cols-3">
              <div className="flex flex-col gap-1 border-b py-2.5 sm:border-b-0 sm:py-1">
                <dt className="text-xs text-muted-foreground">{t('settings.serviceName')}</dt>
                <dd className="text-sm font-medium break-words">
                  {systemQuery.isPending ? <Skeleton className="h-5 w-28" /> : (info?.name ?? '-')}
                </dd>
              </div>
              <div className="flex flex-col gap-1 border-b py-2.5 sm:border-b-0 sm:py-1">
                <dt className="text-xs text-muted-foreground">{t('settings.version')}</dt>
                <dd className="text-sm font-medium break-words tabular-nums">
                  {systemQuery.isPending ? <Skeleton className="h-5 w-16" /> : (info?.version ?? '-')}
                </dd>
              </div>
              <div className="flex flex-col gap-1 py-2.5 sm:py-1">
                <dt className="text-xs text-muted-foreground">{t('settings.status')}</dt>
                <dd>
                  {healthQuery.isPending ? (
                    <Skeleton className="h-5 w-16" />
                  ) : (
                    <Badge
                      className={
                        healthy
                          ? 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-400'
                          : 'bg-red-500/10 text-red-700 dark:text-red-400'
                      }
                    >
                      {healthy ? t('settings.healthy') : t('settings.unhealthy')}
                    </Badge>
                  )}
                </dd>
              </div>
            </dl>
          )}
        </div>
      </section>

      {/* Runtime config — real values from GET /api/v1/system */}
      <section className="rounded-lg border">
        <SectionHeader icon={<Cpu className="size-4" />} title={t('settings.runtimeConfig')} />
        <div className="p-4">
          {systemQuery.isPending ? (
            <div className="space-y-3">
              <Skeleton className="h-6 w-full" />
              <Skeleton className="h-6 w-full" />
              <Skeleton className="h-6 w-2/3" />
            </div>
          ) : systemQuery.isError ? (
            <ErrorState
              error={systemQuery.error}
              onRetry={recheck}
              entity={t('settings.runtimeConfig')}
            />
          ) : info ? (
            <dl className="divide-y">
              <ConfigRow label={t('settings.eventConcurrency')} value={String(info.queue.event_concurrency)} />
              <ConfigRow label={t('settings.deliveryConcurrency')} value={String(info.queue.delivery_concurrency)} />
              <ConfigRow
                label={t('settings.jobRetention')}
                value={fmtDur(info.queue.job_retention_seconds)}
              />
              <ConfigRow
                label={t('settings.retryBackoff')}
                value={t('settings.retryBackoffValue', {
                  base: fmtDur(info.queue.retry_base_delay_seconds),
                  max: fmtDur(info.queue.retry_max_delay_seconds),
                  jitter: info.queue.retry_jitter_percent,
                })}
              />
              <ConfigRow label={t('settings.maxPayload')} value={formatBytes(info.ingest.max_payload_bytes)} />
              <ConfigRow
                label={t('settings.rateLimit')}
                value={t('settings.rateLimitValue', {
                  rate: info.ingest.rate_per_second,
                  burst: info.ingest.burst,
                })}
              />
              <ConfigRow
                label={t('settings.loginLockout')}
                value={t('settings.loginLockoutValue', {
                  n: info.login.max_failures,
                  m: fmtDur(info.login.lockout_seconds),
                })}
              />
              <ConfigRow label={t('settings.logLevel')} value={info.log_level} />
            </dl>
          ) : null}
          <p className="mt-3 text-xs text-muted-foreground">{t('settings.runtimeNote')}</p>
        </div>
      </section>

      {/* Theme */}
      <section className="rounded-lg border">
        <SectionHeader icon={<Monitor className="size-4" />} title={t('settings.theme')} />
        <div className="p-4">
          <div className="flex flex-wrap gap-2">
            {themeChoices.map(({ value, label, icon: Icon }) => (
              <Button
                key={value}
                variant="outline"
                size="sm"
                aria-pressed={currentTheme === value}
                className={cn(currentTheme === value && 'border-primary bg-primary/10 text-primary hover:bg-primary/15')}
                onClick={() => setTheme(value)}
              >
                <Icon />
                {label}
              </Button>
            ))}
          </div>
        </div>
      </section>
    </div>
  )
}

function ConfigRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-3 py-2.5">
      <dt className="text-sm">{label}</dt>
      <dd className="text-sm text-right break-words tabular-nums">{value}</dd>
    </div>
  )
}

/** 1048576 → "1 MB"; 204800 → "200 KB" */
function formatBytes(bytes: number): string {
  if (bytes >= 1024 * 1024) {
    const mb = bytes / (1024 * 1024)
    return `${Number.isInteger(mb) ? mb : mb.toFixed(1)} MB`
  }
  const kb = Math.round(bytes / 1024)
  return `${kb} KB`
}

function SectionHeader({ icon, title }: { icon: ReactNode; title: string }) {
  return (
    <div className="flex items-center gap-2 border-b px-4 py-3">
      <span className="text-muted-foreground">{icon}</span>
      <h2 className="text-sm font-semibold">{title}</h2>
    </div>
  )
}
