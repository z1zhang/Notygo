/**
 * Format a time string for display.
 * Accepts ISO strings from the API (e.g., "2024-01-15T10:30:00.000Z")
 * and returns a localized string like "2024/1/15 18:30:00".
 */
export function formatTime(val: string | null | undefined): string {
  if (!val) return '-'
  try {
    return new Date(val).toLocaleString()
  } catch {
    return '-'
  }
}
