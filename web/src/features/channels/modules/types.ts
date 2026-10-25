import type { FC } from 'react'
import type { UseFormReturn } from 'react-hook-form'
import type { z } from 'zod'

/** One node of the request-body tree: leaf (value_source) or group (type + children). */
export type BodyFieldNode = {
  key: string
  value_source: string // leaf: 'title' | 'body' | 'format' | 'literal'
  value: string // literal value_source only
  type: string // group: 'object' | 'array'
  children: BodyFieldNode[]
}

/**
 * Loose stand-in for the assembled channel form. Channel modules only
 * type the fields they actually use; the exact form type is assembled in
 * index.ts from the module field interfaces.
 */
export type AnyForm = Record<string, any>

export type ChannelFieldsProps = { form: UseFormReturn<AnyForm> }

/**
 * A channel module is the frontend half of a delivery channel: form fields,
 * config serialization, and backfill. Mirrors the backend sender registry —
 * a new channel is one file plus one line in index.ts.
 */
export interface ChannelModule {
  /** Channel types served by this module (the registry maps type → module). */
  types: readonly string[]
  /** zod fields contributed to the shared channelSchema (raw shape). */
  formSchema: Record<string, z.ZodType>
  /** Default values for the channel fields; also the reset values on type switch. */
  defaults: Record<string, unknown>
  /** Default msg_type per served type, if the channel has msg types. */
  defaultMsgType?: Record<string, string>
  /** Conditional form block rendered when the channel has this type. */
  Fields: FC<ChannelFieldsProps>
  /** Serialize the form values into the channel config JSON. */
  toConfig: (data: AnyForm) => Record<string, unknown>
  /** Backfill form fields from the stored channel config. */
  fromConfig: (config: Record<string, unknown>, type: string) => Record<string, unknown>
  /** Extra superRefine validation (required fields etc.). */
  validate?: (data: AnyForm, ctx: z.RefinementCtx) => void
}

// ── Config parsing helpers (server config is a loosely-typed object) ──────

export function asString(value: unknown): string {
  return typeof value === 'string' ? value : ''
}

export function asNumber(value: unknown, fallback: number): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : fallback
}
