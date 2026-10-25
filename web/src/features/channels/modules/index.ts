import { z } from 'zod'
import type { Resolver } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { emailModule, type EmailFormFields } from './email'
import { httpModule, type HttpFormFields } from './http'
import { webhookModule, type WebhookFormFields } from './webhook'
import type { ChannelModule } from './types'

export type { BodyFieldNode } from './types'

export const CHANNEL_TYPES = ['dingtalk', 'feishu', 'wecom', 'email', 'http'] as const
export type ChannelType = (typeof CHANNEL_TYPES)[number]

// ── Registry: one entry per channel type (mirrors the backend sender
// registry in cmd/server/main.go). New channel = one module file + one line.
const MODULES: ChannelModule[] = [webhookModule, emailModule, httpModule]

const CHANNELS: Record<ChannelType, ChannelModule> = {
  dingtalk: webhookModule,
  feishu: webhookModule,
  wecom: webhookModule,
  email: emailModule,
  http: httpModule,
}

/** Module for a channel type; unknown types fall back to http. */
export function channelModule(type: string): ChannelModule {
  return CHANNELS[type as ChannelType] ?? httpModule
}

/** Unique modules (webhookModule serves three types). */
export function uniqueModules(): ChannelModule[] {
  return [...new Set(Object.values(CHANNELS))]
}

// ── Form schema: base fields + every channel module's fields ──────────────
export const channelSchema = z
  .object({
    name: z.string().min(1),
    type: z.enum(CHANNEL_TYPES),
    secret: z.string(),
    timeout_ms: z.number().min(1000),
    max_attempts: z.number().int().min(1).max(100),
    enabled: z.boolean(),
    ...Object.assign({}, ...MODULES.map((m) => m.formSchema)),
  })
  .superRefine((data, ctx) => {
    for (const m of uniqueModules()) m.validate?.(data, ctx)
  })

export type ChannelForm = {
  name: string
  type: ChannelType
  secret: string
  timeout_ms: number
  max_attempts: number
  enabled: boolean
} & WebhookFormFields &
  EmailFormFields &
  HttpFormFields

export const defaultForm: ChannelForm = {
  name: '',
  type: 'http',
  secret: '',
  timeout_ms: 10000,
  max_attempts: 5,
  enabled: true,
  ...Object.assign({}, ...MODULES.map((m) => m.defaults)),
} as ChannelForm

// zod v4's recursive z.lazy degrades inference to unknown[], so pin the resolver type
export const channelResolver = zodResolver(channelSchema) as unknown as Resolver<ChannelForm>
