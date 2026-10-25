import { z } from 'zod'
import { useTranslation } from 'react-i18next'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { asString, type AnyForm, type ChannelModule } from './types'

export interface WebhookFormFields {
  webhook_url: string
  msg_type: string
}

const WEBHOOK_TYPES = ['dingtalk', 'feishu', 'wecom'] as const

const MSG_TYPE_OPTIONS: Record<string, string[]> = {
  dingtalk: ['markdown', 'actionCard'],
  feishu: ['interactive', 'text', 'post'],
  wecom: ['markdown', 'text', 'image', 'news'],
}

const DEFAULT_MSG_TYPE: Record<string, string> = {
  dingtalk: 'markdown',
  feishu: 'interactive',
  wecom: 'markdown',
}

function WebhookFields({ form }: { form: AnyForm }) {
  const { t } = useTranslation()
  const type: string = form.watch('type')
  const options = MSG_TYPE_OPTIONS[type] ?? []
  const errors = form.formState.errors

  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-[minmax(0,1fr)_176px]">
      <div className="space-y-2">
        <Label htmlFor="destination-webhook-url">{t('destinations.webhookUrl')}</Label>
        <Input
          id="destination-webhook-url"
          placeholder="https://..."
          {...form.register('webhook_url')}
        />
        {errors.webhook_url ? (
          <p className="text-xs text-destructive">{t('destinations.webhookUrlRequired')}</p>
        ) : null}
      </div>
      <div className="space-y-2">
        <Label>{t('destinations.msgType')}</Label>
        <Select
          value={form.watch('msg_type')}
          onValueChange={(v) => form.setValue('msg_type', v)}
        >
          <SelectTrigger className="w-full">
            <SelectValue placeholder={t('destinations.msgType')} />
          </SelectTrigger>
          <SelectContent>
            {options.map((option) => (
              <SelectItem key={option} value={option}>
                {t(`enums.msgType.${option}`, { defaultValue: option })}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
    </div>
  )
}

export const webhookModule: ChannelModule = {
  types: WEBHOOK_TYPES,
  formSchema: {
    webhook_url: z.string(),
    msg_type: z.string(),
  },
  defaults: { webhook_url: '', msg_type: '' },
  defaultMsgType: DEFAULT_MSG_TYPE,
  Fields: WebhookFields,
  toConfig: (data) => {
    const config: Record<string, unknown> = { webhook_url: data.webhook_url }
    if (data.msg_type !== '') config.msg_type = data.msg_type
    return config
  },
  fromConfig: (config, type) => {
    // Stored msg_type must be one of the type's options, else fall back to the default.
    const options = MSG_TYPE_OPTIONS[type] ?? []
    const raw = asString(config.msg_type)
    return {
      webhook_url: asString(config.webhook_url),
      msg_type: options.includes(raw) ? raw : (DEFAULT_MSG_TYPE[type] ?? ''),
    }
  },
  validate: (data, ctx) => {
    if (WEBHOOK_TYPES.includes(data.type) && !(data.webhook_url ?? '').trim()) {
      ctx.addIssue({ code: 'custom', path: ['webhook_url'], message: 'webhookUrlRequired' })
    }
  },
}
