import type { ApiErrorEnvelope } from './types'

const TOKEN_KEY = 'token'
const EXPIRES_KEY = 'expiresAt'

export class ApiError extends Error {
  status: number
  code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

export function getToken(): string | null {
  try {
    return localStorage.getItem(TOKEN_KEY)
  } catch {
    return null
  }
}

export function getExpiresAt(): string | null {
  try {
    return localStorage.getItem(EXPIRES_KEY)
  } catch {
    return null
  }
}

export function setSession(token: string, expiresAt?: string) {
  localStorage.setItem(TOKEN_KEY, token)
  if (expiresAt) {
    localStorage.setItem(EXPIRES_KEY, expiresAt)
  } else {
    localStorage.removeItem(EXPIRES_KEY)
  }
}

export function clearSession() {
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(EXPIRES_KEY)
}

/** True when a stored token exists and its expiry (if any) is in the future. */
export function hasValidSession(): boolean {
  const token = getToken()
  if (!token) return false
  const expires = getExpiresAt()
  if (expires) {
    return new Date(expires).getTime() > Date.now()
  }
  return true
}

function handleUnauthorized() {
  clearSession()
  // Redirect to login preserving the current location (skip if already there)
  if (!window.location.pathname.startsWith('/login')) {
    const redirect = encodeURIComponent(window.location.pathname + window.location.search)
    window.location.assign(`/login?redirect=${redirect}`)
  }
}

async function request<T>(
  method: string,
  path: string,
  body?: unknown,
  params?: Record<string, string | number | undefined>,
): Promise<T> {
  let url = path
  if (params) {
    const search = new URLSearchParams()
    for (const [key, value] of Object.entries(params)) {
      if (value !== undefined && value !== '') {
        search.set(key, String(value))
      }
    }
    const qs = search.toString()
    if (qs) url = `${path}?${qs}`
  }

  const headers: Record<string, string> = {}
  const token = getToken()
  if (token) {
    headers.Authorization = `Bearer ${token}`
  }
  if (body !== undefined) {
    headers['Content-Type'] = 'application/json'
  }

  const controller = new AbortController()
  const timeout = setTimeout(() => controller.abort(), 30_000)

  let response: Response
  try {
    response = await fetch(url, {
      method,
      headers,
      body: body !== undefined ? JSON.stringify(body) : undefined,
      signal: controller.signal,
    })
  } catch (e) {
    if (e instanceof DOMException && e.name === 'AbortError') {
      throw new ApiError(0, 'NETWORK_TIMEOUT', '请求超时')
    }
    throw new ApiError(0, 'NETWORK_ERROR', '网络错误，请检查连接')
  } finally {
    clearTimeout(timeout)
  }

  if (response.status === 401) {
    handleUnauthorized()
    throw new ApiError(401, 'INGEST_001', '会话已过期')
  }

  if (response.status === 204) {
    return undefined as T
  }

  // Parse JSON when possible; some error responses may be empty
  let payload: unknown = null
  const text = await response.text()
  if (text) {
    try {
      payload = JSON.parse(text)
    } catch {
      payload = null
    }
  }

  if (!response.ok) {
    const envelope = payload as ApiErrorEnvelope | null
    const code = envelope?.error?.code ?? 'HTTP_' + response.status
    const message = envelope?.error?.message ?? `请求失败（HTTP ${response.status}）`
    throw new ApiError(response.status, code, message)
  }

  return payload as T
}

export const apiClient = {
  get: <T>(path: string, params?: Record<string, string | number | undefined>) =>
    request<T>('GET', path, undefined, params),
  post: <T>(path: string, body?: unknown) => request<T>('POST', path, body),
  put: <T>(path: string, body?: unknown) => request<T>('PUT', path, body),
  delete: <T>(path: string) => request<T>('DELETE', path),
}

/** Extract a display message from an unknown error. */
export function getErrorMessage(e: unknown, fallback: string): string {
  if (e instanceof ApiError && e.message) {
    // Server messages are in English; show code + message for clarity
    return `${e.message}（${e.code}）`
  }
  if (e instanceof Error && e.message) {
    return e.message
  }
  return fallback
}
