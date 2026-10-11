import { api } from './index'
import type { ListApiReturn, MappingResponse } from './types'

export function listMappings(params?: { page?: number; size?: number }): Promise<ListApiReturn<MappingResponse>> {
  return api.get('/api/v1/mappings', { params })
}

export function getMapping(id: string) {
  return api.get<MappingResponse>(`/api/v1/mappings/${id}`)
}

export function createMapping(data: Partial<MappingResponse>) {
  return api.post<MappingResponse>('/api/v1/mappings', data)
}

export function updateMapping(id: string, data: Partial<MappingResponse>) {
  return api.put<MappingResponse>(`/api/v1/mappings/${id}`, data)
}

export function deleteMapping(id: string) {
  return api.delete(`/api/v1/mappings/${id}`)
}
