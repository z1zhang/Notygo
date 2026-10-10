import { api } from './index'

export function listDeliveries(params: {
  page?: number
  size?: number
  status?: string
  destination_id?: string
  event_id?: string
}) {
  return api.get('/api/v1/deliveries', { params })
}

export function getDelivery(id: string) {
  return api.get(`/api/v1/deliveries/${id}`)
}

export function retryDelivery(id: string) {
  return api.post(`/api/v1/deliveries/${id}/retry`)
}

export function bulkRetry(filter: Record<string, unknown>) {
  return api.post('/api/v1/deliveries/retry', filter)
}
