import { apiClient } from './api-client'
import type {
  AuditLogResponse,
  DashboardStatsResponse,
  ChannelResponse,
  GetDeliveryResponse,
  GetEventResponse,
  LoginResponse,
  MappingEntry,
  MappingResponse,
  PaginatedResponse,
  PreviewDeliveryResponse,
  PreviewMappingResponse,
  PreviewParseResponse,
  PreviewTemplateResponse,
  RuleResponse,
  SystemInfoResponse,
  EndpointResponse,
  TemplateResponse,
  DeliveryResponse,
  EventResponse,
  QueueStatsResponse,
} from './types'

const BASE = '/api/v1'

// ── Auth ─────────────────────────────────────────────────

export const login = (username: string, password: string) =>
  apiClient.post<LoginResponse>(`${BASE}/auth/login`, { username, password })

// ── Endpoints ──────────────────────────────────────────────

export const listEndpoints = (params?: { page?: number; size?: number }) =>
  apiClient.get<PaginatedResponse<EndpointResponse>>(`${BASE}/endpoints`, params)

export const getEndpoint = (id: string) => apiClient.get<EndpointResponse>(`${BASE}/endpoints/${id}`)

export const createEndpoint = (data: Record<string, unknown>) =>
  apiClient.post<EndpointResponse>(`${BASE}/endpoints`, data)

export const updateEndpoint = (id: string, data: Record<string, unknown>) =>
  apiClient.put<EndpointResponse>(`${BASE}/endpoints/${id}`, data)

export const deleteEndpoint = (id: string) => apiClient.delete<void>(`${BASE}/endpoints/${id}`)

// ── Mappings ─────────────────────────────────────────────

export const listMappings = (params?: { page?: number; size?: number }) =>
  apiClient.get<PaginatedResponse<MappingResponse>>(`${BASE}/mappings`, params)

export const getMapping = (id: string) => apiClient.get<MappingResponse>(`${BASE}/mappings/${id}`)

export const createMapping = (data: Record<string, unknown>) =>
  apiClient.post<MappingResponse>(`${BASE}/mappings`, data)

export const updateMapping = (id: string, data: Record<string, unknown>) =>
  apiClient.put<MappingResponse>(`${BASE}/mappings/${id}`, data)

export const deleteMapping = (id: string) => apiClient.delete<void>(`${BASE}/mappings/${id}`)

// ── Rules ────────────────────────────────────────────────

export const listRules = (params?: { page?: number; size?: number }) =>
  apiClient.get<PaginatedResponse<RuleResponse>>(`${BASE}/rules`, params)

export const getRule = (id: string) => apiClient.get<RuleResponse>(`${BASE}/rules/${id}`)

export const createRule = (data: Record<string, unknown>) =>
  apiClient.post<RuleResponse>(`${BASE}/rules`, data)

export const updateRule = (id: string, data: Record<string, unknown>) =>
  apiClient.put<RuleResponse>(`${BASE}/rules/${id}`, data)

export const deleteRule = (id: string) => apiClient.delete<void>(`${BASE}/rules/${id}`)

// ── Templates ────────────────────────────────────────────

export const listTemplates = (params?: { page?: number; size?: number }) =>
  apiClient.get<PaginatedResponse<TemplateResponse>>(`${BASE}/templates`, params)

export const getTemplate = (id: string) => apiClient.get<TemplateResponse>(`${BASE}/templates/${id}`)

export const createTemplate = (data: Record<string, unknown>) =>
  apiClient.post<TemplateResponse>(`${BASE}/templates`, data)

export const updateTemplate = (id: string, data: Record<string, unknown>) =>
  apiClient.put<TemplateResponse>(`${BASE}/templates/${id}`, data)

export const deleteTemplate = (id: string) => apiClient.delete<void>(`${BASE}/templates/${id}`)

// ── Channels ─────────────────────────────────────────

