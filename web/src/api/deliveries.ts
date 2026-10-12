import { api } from './index'
import type { AxiosResponse } from 'axios'
import type { ListApiReturn, DeliveryResponse, DeliveryAttemptResponse } from './types'

export interface GetDeliveryResponse {
  delivery: DeliveryResponse
  delivery_attempts: DeliveryAttemptResponse[]
}

export function listDeliveries(params: {
  page?: number
  size?: number
  status?: string
  destination_id?: string
  event_id?: string
}): Promise<ListApiReturn<DeliveryResponse>> {
  return api.get('/api/v1/deliveries', { params })
}

export function getDelivery(id: string): Promise<AxiosResponse<GetDeliveryResponse>> {
  return api.get(`/api/v1/deliveries/${id}`)
}

export function retryDelivery(id: string) {
  return api.post(`/api/v1/deliveries/${id}/retry`)
}

export function bulkRetry(filter: Record<string, unknown>) {
  return api.post('/api/v1/deliveries/retry', filter)
}
