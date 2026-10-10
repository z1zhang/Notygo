import { api } from './index'

export function listEvents(params: {
  page?: number
  size?: number
  status?: string
  source_id?: string
}) {
  return api.get('/api/v1/events', { params })
}

export function getEvent(id: string) {
  return api.get(`/api/v1/events/${id}`)
}

export function replayEvent(id: string) {
  return api.post(`/api/v1/events/${id}/replay`)
}
