import { api } from './index'
import type { AxiosResponse } from 'axios'
import type { MappingEntry } from './types'

// Types for preview responses
export interface StructureInfo {
  type: string        // "array" | "object"
  root_is_array: boolean
  item_count: number
}

export interface SuggestedField {
  source_path: string
  target_key: string
  value_type: string
  multi_result_mode: string | null
  sample_value: string
}

export interface PreviewParseResponse {
  structure: StructureInfo
  suggested_fields: SuggestedField[]
}

export interface ValidationError {
  field: string
  code: string
  detail: string
}

export interface PreviewMappingResponse {
  parsed_fields: Record<string, unknown>
  validation_errors: ValidationError[]
}

export interface PreviewDeliveryResponse {
  parsed_fields: Record<string, unknown>
  validation_errors: ValidationError[]
  rendered_title: string
  rendered_body: string
  format: string
}

export function previewParse(payload: string): Promise<AxiosResponse<PreviewParseResponse>> {
  return api.post('/api/v1/preview/parse', { payload })
}

export function previewMapping(payload: string, fields: MappingEntry[]): Promise<AxiosResponse<PreviewMappingResponse>> {
  return api.post('/api/v1/preview/mapping', { payload, fields })
}

export function previewDelivery(payload: string, fields: MappingEntry[], template: { title_template: string; body_template: string; format: string }): Promise<AxiosResponse<PreviewDeliveryResponse>> {
  return api.post('/api/v1/preview/delivery', { payload, fields, ...template })
}
