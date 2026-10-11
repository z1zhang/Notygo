<template>
  <div class="events-container">
    <div class="page-filters">
      <el-select v-model="filters.status" :placeholder="$t('common.status')" clearable style="width: 160px" @change="fetchData">
        <el-option :label="$t('events.received')" value="received" />
        <el-option :label="$t('events.processing')" value="processing" />
        <el-option :label="$t('events.completed')" value="completed" />
        <el-option :label="$t('events.failed')" value="failed" />
      </el-select>
      <el-select v-model="filters.source_id" :placeholder="$t('events.source')" clearable style="width: 200px" @change="fetchData">
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
        <el-table-column :label="$t('common.id')" width="180">
          <template #default="{ row }">
            <span class="truncate-id">{{ truncateId(row.id) }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('events.source')" width="140">
          <template #default="{ row }">
            {{ getSourceName(row.source_id) }}
          </template>
        </el-table-column>
        <el-table-column prop="status" :label="$t('common.status')" width="120">
          <template #default="{ row }">
            <el-tag :type="statusTagType(row.status)" size="small">{{ $t('enums.eventStatus.' + row.status, row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="$t('events.receivedAt')" width="180">
          <template #default="{ row }">
            {{ formatTime(row.received_at) }}
          </template>
        </el-table-column>
        <el-table-column :label="$t('common.error')" show-overflow-tooltip>
          <template #default="{ row }">
            {{ row.error_summary || row.error_code || '-' }}
          </template>
        </el-table-column>
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
      :title="$t('events.eventDetail')"
      size="640px"
      destroy-on-close
    >
      <div v-loading="detailLoading">
        <template v-if="detail">
          <el-descriptions :column="2" border class="detail-desc">
            <el-descriptions-item :label="$t('common.id')" :span="2">
              <span class="monospace-text">{{ detail.id }}</span>
            </el-descriptions-item>
            <el-descriptions-item :label="$t('common.status')">
              <el-tag :type="statusTagType(detail.status)" size="small">{{ $t('enums.eventStatus.' + detail.status, detail.status) }}</el-tag>
            </el-descriptions-item>
            <el-descriptions-item :label="$t('events.source')">
              {{ getSourceName(detail.source_id) }}
            </el-descriptions-item>
            <el-descriptions-item :label="$t('events.receivedAt')">
              {{ formatTime(detail.received_at) }}
            </el-descriptions-item>
            <el-descriptions-item :label="$t('events.contentType')">
              {{ detail.content_type || '-' }}
            </el-descriptions-item>
            <el-descriptions-item v-if="detail.error_summary" :label="$t('common.error')" :span="2">
              <el-text type="danger">{{ detail.error_code }}: {{ detail.error_summary }}</el-text>
            </el-descriptions-item>
          </el-descriptions>

          <el-divider content-position="left">{{ $t('events.rawPayload') }}</el-divider>
          <el-input
            :model-value="payloadDisplay"
            type="textarea"
            :rows="8"
            readonly
            class="monospace-input"
          />

          <el-divider content-position="left">{{ $t('events.parsedFields') }}</el-divider>
          <el-table v-if="parsedFields.length" :data="parsedFields" size="small" border>
            <el-table-column prop="key" :label="$t('events.key')" width="180" />
            <el-table-column prop="value" :label="$t('events.value')" show-overflow-tooltip />
          </el-table>
          <el-empty v-else :description="$t('events.noParsedFields')" :image-size="40" />

          <el-divider content-position="left">{{ $t('events.eventRun') }}</el-divider>
          <template v-if="latestRun">
            <el-descriptions :column="2" border size="small">
              <el-descriptions-item :label="$t('events.runType')">{{ latestRun.run_type }}</el-descriptions-item>
              <el-descriptions-item :label="$t('common.status')">
                <el-tag :type="latestRun.status === 'completed' ? 'success' : latestRun.status === 'failed' ? 'danger' : 'warning'" size="small">
                  {{ $t('enums.eventStatus.' + latestRun.status, latestRun.status) }}
                </el-tag>
              </el-descriptions-item>
              <el-descriptions-item :label="$t('events.startedAt')">{{ formatTime(latestRun.started_at) }}</el-descriptions-item>
              <el-descriptions-item :label="$t('events.finishedAt')">{{ latestRun.finished_at ? formatTime(latestRun.finished_at) : '-' }}</el-descriptions-item>
            </el-descriptions>

            <div v-if="latestRun.error_summary" style="margin-top: 8px">
              <el-text type="danger" size="small">{{ latestRun.error_code }}: {{ latestRun.error_summary }}</el-text>
            </div>
          </template>
          <el-empty v-else :description="$t('events.noEventRun')" :image-size="40" />

          <el-divider content-position="left">{{ $t('events.matchedRules') }}</el-divider>
          <el-table v-if="matchedRules.length" :data="matchedRules" size="small" border>
            <el-table-column prop="name" :label="$t('events.rule')" min-width="140" />
            <el-table-column prop="priority" :label="$t('rules.priority')" width="90" />
            <el-table-column :label="$t('rules.destinations')" min-width="160">
              <template #default="{ row }">
                <el-tag v-for="dId in (row.destinations ?? []).slice(0, 2)" :key="dId" size="small" style="margin-right: 4px">
                  {{ truncateId(dId) }}
                </el-tag>
                <el-tag v-if="(row.destinations ?? []).length > 2" size="small" type="info">
                  +{{ row.destinations.length - 2 }}
                </el-tag>
              </template>
            </el-table-column>
          </el-table>
          <el-empty v-else :description="$t('events.noMatchedRules')" :image-size="40" />

          <el-divider content-position="left">{{ $t('events.relatedDeliveries') }}</el-divider>
          <el-table v-if="relatedDeliveries.length" :data="relatedDeliveries" size="small" border>
            <el-table-column :label="$t('common.id')" width="120">
              <template #default="{ row }">
                {{ truncateId(row.id) }}
              </template>
            </el-table-column>
            <el-table-column :label="$t('events.destination')" min-width="120">
              <template #default="{ row }">
                {{ row.destination_name || truncateId(row.destination_id || '') }}
              </template>
            </el-table-column>
            <el-table-column prop="status" :label="$t('common.status')" width="110">
              <template #default="{ row }">
                <el-tag :type="deliveryStatusTagType(row.status)" size="small">{{ $t('enums.deliveryStatus.' + row.status, row.status) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="attempt_count" :label="$t('events.attempts')" width="90" />
          </el-table>
          <el-empty v-else :description="$t('events.noDeliveries')" :image-size="40" />

          <div class="drawer-actions">
            <el-button type="warning" @click="handleReplay">
              <el-icon><RefreshRight /></el-icon> {{ $t('events.replay') }}
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
import { useI18n } from 'vue-i18n'
import { RefreshRight } from '@element-plus/icons-vue'
import { listEvents, getEvent, replayEvent } from '@/api/events'
import { listDeliveries } from '@/api/deliveries'
import { listSources } from '@/api/sources'

const { t } = useI18n()

interface RuleSnapshot {
  id: string
  name: string
  priority: number
  conditions: unknown
  destinations: string[]
}

interface EventRunData {
  id: string
  event_id: string
  run_type: string
  status: string
  started_at: string
  finished_at?: string | null
  error_code?: string | null
  error_summary?: string | null
  parsed_fields: Record<string, unknown>
  rule_set_snapshot: RuleSnapshot[]
  matched_rule_ids: string[]
}

interface EventData {
  id: string
  source_id: string
  source_event_id?: string | null
  request_id: string
  received_at: string
  content_type: string
  payload_raw: string
  payload_hash: string
  status: string
  error_code?: string | null
  error_summary?: string | null
  [key: string]: unknown
}

interface KeyValue {
  key: string
  value: string
}

interface DeliveryRow {
  id: string
  destination_id: string
  destination_name?: string
  status: string
  attempt_count: number
  [key: string]: unknown
}

const loading = ref(false)
const detailLoading = ref(false)
const drawerVisible = ref(false)

// Detail state
const detail = ref<EventData | null>(null)
const latestRun = ref<EventRunData | null>(null)
const relatedDeliveries = ref<DeliveryRow[]>([])

const tableData = ref<EventData[]>([])
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

function formatTime(val: string | null | undefined): string {
  if (!val) return '-'
  try {
    return new Date(val).toLocaleString()
  } catch {
    return val
  }
}

function getSourceName(sourceId: string): string {
  if (!sourceId) return '-'
  return sourceOptions.value.find((s) => s.id === sourceId)?.name ?? truncateId(sourceId)
}

function statusTagType(status: string): 'success' | 'warning' | 'danger' | 'info' {
  const map: Record<string, 'success' | 'warning' | 'danger' | 'info'> = {
    received: 'info',
    processing: 'warning',
    completed: 'success',
    failed: 'danger',
    expired: 'info',
  }
  return map[status] ?? 'info'
}

function deliveryStatusTagType(status: string): 'success' | 'warning' | 'danger' | 'info' {
  const map: Record<string, 'success' | 'warning' | 'danger' | 'info'> = {
    pending: 'info',
    sending: 'warning',
    success: 'success',
    retry_wait: 'warning',
    dead_letter: 'danger',
  }
  return map[status] ?? 'info'
}

// Parse payload_raw (JSON string) for display
const payloadDisplay = computed<string>(() => {
  if (!detail.value?.payload_raw) return ''
  try {
    const parsed = JSON.parse(detail.value.payload_raw)
    return JSON.stringify(parsed, null, 2)
  } catch {
    return detail.value.payload_raw
  }
})

// Parse fields from latest run
const parsedFields = computed<KeyValue[]>(() => {
  if (!latestRun.value?.parsed_fields) return []
  const fields = latestRun.value.parsed_fields
  return Object.entries(fields).map(([key, value]) => ({
    key,
    value: typeof value === 'object' ? JSON.stringify(value) : String(value ?? ''),
  }))
})

// Build matched rules from rule_set_snapshot filtered by matched_rule_ids
const matchedRules = computed<RuleSnapshot[]>(() => {
  if (!latestRun.value) return []
  const { rule_set_snapshot, matched_rule_ids } = latestRun.value
  if (!matched_rule_ids?.length || !rule_set_snapshot?.length) return []
  return rule_set_snapshot.filter((r) => matched_rule_ids.includes(r.id))
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
    tableData.value = res.data?.data ?? []
    total.value = res.data?.total ?? 0
  } catch (e: unknown) {
    const msg = (e as { response?: { data?: { message?: string } } })?.response?.data?.message || t('events.failedToLoadEvents')
    ElMessage.error(msg)
  } finally {
    loading.value = false
  }
}

async function fetchSourceOptions() {
  try {
    const res = await listSources({ page: 1, size: 200 })
    sourceOptions.value = (res.data?.data ?? []).map(
      (s: Record<string, unknown>) => ({ id: String(s.id ?? ''), name: String(s.name ?? '') })
    )
  } catch {
    // silent
  }
}

async function openDetail(row: EventData) {
  drawerVisible.value = true
  detailLoading.value = true
  detail.value = null
  latestRun.value = null
  relatedDeliveries.value = []
  try {
    const res = await getEvent(row.id)
    const data = res.data as Record<string, unknown>

    // Backend GetEvent returns: { event: {...}, latest_run: {...}, parsed_fields: {...} }
    if (data.event) {
      detail.value = data.event as EventData
    } else {
      // Fallback: if the API returns the event directly
      detail.value = data as unknown as EventData
    }

    if (data.latest_run) {
      latestRun.value = data.latest_run as EventRunData
    }

    // Fetch related deliveries
    const delRes = await listDeliveries({ page: 1, size: 50, event_id: row.id })
    relatedDeliveries.value = (delRes.data?.data ?? []) as DeliveryRow[]
  } catch (e: unknown) {
    const msg = (e as { response?: { data?: { message?: string } } })?.response?.data?.message || t('events.failedToLoadDetail')
    ElMessage.error(msg)
  } finally {
    detailLoading.value = false
  }
}

async function handleReplay() {
  if (!detail.value) return
  try {
    await replayEvent(detail.value.id)
    ElMessage.success(t('events.replayInitiated'))
  } catch (e: unknown) {
    const msg = (e as { response?: { data?: { message?: string } } })?.response?.data?.message || t('events.replayFailed')
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

  .monospace-text {
    font-family: 'Courier New', Courier, monospace;
    font-size: 12px;
    word-break: break-all;
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
