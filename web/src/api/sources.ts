import { api } from './index'
import type { ListApiReturn, SourceResponse } from './types'

export function listSources(params?: { page?: number; size?: number }): Promise<ListApiReturn<SourceResponse>> {
  return api.get('/api/v1/sources', { params })
}

export function getSource(id: string) {
  return api.get<SourceResponse>(`/api/v1/sources/${id}`)
}

export function createSource(data: Partial<SourceResponse>) {
  return api.post<SourceResponse>('/api/v1/sources', data)
}

export function updateSource(id: string, data: Partial<SourceResponse>) {
  return api.put<SourceResponse>(`/api/v1/sources/${id}`, data)
}

export function deleteSource(id: string) {
  return api.delete(`/api/v1/sources/${id}`)
}
