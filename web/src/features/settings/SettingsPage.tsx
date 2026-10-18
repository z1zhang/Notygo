import type { ReactNode } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTheme } from 'next-themes'
import { CalendarClock, Cpu, Monitor, Moon, Server, Sun } from 'lucide-react'
import { useI18n } from '@/i18n'
import { getHealthz, getRootInfo } from '@/lib/api'
import { PageHeader } from '@/components/layout/PageHeader'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { ErrorState } from '@/components/feedback/ErrorState'
import { RefreshButton } from '@/components/feedback/RefreshButton'
import { cn } from '@/lib/utils'

type ThemeChoice = 'light' | 'dark' | 'system'

const retentionSettings = [
  { key: 'eventRetentionDays' },
  { key: 'deliveryAttemptRetentionDays' },
] as const

const workerSettings = [
  { key: 'processorConcurrency' },
  { key: 'deliveryWorkerConcurrency' },
  { key: 'leaseTimeout' },
] as const

export default function SettingsPage() {
  const { t } = useI18n()
  const { theme, setTheme } = useTheme()

  const rootInfoQuery = useQuery({
    queryKey: ['system', 'root-info'],
    queryFn: getRootInfo,
  })
  const healthQuery = useQuery({
    queryKey: ['system', 'healthz'],
    queryFn: getHealthz,
  })

  const healthy = healthQuery.data?.status === 'ok'
  const systemFailed = rootInfoQuery.isError && healthQuery.isError

  function recheck() {
    void rootInfoQuery.refetch()
    void healthQuery.refetch()
  }

  const themeChoices: { value: ThemeChoice; label: string; icon: typeof Sun }[] = [
    { value: 'light', label: t('settings.themeLight'), icon: Sun },
    { value: 'dark', label: t('settings.themeDark'), icon: Moon },
    { value: 'system', label: t('settings.themeSystem'), icon: Monitor },
  ]
  const currentTheme: ThemeChoice =
    theme === 'light' || theme === 'dark' ? theme : 'system'

  return (
    <div className="space-y-4">
      <PageHeader
        title={t('nav.settings')}
        actions={
          <RefreshButton
            label={t('settings.recheck')}
            onClick={recheck}
            loading={rootInfoQuery.isFetching || healthQuery.isFetching}
          />
        }
      />

      {/* System info — real data from GET / and /healthz */}
      <section className="rounded-lg border">
        <SectionHeader icon={<Server className="size-4" />} title={t('settings.systemInfo')} />
        <div className="p-4">
          {systemFailed ? (
            <ErrorState
              error={rootInfoQuery.error ?? healthQuery.error}
              onRetry={recheck}
              entity={t('settings.systemInfo')}
            />
          ) : (
            <dl className="grid grid-cols-1 gap-x-6 sm:grid-cols-3">
              <div className="flex flex-col gap-1 border-b py-2.5 sm:border-b-0 sm:py-1">
                <dt className="text-xs text-muted-foreground">{t('settings.serviceName')}</dt>
                <dd className="text-sm font-medium break-words">
                  {rootInfoQuery.isPending ? <Skeleton className="h-5 w-28" /> : (rootInfoQuery.data?.name ?? '-')}
                </dd>
              </div>
              <div className="flex flex-col gap-1 border-b py-2.5 sm:border-b-0 sm:py-1">
                <dt className="text-xs text-muted-foreground">{t('settings.version')}</dt>
                <dd className="text-sm font-medium break-words tabular-nums">
                  {rootInfoQuery.isPending ? <Skeleton className="h-5 w-16" /> : (rootInfoQuery.data?.version ?? '-')}
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

      {/* Retention settings — env-var driven, not exposed via API */}
      <section className="rounded-lg border">
        <SectionHeader
          icon={<CalendarClock className="size-4" />}
          title={t('settings.retentionSettings')}
        />
        <div className="p-4">
          <dl className="divide-y">
            {retentionSettings.map(({ key }) => (
              <div key={key} className="flex items-center justify-between gap-3 py-2.5">
                <dt className="text-sm">{t(`settings.${key}`)}</dt>
                <dd className="flex items-center gap-2">
                  <span className="text-sm text-muted-foreground">-</span>
                  <Badge variant="secondary">{t('settings.env')}</Badge>
                </dd>
              </div>
            ))}
          </dl>
          <p className="mt-3 text-xs text-muted-foreground">{t('settings.retentionNote')}</p>
        </div>
      </section>

      {/* Worker config — env-var driven, not exposed via API */}
      <section className="rounded-lg border">
        <SectionHeader icon={<Cpu className="size-4" />} title={t('settings.workerConfig')} />
        <div className="p-4">
          <dl className="divide-y">
            {workerSettings.map(({ key }) => (
              <div key={key} className="flex items-center justify-between gap-3 py-2.5">
                <dt className="text-sm">{t(`settings.${key}`)}</dt>
                <dd className="flex items-center gap-2">
                  <span className="text-sm text-muted-foreground">-</span>
                  <Badge variant="secondary">{t('settings.env')}</Badge>
                </dd>
              </div>
            ))}
          </dl>
          <p className="mt-3 text-xs text-muted-foreground">{t('settings.workerNote')}</p>
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

function SectionHeader({ icon, title }: { icon: ReactNode; title: string }) {
  return (
    <div className="flex items-center gap-2 border-b px-4 py-3">
      <span className="text-muted-foreground">{icon}</span>
      <h2 className="text-sm font-semibold">{title}</h2>
    </div>
  )
}
