import { useQuery } from '@tanstack/react-query'
import { listChannels, listMappings, listRules, listEndpoints, listTemplates } from '@/lib/api'

export interface LookupItem {
  id: string
  name: string
}

function toItems(rows: { id: string; name: string }[] | undefined): LookupItem[] {
  return (rows ?? []).map((r) => ({ id: r.id, name: r.name }))
}

export function useEndpointOptions() {
  return useQuery({
    queryKey: ['lookup', 'endpoints'],
    queryFn: () => listEndpoints({ page: 1, size: 100 }),
    select: (res) => toItems(res.data),
  })
}

export function useMappingOptions() {
  return useQuery({
    queryKey: ['lookup', 'mappings'],
    queryFn: () => listMappings({ page: 1, size: 100 }),
    select: (res) => toItems(res.data),
  })
}

export function useRuleOptions() {
  return useQuery({
    queryKey: ['lookup', 'rules'],
    queryFn: () => listRules({ page: 1, size: 100 }),
    select: (res) => toItems(res.data),
  })
}

export function useTemplateOptions() {
  return useQuery({
    queryKey: ['lookup', 'templates'],
    queryFn: () => listTemplates({ page: 1, size: 100 }),
    select: (res) => toItems(res.data),
  })
}

export function useChannelOptions() {
  return useQuery({
    queryKey: ['lookup', 'channels'],
    queryFn: () => listChannels({ page: 1, size: 100 }),
    select: (res) => toItems(res.data),
  })
}

/** Resolve an entity name from lookup items; '-' when missing. */
export function lookupName(items: LookupItem[] | undefined, id: string | null | undefined): string {
  if (!id) return '-'
  return items?.find((i) => i.id === id)?.name ?? id
}
