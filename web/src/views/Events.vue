<template>
  <div class="events-container">
    <div class="page-filters">
      <el-select v-model="filters.status" placeholder="Status" clearable style="width: 160px" @change="fetchData">
        <el-option label="Received" value="received" />
        <el-option label="Processing" value="processing" />
        <el-option label="Delivered" value="delivered" />
        <el-option label="Failed" value="failed" />
      </el-select>
      <el-select v-model="filters.source_id" placeholder="Source" clearable style="width: 200px" @change="fetchData">
        <el-option
          v-for="s in sourceOptions"
          :key="s.id"
          :label="s.name"
          :value="s.id"
        />
      </el-select>
    </div>

    <el-card shadow="never" v-loading="loading">
      <el-table :data="tableData" stripe style="width: 100%" @row-click="openDetail">
        <el-table-column label="ID" width="180">
          <template #default="{ row }">
            <span class="truncate-id">{{ truncateId(row.id) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="source_name" label="Source" width="140">
          <template #default="{ row }">
            {{ row.source_name || row.source_id || '-' }}
          </template>
        </el-table-column>
        <el-table-column prop="status" label="Status" width="120">
          <template #default="{ row }">
            <el-tag :type="statusTagType(row.status)" size="small">{{ row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="received_at" label="Received At" width="180">
          <template #default="{ row }">
            {{ formatTime(row.received_at ?? row.created_at) }}
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
      title="Event Detail"
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
            <el-descriptions-item label="Source">{{ detail.source_name || detail.source_id || '-' }}</el-descriptions-item>
            <el-descriptions-item label="Received At">{{ formatTime(detail.received_at ?? detail.created_at) }}</el-descriptions-item>
          </el-descriptions>

          <el-divider content-position="left">Raw Payload</el-divider>
          <el-input
            :model-value="formatJson(detail.payload)"
            type="textarea"
            :rows="8"
            readonly
            class="monospace-input"
          />

          <el-divider content-position="left">Parsed Fields</el-divider>
          <el-table :data="parsedFields" size="small" border>
            <el-table-column prop="key" label="Key" width="180" />
            <el-table-column prop="value" label="Value" show-overflow-tooltip />
          </el-table>

          <el-divider content-position="left">Matched Rules</el-divider>
          <el-table :data="detail.matched_rules ?? []" size="small" border>
            <el-table-column prop="rule_name" label="Rule" min-width="140" />
            <el-table-column prop="template_name" label="Template" min-width="140" />
          </el-table>
          <el-empty v-if="!(detail.matched_rules?.length)" description="No matched rules" :image-size="40" />

          <el-divider content-position="left">Related Deliveries</el-divider>
          <el-table :data="relatedDeliveries" size="small" border>
            <el-table-column label="ID" width="120">
              <template #default="{ row }">
                {{ truncateId(row.id) }}
              </template>
            </el-table-column>
            <el-table-column prop="destination_name" label="Destination" min-width="120" />
            <el-table-column prop="status" label="Status" width="110">
              <template #default="{ row }">
                <el-tag :type="deliveryStatusTagType(row.status)" size="small">{{ row.status }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="attempts" label="Attempts" width="80" />
          </el-table>

          <div class="drawer-actions">
            <el-button type="warning" @click="handleReplay">
              <el-icon><RefreshRight /></el-icon> Replay
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
import { listEvents, getEvent, replayEvent } from '@/api/events'
import { listDeliveries } from '@/api/deliveries'
import { listSources } from '@/api/sources'

interface MatchedRule {
  rule_name: string
  template_name: string
  [key: string]: unknown
}

interface EventRow {
  id: string
  source_id: string
  source_name: string
  status: string
  received_at: string
  created_at: string
  error: string
  payload: unknown
  parsed_fields: Record<string, unknown>
  matched_rules: MatchedRule[]
  [key: string]: unknown
}

interface KeyValue {
  key: string
  value: string
}

const loading = ref(false)
const detailLoading = ref(false)
const drawerVisible = ref(false)
const detail = ref<EventRow | null>(null)
const relatedDeliveries = ref<Record<string, unknown>[]>([])

const tableData = ref<EventRow[]>([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(20)

const sourceOptions = ref<{ id: string; name: string }[]>([])

const filters = reactive({
  status: '',
  source_id: '',
})

function truncateId(id: string): string {
  if (!id) return '-'
  return id.length > 12 ? id.substring(0, 12) + '...' : id
}

function formatTime(val: string): string {
  if (!val) return '-'
  return new Date(val).toLocaleString()
}

function formatJson(obj: unknown): string {
  if (!obj) return ''
  try {
    return JSON.stringify(obj, null, 2)
  } catch {
    return String(obj)
  }
}

function statusTagType(status: string): 'success' | 'warning' | 'danger' | 'info' {
  const map: Record<string, 'success' | 'warning' | 'danger' | 'info'> = {
    received: 'info',
    processing: 'warning',
    delivered: 'success',
    failed: 'danger',
  }
  return map[status] ?? 'info'
}

function deliveryStatusTagType(status: string): 'success' | 'warning' | 'danger' | 'info' {
  const map: Record<string, 'success' | 'warning' | 'danger' | 'info'> = {
    pending: 'info',
    delivering: 'warning',
    delivered: 'success',
    failed: 'danger',
    dead_letter: 'danger',
  }
  return map[status] ?? 'info'
}

const parsedFields = computed<KeyValue[]>(() => {
  if (!detail.value?.parsed_fields) return []
  const fields = detail.value.parsed_fields as Record<string, unknown>
  return Object.entries(fields).map(([key, value]) => ({
    key,
    value: typeof value === 'object' ? JSON.stringify(value) : String(value ?? ''),
  }))
})

async function fetchData() {
  loading.value = true
  try {
    const params: Record<string, unknown> = {
      page: currentPage.value,
      size: pageSize.value,
    }
    if (filters.status) params.status = filters.status
    if (filters.source_id) params.source_id = filters.source_id
    const res = await listEvents(params)
    tableData.value = res.data?.items ?? []
    total.value = res.data?.total ?? 0
  } catch (e: unknown) {
    const msg = (e as { response?: { data?: { message?: string } } })?.response?.data?.message || 'Failed to load events'
    ElMessage.error(msg)
  } finally {
    loading.value = false
  }
}

async function fetchSourceOptions() {
  try {
    const res = await listSources({ page: 1, size: 200 })
    sourceOptions.value = (res.data?.items ?? []).map(
      (s: Record<string, unknown>) => ({ id: String(s.id ?? ''), name: String(s.name ?? '') })
    )
  } catch {
    // silent
  }
}

async function openDetail(row: EventRow) {
  drawerVisible.value = true
  detailLoading.value = true
  detail.value = null
  relatedDeliveries.value = []
  try {
    const res = await getEvent(row.id)
    detail.value = res.data ?? null
    // Fetch related deliveries
    const delRes = await listDeliveries({ page: 1, size: 50, event_id: row.id })
    relatedDeliveries.value = delRes.data?.items ?? []
  } catch (e: unknown) {
    const msg = (e as { response?: { data?: { message?: string } } })?.response?.data?.message || 'Failed to load event detail'
    ElMessage.error(msg)
  } finally {
    detailLoading.value = false
  }
}

async function handleReplay() {
  if (!detail.value) return
  try {
    await replayEvent(detail.value.id)
    ElMessage.success('Event replay initiated')
  } catch (e: unknown) {
    const msg = (e as { response?: { data?: { message?: string } } })?.response?.data?.message || 'Replay failed'
    ElMessage.error(msg)
  }
}

onMounted(() => {
  fetchData()
  fetchSourceOptions()
})
</script>

<style lang="scss" scoped>
.events-container {
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

  :deep(.monospace-input .el-textarea__inner) {
    font-family: 'Courier New', Courier, monospace;
    font-size: 13px;
  }

  .detail-desc {
    margin-bottom: 12px;
  }

  .drawer-actions {
    margin-top: 20px;
    display: flex;
    justify-content: flex-end;
  }
}
</style>
