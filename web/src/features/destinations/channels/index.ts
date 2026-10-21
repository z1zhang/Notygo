import { z } from 'zod'
import type { Resolver } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { emailModule, type EmailFormFields } from './email'
import { httpModule, type HttpFormFields } from './http'
import { webhookModule, type WebhookFormFields } from './webhook'
import type { ChannelModule } from './types'

export type { BodyFieldNode } from './types'

export const DEST_TYPES = ['dingtalk', 'feishu', 'wecom', 'email', 'http'] as const
export type DestType = (typeof DEST_TYPES)[number]

// ── Registry: one entry per destination type (mirrors the backend sender
// registry in cmd/server/main.go). New channel = one module file + one line.
const MODULES: ChannelModule[] = [webhookModule, emailModule, httpModule]

const CHANNELS: Record<DestType, ChannelModule> = {
  dingtalk: webhookModule,
  feishu: webhookModule,
  wecom: webhookModule,
  email: emailModule,
  http: httpModule,
}

/** Module for a destination type; unknown types fall back to http. */
export function channelModule(type: string): ChannelModule {
  return CHANNELS[type as DestType] ?? httpModule
}

/** Unique modules (webhookModule serves three types). */
export function uniqueModules(): ChannelModule[] {
  return [...new Set(Object.values(CHANNELS))]
}

// ── Form schema: base fields + every channel module's fields ──────────────
export const destinationSchema = z
  .object({
    name: z.string().min(1),
    type: z.enum(DEST_TYPES),
    secret: z.string(),
    timeout_ms: z.number().min(1000),
    enabled: z.boolean(),
    ...Object.assign({}, ...MODULES.map((m) => m.formSchema)),
  })
  .superRefine((data, ctx) => {
    for (const m of uniqueModules()) m.validate?.(data, ctx)
  })

export type DestinationForm = {
  name: string
  type: DestType
  secret: string
  timeout_ms: number
  enabled: boolean
} & WebhookFormFields &
  EmailFormFields &
  HttpFormFields

export const defaultForm: DestinationForm = {
  name: '',
  type: 'http',
  secret: '',
  timeout_ms: 10000,
  enabled: true,
  ...Object.assign({}, ...MODULES.map((m) => m.defaults)),
} as DestinationForm

// zod v4's recursive z.lazy degrades inference to unknown[], so pin the resolver type
export const destinationResolver = zodResolver(destinationSchema) as unknown as Resolver<DestinationForm>
