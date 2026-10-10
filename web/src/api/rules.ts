import { api } from './index'

export function listRules(params?: { page?: number; size?: number }) {
  return api.get('/api/v1/rules', { params: params ?? {} })
}

export function getRule(id: string) {
  return api.get(`/api/v1/rules/${id}`)
}

export function createRule(data: Record<string, unknown>) {
  return api.post('/api/v1/rules', data)
}

export function updateRule(id: string, data: Record<string, unknown>) {
  return api.put(`/api/v1/rules/${id}`, data)
}

export function deleteRule(id: string) {
  return api.delete(`/api/v1/rules/${id}`)
}
