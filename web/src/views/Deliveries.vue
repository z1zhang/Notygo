<template>
  <div class="deliveries-container">
    <div class="page-filters">
      <el-select v-model="filters.status" placeholder="Status" clearable style="width: 160px" @change="fetchData">
        <el-option label="Pending" value="pending" />
        <el-option label="Delivering" value="delivering" />
        <el-option label="Delivered" value="delivered" />
        <el-option label="Failed" value="failed" />
        <el-option label="Dead Letter" value="dead_letter" />
      </el-select>
      <el-select v-model="filters.destination_id" placeholder="Destination" clearable style="width: 200px" @change="fetchData">
        <el-option
          v-for="d in destinationOptions"
          :key="d.id"
          :label="d.name"
          :value="d.id"
        />
      </el-select>
    </div>

    <el-card shadow="never" v-loading="loading">
      <el-table :data="tableData" stripe style="width: 100%" @row-click="openDetail">
        <el-table-column label="ID" width="160">
          <template #default="{ row }">
            <span class="truncate-id">{{ truncateId(row.id) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="Event ID" width="160">
          <template #default="{ row }">
            <span class="truncate-id">{{ truncateId(row.event_id) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="destination_name" label="Destination" width="140">
          <template #default="{ row }">
            {{ row.destination_name || row.destination_id || '-' }}
          </template>
        </el-table-column>
        <el-table-column prop="status" label="Status" width="120">
          <template #default="{ row }">
            <el-tag :type="statusTagType(row.status)" size="small">{{ row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="attempts" label="Attempts" width="90" />
        <el-table-column label="Next Attempt" width="180">
          <template #default="{ row }">
            {{ formatTime(row.next_attempt_at) }}
          </template>
        </el-table-column>
        <el-table-column prop="error" label="Error" show-overflow-tooltip />
      </el-table>

      <div class="pagination-wrap" v-if="total > 0">
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          :total="total"
          :page-sizes="[10, 20, 50]"
          layout="total, sizes, prev, pager, next"
          @change="fetchData"
        />
      </div>
    </el-card>

    <el-drawer
      v-model="drawerVisible"
      title="Delivery Detail"
      size="600px"
      destroy-on-close
    >
      <div v-loading="detailLoading">
        <template v-if="detail">
          <el-descriptions :column="2" border class="detail-desc">
            <el-descriptions-item label="ID">{{ detail.id }}</el-descriptions-item>
            <el-descriptions-item label="Status">
              <el-tag :type="statusTagType(detail.status)" size="small">{{ detail.status }}</el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="Event ID">{{ truncateId(detail.event_id) }}</el-descriptions-item>
            <el-descriptions-item label="Destination">{{ detail.destination_name || detail.destination_id || '-' }}</el-descriptions-item>
            <el-descriptions-item label="Attempts">{{ detail.attempts ?? 0 }}</el-descriptions-item>
            <el-descriptions-item label="Next Attempt">{{ formatTime(detail.next_attempt_at) }}</el-descriptions-item>
          </el-descriptions>

          <el-divider content-position="left">Rendered Title</el-divider>
          <div class="rendered-output">{{ detail.rendered_title ?? '-' }}</div>

          <el-divider content-position="left">Rendered Body</el-divider>
          <pre class="rendered-output rendered-body">{{ detail.rendered_body ?? '-' }}</pre>

          <el-divider content-position="left">Delivery Attempts</el-divider>
          <el-table :data="detail.attempts_log ?? []" size="small" border>
            <el-table-column prop="attempt_no" label="#" width="60" />
            <el-table-column prop="status" label="Status" width="110">
              <template #default="{ row }">
                <el-tag :type="statusTagType(row.status)" size="small">{{ row.status }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="duration_ms" label="Duration (ms)" width="120" />
            <el-table-column prop="error" label="Error" show-overflow-tooltip />
          </el-table>
          <el-empty v-if="!(detail.attempts_log?.length)" description="No attempts recorded" :image-size="40" />

          <div class="drawer-actions" v-if="canRetry">
            <el-button type="warning" @click="handleRetry">
              <el-icon><RefreshRight /></el-icon> Retry
            </el-button>
          </div>
        </template>
      </div>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { listDeliveries, getDelivery, retryDelivery } from '@/api/deliveries'
import { listDestinations } from '@/api/destinations'

interface DeliveryRow {
  id: string
  event_id: string
  destination_id: string
  destination_name: string
  status: string
  attempts: number
  next_attempt_at: string
  error: string
  rendered_title: string
  rendered_body: string
  attempts_log: { attempt_no: number; status: string; duration_ms: number; error: string }[]
  [key: string]: unknown
}

const loading = ref(false)
const detailLoading = ref(false)
const drawerVisible = ref(false)
const detail = ref<DeliveryRow | null>(null)

const tableData = ref<DeliveryRow[]>([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(20)

const destinationOptions = ref<{ id: string; name: string }[]>([])

const filters = reactive({
  status: '',
  destination_id: '',
})

function truncateId(id: string): string {
  if (!id) return '-'
  return id.length > 12 ? id.substring(0, 12) + '...' : id
}

function formatTime(val: string): string {
  if (!val) return '-'
  return new Date(val).toLocaleString()
}

function statusTagType(status: string): 'success' | 'warning' | 'danger' | 'info' {
  const map: Record<string, 'success' | 'warning' | 'danger' | 'info'> = {
    pending: 'info',
    delivering: 'warning',
    delivered: 'success',
    failed: 'danger',
    dead_letter: 'danger',
  }
  return map[status] ?? 'info'
}

const canRetry = computed(() => {
  if (!detail.value) return false
  return detail.value.status === 'failed' || detail.value.status === 'dead_letter'
})

async function fetchData() {
  loading.value = true
  try {
    const params: Record<string, unknown> = {
      page: currentPage.value,
      size: pageSize.value,
    }
    if (filters.status) params.status = filters.status
    if (filters.destination_id) params.destination_id = filters.destination_id
    const res = await listDeliveries(params)
    tableData.value = res.data?.items ?? []
    total.value = res.data?.total ?? 0
  } catch (e: unknown) {
    const msg = (e as { response?: { data?: { message?: string } } })?.response?.data?.message || 'Failed to load deliveries'
    ElMessage.error(msg)
  } finally {
    loading.value = false
  }
}

async function fetchDestinationOptions() {
  try {
    const res = await listDestinations({ page: 1, size: 200 })
    destinationOptions.value = (res.data?.items ?? []).map(
      (d: Record<string, unknown>) => ({ id: String(d.id ?? ''), name: String(d.name ?? '') })
    )
  } catch {
    // silent
  }
}

async function openDetail(row: DeliveryRow) {
  drawerVisible.value = true
  detailLoading.value = true
  detail.value = null
  try {
    const res = await getDelivery(row.id)
    detail.value = res.data ?? null
  } catch (e: unknown) {
    const msg = (e as { response?: { data?: { message?: string } } })?.response?.data?.message || 'Failed to load delivery detail'
    ElMessage.error(msg)
  } finally {
    detailLoading.value = false
  }
}

async function handleRetry() {
  if (!detail.value) return
  try {
    await retryDelivery(detail.value.id)
    ElMessage.success('Delivery retry initiated')
    // Refresh detail
    const res = await getDelivery(detail.value.id)
    detail.value = res.data ?? null
  } catch (e: unknown) {
    const msg = (e as { response?: { data?: { message?: string } } })?.response?.data?.message || 'Retry failed'
    ElMessage.error(msg)
  }
}

onMounted(() => {
  fetchData()
  fetchDestinationOptions()
})
</script>

<style lang="scss" scoped>
.deliveries-container {
  .page-filters {
    margin-bottom: 16px;
    display: flex;
    gap: 12px;
  }

  .pagination-wrap {
    margin-top: 16px;
    display: flex;
    justify-content: flex-end;
  }

  .truncate-id {
    font-family: monospace;
    font-size: 12px;
    cursor: pointer;
  }

  .detail-desc {
    margin-bottom: 12px;
  }

  .rendered-output {
    background-color: #f5f7fa;
    padding: 12px;
    border-radius: 4px;
    font-size: 14px;
    white-space: pre-wrap;
    word-break: break-all;
  }

  .rendered-body {
    font-family: 'Courier New', Courier, monospace;
    font-size: 13px;
  }

  .drawer-actions {
    margin-top: 20px;
    display: flex;
    justify-content: flex-end;
  }
}
</style>
