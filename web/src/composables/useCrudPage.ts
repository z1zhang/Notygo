import { ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import type { AxiosResponse } from 'axios'
import { getErrorMessage } from '@/utils/helpers'
import type { PaginatedResponse } from '@/api/types'

/**
 * Composable for common CRUD page state and operations.
 * Manages loading, saving, dialog visibility, pagination, and data fetching.
 * Form/dialog content is left to each view since they differ too much.
 */
export function useCrudPage<T extends { id?: string; name?: string }>(options: {
  listApi: (params: { page: number; size: number }) => Promise<AxiosResponse<PaginatedResponse<T>>>
  deleteApi: (id: string) => Promise<AxiosResponse>
  entityI18nKey: string // e.g., 'nav.sources' — used for delete confirmation and success messages
}) {
  const { t } = useI18n()

  const loading = ref(false)
  const saving = ref(false)
  const dialogVisible = ref(false)
  const isEditing = ref(false)
  const editingId = ref('')
  const tableData = ref<T[]>([]) as { value: T[] }
  const total = ref(0)
  const currentPage = ref(1)
  const pageSize = ref(20)

  async function fetchData() {
    loading.value = true
    try {
      const res = await options.listApi({ page: currentPage.value, size: pageSize.value })
      tableData.value = res.data?.data ?? []
      total.value = res.data?.total ?? 0
    } catch (e: unknown) {
      ElMessage.error(getErrorMessage(e, t('common.failedToLoad', { entity: t(options.entityI18nKey) })))
    } finally {
      loading.value = false
    }
  }

  async function handleDelete(id: string, name?: string) {
    try {
      await ElMessageBox.confirm(
        t('common.confirmDeleteMsg', { entity: t(options.entityI18nKey), name: name ?? id }),
        t('common.confirmDelete'),
        {
          type: 'warning',
          confirmButtonText: t('common.confirm'),
          cancelButtonText: t('common.cancel'),
        }
      )
      await options.deleteApi(id)
      ElMessage.success(t('common.deleted', { entity: t(options.entityI18nKey) }))
      await fetchData()
    } catch {
      // cancelled or error already shown
    }
  }

  function openCreateDialog() {
    isEditing.value = false
    editingId.value = ''
    dialogVisible.value = true
  }

  function openEditDialog(id: string) {
    isEditing.value = true
    editingId.value = id
    dialogVisible.value = true
  }

  function closeDialog() {
    dialogVisible.value = false
  }

  return {
    loading,
    saving,
    dialogVisible,
    isEditing,
    editingId,
    tableData,
    total,
    currentPage,
    pageSize,
    fetchData,
    handleDelete,
    openCreateDialog,
    openEditDialog,
    closeDialog,
  }
}
