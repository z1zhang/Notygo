import { api } from './index'
import type { ListApiReturn, EventResponse } from './types'

export function listEvents(params: {
  page?: number
  size?: number
  status?: string
  source_id?: string
}): Promise<ListApiReturn<EventResponse>> {
  return api.get('/api/v1/events', { params })
}

export function getEvent(id: string) {
  return api.get<EventResponse>(`/api/v1/events/${id}`)
}

export function replayEvent(id: string) {
  return api.post(`/api/v1/events/${id}/replay`)
}