export const listChannels = (params?: { page?: number; size?: number }) =>
  apiClient.get<PaginatedResponse<ChannelResponse>>(`${BASE}/channels`, params)

export const getChannel = (id: string) =>
  apiClient.get<ChannelResponse>(`${BASE}/channels/${id}`)

export const createChannel = (data: Record<string, unknown>) =>
  apiClient.post<ChannelResponse>(`${BASE}/channels`, data)

export const updateChannel = (id: string, data: Record<string, unknown>) =>
  apiClient.put<ChannelResponse>(`${BASE}/channels/${id}`, data)

export const deleteChannel = (id: string) => apiClient.delete<void>(`${BASE}/channels/${id}`)

export const testChannel = (id: string) =>
  apiClient.post<{ success: boolean; status_code?: number; response_body?: string }>(
    `${BASE}/channels/${id}/test`,
  )

// ── Events ───────────────────────────────────────────────

export const listEvents = (params: {
  page?: number
  size?: number
  status?: string
  endpoint_id?: string
}) => apiClient.get<PaginatedResponse<EventResponse>>(`${BASE}/events`, params)

export const getEvent = (id: string) => apiClient.get<GetEventResponse>(`${BASE}/events/${id}`)

export const replayEvent = (id: string) =>
  apiClient.post<{ event_id: string; message: string }>(`${BASE}/events/${id}/replay`)

// ── Deliveries ───────────────────────────────────────────

export const listDeliveries = (params: {
  page?: number
  size?: number
  status?: string
  channel_id?: string
  event_id?: string
}) => apiClient.get<PaginatedResponse<DeliveryResponse>>(`${BASE}/deliveries`, params)

export const getDelivery = (id: string) =>
  apiClient.get<GetDeliveryResponse>(`${BASE}/deliveries/${id}`)

export const retryDelivery = (id: string) =>
  apiClient.post<{ delivery: DeliveryResponse; message: string }>(`${BASE}/deliveries/${id}/retry`)

export const bulkRetry = (filter: { status?: string; channel_id?: string }) =>
  apiClient.post<{ reset_count: number; message: string }>(`${BASE}/deliveries/bulk-retry`, filter)

// ── Audit ────────────────────────────────────────────────

export const listAuditLogs = (params: {
  page?: number
  size?: number
  action?: string
  target_type?: string
  start_time?: string
  end_time?: string
}) => apiClient.get<PaginatedResponse<AuditLogResponse>>(`${BASE}/audit`, params)

// ── Dashboard ────────────────────────────────────────────

export const getDashboardStats = () =>
  apiClient.get<DashboardStatsResponse>(`${BASE}/dashboard/stats`)

export const getQueueStats = () =>
  apiClient.get<QueueStatsResponse>(`${BASE}/queue/stats`)

// ── Preview ──────────────────────────────────────────────

export const previewParse = (payload: string) =>
  apiClient.post<PreviewParseResponse>(`${BASE}/preview/parse`, { payload })

export const previewMapping = (payload: string, fields: MappingEntry[]) =>
  apiClient.post<PreviewMappingResponse>(`${BASE}/preview/mapping`, { payload, fields })

export const previewDelivery = (
  payload: string,
  fields: MappingEntry[],
  template: { title_template: string; body_template: string; format: string },
) =>
  apiClient.post<PreviewDeliveryResponse>(`${BASE}/preview/delivery`, {
    payload,
    fields,
    ...template,
  })

export const previewTemplate = (
  spec: {
    template_id?: string
    title_template?: string
    body_template?: string
    format?: string
  },
  sampleData: Record<string, unknown>,
) =>
  apiClient.post<PreviewTemplateResponse>(`${BASE}/preview/template`, {
    ...spec,
    sample_data: sampleData,
  })

// ── System ───────────────────────────────────────────────

export const getHealthz = () => apiClient.get<{ status: string }>('/healthz')

export const getSystemInfo = () => apiClient.get<SystemInfoResponse>(`${BASE}/system`)
