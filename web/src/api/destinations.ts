import { api } from './index'
import type { ListApiReturn, DestinationResponse } from './types'

export function listDestinations(params?: { page?: number; size?: number }): Promise<ListApiReturn<DestinationResponse>> {
  return api.get('/api/v1/destinations', { params })
}

export function getDestination(id: string) {
  return api.get<DestinationResponse>(`/api/v1/destinations/${id}`)
}

export function createDestination(data: Partial<DestinationResponse>) {
  return api.post<DestinationResponse>('/api/v1/destinations', data)
}

export function updateDestination(id: string, data: Partial<DestinationResponse>) {
  return api.put<DestinationResponse>(`/api/v1/destinations/${id}`, data)
}

export function deleteDestination(id: string) {
  return api.delete(`/api/v1/destinations/${id}`)
}
