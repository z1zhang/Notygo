import { api } from './index'

export function listDestinations(params?: { page?: number; size?: number }) {
  return api.get('/api/v1/destinations', { params: params ?? {} })
}

export function getDestination(id: string) {
  return api.get(`/api/v1/destinations/${id}`)
}

export function createDestination(data: Record<string, unknown>) {
  return api.post('/api/v1/destinations', data)
}

export function updateDestination(id: string, data: Record<string, unknown>) {
  return api.put(`/api/v1/destinations/${id}`, data)
}

export function deleteDestination(id: string) {
  return api.delete(`/api/v1/destinations/${id}`)
}
