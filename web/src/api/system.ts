import { api } from './index'

export function getHealthz() {
  return api.get('/healthz')
}

export function getVersion() {
  return api.get('/api/v1/version')
}
