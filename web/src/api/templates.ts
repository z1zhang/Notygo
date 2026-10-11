import { api } from './index'
import type { ListApiReturn, TemplateResponse } from './types'

export function listTemplates(params?: { page?: number; size?: number }): Promise<ListApiReturn<TemplateResponse>> {
  return api.get('/api/v1/templates', { params })
}

export function getTemplate(id: string) {
  return api.get<TemplateResponse>(`/api/v1/templates/${id}`)
}

export function createTemplate(data: Partial<TemplateResponse>) {
  return api.post<TemplateResponse>('/api/v1/templates', data)
}

export function updateTemplate(id: string, data: Partial<TemplateResponse>) {
  return api.put<TemplateResponse>(`/api/v1/templates/${id}`, data)
}

export function deleteTemplate(id: string) {
  return api.delete(`/api/v1/templates/${id}`)
}

export function previewTemplate(data: Record<string, unknown>) {
  return api.post('/api/v1/preview/template', data)
}
