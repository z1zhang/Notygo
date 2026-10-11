import { api } from './index'
import type { ListApiReturn, RuleResponse } from './types'

export function listRules(params?: { page?: number; size?: number }): Promise<ListApiReturn<RuleResponse>> {
  return api.get('/api/v1/rules', { params })
}

export function getRule(id: string) {
  return api.get<RuleResponse>(`/api/v1/rules/${id}`)
}

export function createRule(data: Partial<RuleResponse>) {
  return api.post<RuleResponse>('/api/v1/rules', data)
}

export function updateRule(id: string, data: Partial<RuleResponse>) {
  return api.put<RuleResponse>(`/api/v1/rules/${id}`, data)
}

export function deleteRule(id: string) {
  return api.delete(`/api/v1/rules/${id}`)
}
