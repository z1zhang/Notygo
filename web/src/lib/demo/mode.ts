// Demo mode: when VITE_DEMO_MODE=true the app runs without a backend — all
// API requests are intercepted and answered from browser-local mock data
// (see ./handler.ts). Used for the public preview site on Netlify.

// Guarded so the demo modules stay importable outside Vite (e.g. tests)
const env = (import.meta as { env?: Record<string, string | undefined> }).env

export const isDemoMode = env?.VITE_DEMO_MODE === 'true'

/** Error thrown by demo handlers; api-client converts it to an ApiError. */
export class DemoHttpError extends Error {
  status: number
  code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.name = 'DemoHttpError'
    this.status = status
    this.code = code
  }
}
