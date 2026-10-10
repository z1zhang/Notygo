import { api } from './index'

export function listTemplates(params?: { page?: number; size?: number }) {
  return api.get('/api/v1/templates', { params: params ?? {} })
}

export function getTemplate(id: string) {
  return api.get(`/api/v1/templates/${id}`)
}

export function createTemplate(data: Record<string, unknown>) {
  return api.post('/api/v1/templates', data)
}

export function updateTemplate(id: string, data: Record<string, unknown>) {
  return api.put(`/api/v1/templates/${id}`, data)
}

export function deleteTemplate(id: string) {
  return api.delete(`/api/v1/templates/${id}`)
}

export function previewTemplate(data: Record<string, unknown>) {
  return api.post('/api/v1/preview/template', data)
}
