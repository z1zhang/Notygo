// Demo state persistence: the whole mock database lives in localStorage so
// demo visitors can add/edit/delete and see their changes survive a refresh.
// Clearing site data resets the demo back to the seed.
import { buildSeed, type DemoState } from './seed'

const STORE_KEY = 'zpush-demo-db'

let cached: DemoState | null = null

export function loadState(): DemoState {
  if (cached) return cached
  try {
    const raw = localStorage.getItem(STORE_KEY)
    if (raw) {
      cached = JSON.parse(raw) as DemoState
      return cached
    }
  } catch {
    // corrupted store — fall through and reseed
  }
  cached = buildSeed()
  persist()
  return cached
}

export function persist() {
  if (!cached) return
  try {
    localStorage.setItem(STORE_KEY, JSON.stringify(cached))
  } catch {
    // storage full or unavailable — demo keeps working in memory
  }
}

let idCounter = 0

export function nextId(prefix: string): string {
  idCounter += 1
  return `${prefix}-${Date.now().toString(36)}${idCounter.toString(36)}`
}

export function nowIso(): string {
  return new Date().toISOString()
}
