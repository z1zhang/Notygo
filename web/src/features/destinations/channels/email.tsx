import { z } from 'zod'
import { useTranslation } from 'react-i18next'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import { asNumber, asString, type AnyForm, type ChannelModule } from './types'

export interface EmailFormFields {
  smtp_host: string
  smtp_port: number
  use_tls: boolean
  from: string
  to: string
  cc: string
}

/** to/cc may be an array of emails or a string — normalize to a textarea string. */
function asAddressList(value: unknown): string {
  if (Array.isArray(value)) {
    return value.filter((v): v is string => typeof v === 'string').join('\n')
  }
  return asString(value)
}

function splitAddressList(value: string): string[] {
  return value
    .split(/[\n,]/)
    .map((v) => v.trim())
    .filter((v) => v !== '')
}

function EmailFields({ form }: { form: AnyForm }) {
  const { t } = useTranslation()

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-[1fr_130px_130px]">
        <div className="space-y-2">
          <Label htmlFor="destination-smtp-host">{t('destinations.smtpHost')}</Label>
          <Input
            id="destination-smtp-host"
            placeholder="smtp.example.com"
            {...form.register('smtp_host')}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="destination-smtp-port">{t('destinations.smtpPort')}</Label>
          <Input
            id="destination-smtp-port"
            type="number"
            min={1}
            max={65535}
            {...form.register('smtp_port', { valueAsNumber: true })}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="destination-use-tls">{t('destinations.useTls')}</Label>
          <div className="flex h-9 items-center">
            <Switch
              id="destination-use-tls"
              checked={form.watch('use_tls')}
              onCheckedChange={(checked) => form.setValue('use_tls', checked)}
            />
          </div>
        </div>
      </div>
      <div className="space-y-2">
        <Label htmlFor="destination-from">{t('destinations.fromAddress')}</Label>
        <Input
          id="destination-from"
          placeholder="noreply@example.com"
          {...form.register('from')}
        />
      </div>
      <div className="space-y-2">
        <Label htmlFor="destination-to">{t('destinations.toAddress')}</Label>
        <Textarea
          id="destination-to"
          rows={3}
          placeholder={t('destinations.toPlaceholder')}
          {...form.register('to')}
        />
      </div>
      <div className="space-y-2">
        <Label htmlFor="destination-cc">{t('destinations.ccAddress')}</Label>
        <Textarea
          id="destination-cc"
          rows={2}
          placeholder={t('destinations.ccPlaceholder')}
          {...form.register('cc')}
        />
      </div>
    </div>
  )
}

export const emailModule: ChannelModule = {
  types: ['email'],
  formSchema: {
    smtp_host: z.string(),
    smtp_port: z.number().min(1).max(65535),
    use_tls: z.boolean(),
    from: z.string(),
    to: z.string(),
    cc: z.string(),
  },
  defaults: { smtp_host: '', smtp_port: 465, use_tls: true, from: '', to: '', cc: '' },
  Fields: EmailFields,
  toConfig: (data) => {
    const config: Record<string, unknown> = {
      smtp_host: data.smtp_host,
      smtp_port: data.smtp_port,
      use_tls: data.use_tls,
      from: data.from,
      to: splitAddressList(data.to),
    }
    const cc = splitAddressList(data.cc)
    if (cc.length > 0) config.cc = cc
    return config
  },
  fromConfig: (config) => ({
    smtp_host: asString(config.smtp_host),
    smtp_port: asNumber(config.smtp_port, 465),
    use_tls: typeof config.use_tls === 'boolean' ? config.use_tls : true,
    from: asString(config.from),
    to: asAddressList(config.to),
    cc: asAddressList(config.cc),
  }),
}
