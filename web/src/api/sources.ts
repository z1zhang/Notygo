import { api } from './index'

export function listSources(params?: { page?: number; size?: number }) {
  return api.get('/api/v1/sources', { params: params ?? {} })
}

export function getSource(id: string) {
  return api.get(`/api/v1/sources/${id}`)
}

export function createSource(data: Record<string, unknown>) {
  return api.post('/api/v1/sources', data)
}

export function updateSource(id: string, data: Record<string, unknown>) {
  return api.put(`/api/v1/sources/${id}`, data)
}

export function deleteSource(id: string) {
  return api.delete(`/api/v1/sources/${id}`)
}
