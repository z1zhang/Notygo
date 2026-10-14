import { ref } from 'vue'
import { listSources } from '@/api/sources'
import { listDestinations } from '@/api/destinations'
import { listTemplates } from '@/api/templates'
import { listMappings } from '@/api/mappings'
import { listRules } from '@/api/rules'

export interface NameOption {
  id: string
  name: string
}

/**
 * Composable for loading dropdown option lists (sources, destinations, templates, mappings, rules).
 * Each option list is loaded independently and cached in refs.
 */
export function useOptions() {
  const sourceOptions = ref<NameOption[]>([])
  const destinationOptions = ref<NameOption[]>([])
  const templateOptions = ref<NameOption[]>([])
  const mappingOptions = ref<NameOption[]>([])
  const ruleOptions = ref<NameOption[]>([])

  function mapToOptions(data: unknown[]): NameOption[] {
    return data.map((item) => {
      const obj = item as Record<string, unknown>
      return {
        id: String(obj.id ?? ''),
        name: String(obj.name ?? ''),
      }
    })
  }

  async function fetchSourceOptions() {
    try {
      const res = await listSources({ page: 1, size: 200 })
      sourceOptions.value = mapToOptions(res.data?.data ?? [])
    } catch { /* silent */ }
  }

  async function fetchDestinationOptions() {
    try {
      const res = await listDestinations({ page: 1, size: 200 })
      destinationOptions.value = mapToOptions(res.data?.data ?? [])
    } catch { /* silent */ }
  }

  async function fetchTemplateOptions() {
    try {
      const res = await listTemplates({ page: 1, size: 200 })
      templateOptions.value = mapToOptions(res.data?.data ?? [])
    } catch { /* silent */ }
  }

  async function fetchMappingOptions() {
    try {
      const res = await listMappings({ page: 1, size: 200 })
      mappingOptions.value = mapToOptions(res.data?.data ?? [])
    } catch { /* silent */ }
  }

  async function fetchRuleOptions() {
    try {
      const res = await listRules({ page: 1, size: 200 })
      ruleOptions.value = mapToOptions(res.data?.data ?? [])
    } catch { /* silent */ }
  }

  /**
   * Load multiple option lists at once.
   * @param keys - which option lists to load
   */
  async function fetchAll(keys: ('sources' | 'destinations' | 'templates' | 'mappings' | 'rules')[] = ['sources', 'destinations', 'templates', 'mappings', 'rules']) {
    const tasks: Promise<void>[] = []
    if (keys.includes('sources')) tasks.push(fetchSourceOptions())
    if (keys.includes('destinations')) tasks.push(fetchDestinationOptions())
    if (keys.includes('templates')) tasks.push(fetchTemplateOptions())
    if (keys.includes('mappings')) tasks.push(fetchMappingOptions())
    if (keys.includes('rules')) tasks.push(fetchRuleOptions())
    await Promise.all(tasks)
  }

  return {
    sourceOptions,
    destinationOptions,
    templateOptions,
    mappingOptions,
    ruleOptions,
    fetchSourceOptions,
    fetchDestinationOptions,
    fetchTemplateOptions,
    fetchMappingOptions,
    fetchRuleOptions,
    fetchAll,
  }
}
