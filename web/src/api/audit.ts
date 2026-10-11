import { api } from './index'
import type { ListApiReturn, AuditLogResponse } from './types'

export function listAuditLogs(params: {
  page?: number
  size?: number
  action?: string
  target_type?: string
  start_time?: string
  end_time?: string
}): Promise<ListApiReturn<AuditLogResponse>> {
  return api.get('/api/v1/audit', { params })
}
