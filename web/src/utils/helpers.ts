/**
 * Truncate an ID string for display, showing the first `len` characters.
 */
export function truncateId(id: string, len = 8): string {
  if (!id) return ''
  return id.length > len ? id.slice(0, len) + '...' : id
}

/**
 * Extract an error message from an unknown error (typically an AxiosError).
 * Falls back to the provided i18n message if no server message is found.
 */
export function getErrorMessage(e: unknown, fallback: string): string {
  const err = e as { response?: { data?: { message?: string } } }
  return err?.response?.data?.message || fallback
}
