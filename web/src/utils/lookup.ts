/**
 * Look up a name from an options list by ID.
 */

interface NameOption {
  id: string
  name: string
}

/**
 * Find the name for a given ID in an options list.
 * Returns the fallback (or the ID prefix) if not found.
 */
export function lookupName(options: NameOption[], id: string, fallback?: string): string {
  if (!id) return fallback ?? '-'
  const found = options.find((o) => o.id === id)
  return found?.name ?? fallback ?? id.slice(0, 8)
}
