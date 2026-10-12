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

/**
 * Copy text to clipboard. Falls back to execCommand for non-secure contexts
 * (e.g. HTTP access in Docker deployments without TLS).
 */
export async function copyToClipboard(text: string): Promise<void> {
  if (navigator.clipboard) {
    await navigator.clipboard.writeText(text)
  } else {
    const ta = document.createElement('textarea')
    ta.value = text
    ta.style.position = 'fixed'
    ta.style.left = '-9999px'
    document.body.appendChild(ta)
    ta.select()
    document.execCommand('copy')
    document.body.removeChild(ta)
  }
}
