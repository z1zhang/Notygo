import { api } from './index'

export function listMappings(params?: { page?: number; size?: number }) {
  return api.get('/api/v1/mappings', { params: params ?? {} })
}

export function getMapping(id: string) {
  return api.get(`/api/v1/mappings/${id}`)
}

export function createMapping(data: Record<string, unknown>) {
  return api.post('/api/v1/mappings', data)
}

export function updateMapping(id: string, data: Record<string, unknown>) {
  return api.put(`/api/v1/mappings/${id}`, data)
}

export function deleteMapping(id: string) {
  return api.delete(`/api/v1/mappings/${id}`)
}
