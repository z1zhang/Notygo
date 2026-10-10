import { api } from './index'

export function listAuditLogs(params: {
  page?: number
  size?: number
  action?: string
  target_type?: string
  start_time?: string
  end_time?: string
}) {
  return api.get('/api/v1/audit', { params })
}
