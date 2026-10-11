<template>
  <div class="deliveries-container">
    <div class="page-filters">
      <el-select v-model="filters.status" :placeholder="$t('common.status')" clearable style="width: 160px" @change="fetchData">
        <el-option :label="$t('deliveries.pending')" value="pending" />
        <el-option :label="$t('deliveries.processing')" value="processing" />
        <el-option :label="$t('deliveries.delivered')" value="delivered" />
        <el-option :label="$t('deliveries.retryWait')" value="retry_wait" />
        <el-option :label="$t('deliveries.failed')" value="failed" />
      </el-select>
      <el-select v-model="filters.destination_id" :placeholder="$t('events.destination')" clearable style="width: 200px" @change="fetchData">
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
        <el-table-column :label="$t('common.id')" width="160">
          <template #default="{ row }">
            <span class="truncate-id">{{ truncateId(row.id) }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('deliveries.eventId')" width="160">
          <template #default="{ row }">
            <span class="truncate-id">{{ truncateId(row.event_id) }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('events.destination')" width="140">
          <template #default="{ row }">
            {{ getDestinationName(row.destination_id) }}
          </template>
        </el-table-column>
        <el-table-column prop="status" :label="$t('common.status')" width="120">
          <template #default="{ row }">
            <el-tag :type="statusTagType(row.status)" size="small">{{ $t('enums.deliveryStatus.' + row.status, row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="attempt_count" :label="$t('events.attempts')" width="90" />
        <el-table-column :label="$t('deliveries.nextAttempt')" width="180">
          <template #default="{ row }">
            {{ formatTime(row.next_attempt_at) }}
          </template>
        </el-table-column>
        <el-table-column :label="$t('common.error')" show-overflow-tooltip>
          <template #default="{ row }">
            {{ row.last_error_summary || row.last_error_code || '-' }}
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
      :title="$t('deliveries.deliveryDetail')"
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
              <el-tag :type="statusTagType(detail.status)" size="small">{{ $t('enums.deliveryStatus.' + detail.status, detail.status) }}</el-tag>
            </el-descriptions-item>
            <el-descriptions-item :label="$t('events.destination')">
              {{ getDestinationName(detail.destination_id) }}
            </el-descriptions-item>
            <el-descriptions-item :label="$t('deliveries.eventId')">
              <span class="monospace-text">{{ truncateId(detail.event_id) }}</span>
            </el-descriptions-item>
            <el-descriptions-item :label="$t('deliveries.ruleId')">
              <span class="monospace-text">{{ truncateId(detail.rule_id) }}</span>
            </el-descriptions-item>
            <el-descriptions-item :label="$t('deliveries.attemptCount')">{{ detail.attempt_count ?? 0 }}</el-descriptions-item>
            <el-descriptions-item :label="$t('deliveries.maxAttempts')">{{ detail.max_attempts ?? '-' }}</el-descriptions-item>
            <el-descriptions-item :label="$t('deliveries.nextAttempt')">{{ formatTime(detail.next_attempt_at) }}</el-descriptions-item>
            <el-descriptions-item :label="$t('deliveries.createdAt')">{{ formatTime(detail.created_at) }}</el-descriptions-item>
            <el-descriptions-item v-if="detail.last_error_summary" :label="$t('deliveries.lastError')" :span="2">
              <el-text type="danger">{{ detail.last_error_code }}: {{ detail.last_error_summary }}</el-text>
            </el-descriptions-item>
          </el-descriptions>

          <el-divider content-position="left">{{ $t('deliveries.renderedTitle') }}</el-divider>
          <div class="rendered-output">{{ detail.rendered_title || '-' }}</div>

          <el-divider content-position="left">{{ $t('deliveries.renderedBody') }}</el-divider>
          <div v-if="renderedBodyHtml" class="rendered-output rendered-body-rich" v-html="renderedBodyHtml" />
          <pre v-else class="rendered-output rendered-body">{{ detail.rendered_body || '-' }}</pre>

          <el-divider content-position="left">{{ $t('deliveries.deliveryAttempts') }}</el-divider>
          <el-table v-if="deliveryAttempts.length" :data="deliveryAttempts" size="small" border>
            <el-table-column prop="attempt_no" label="#" width="60" />
            <el-table-column :label="$t('deliveries.result')" width="80">
              <template #default="{ row }">
                <el-tag :type="row.error_code ? 'danger' : 'success'" size="small">
                  {{ row.error_code ? $t('deliveries.failed') : $t('deliveries.ok') }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="response_status" :label="$t('deliveries.http')" width="80" />
            <el-table-column :label="$t('deliveries.duration')" width="110">
              <template #default="{ row }">
                {{ row.duration_ms != null ? `${row.duration_ms}ms` : '-' }}
              </template>
            </el-table-column>
            <el-table-column :label="$t('common.time')" width="180">
              <template #default="{ row }">
                {{ formatTime(row.started_at) }}
              </template>
            </el-table-column>
            <el-table-column type="expand">
              <template #default="{ row }">
                <div class="attempt-detail">
                  <div v-if="row.error_code" class="attempt-section">
                    <div class="attempt-label">{{ $t('common.error') }}</div>
                    <el-text type="danger">{{ row.error_code }}: {{ row.error_summary }}</el-text>
                  </div>
                  <div v-if="row.response_status" class="attempt-section">
                    <div class="attempt-label">{{ $t('deliveries.httpStatus') }}</div>
                    <span>{{ row.response_status }}</span>
                  </div>
                  <div v-if="row.response_summary" class="attempt-section">
                    <div class="attempt-label">{{ $t('deliveries.responseBody') }}</div>
                    <pre class="attempt-response-body">{{ formatResponseBody(row.response_summary) }}</pre>
                  </div>
                </div>
              </template>
            </el-table-column>
          </el-table>
          <el-empty v-else :description="$t('deliveries.noAttempts')" :image-size="40" />

          <div class="drawer-actions" v-if="canRetry">
            <el-button type="warning" @click="handleRetry">
              <el-icon><RefreshRight /></el-icon> {{ $t('deliveries.retry') }}
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
import { marked } from 'marked'
import { RefreshRight } from '@element-plus/icons-vue'
import { listDeliveries, getDelivery, retryDelivery } from '@/api/deliveries'
import { listDestinations } from '@/api/destinations'

const { t } = useI18n()

interface DeliveryData {
  id: string
  event_id: string
  event_run_id: string
  rule_id: string
  destination_id: string
  status: string
  attempt_count: number
  max_attempts: number
  next_attempt_at: string
  rendered_title: string | null
  rendered_body: string | null
  last_error_code: string | null
  last_error_summary: string | null
  created_at: string
  updated_at: string
  [key: string]: unknown
}

interface DeliveryAttemptData {
  id: string
  attempt_no: number
  started_at: string
  finished_at: string | null
  response_status: number | null
  response_summary: string | null
  error_code: string | null
  error_summary: string | null
  duration_ms: number | null
}

const loading = ref(false)
const detailLoading = ref(false)
const drawerVisible = ref(false)

const detail = ref<DeliveryData | null>(null)
const deliveryAttempts = ref<DeliveryAttemptData[]>([])

const tableData = ref<DeliveryData[]>([])
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

function formatTime(val: string | null | undefined): string {
  if (!val) return '-'
  try {
    return new Date(val).toLocaleString()
  } catch {
    return val
  }
}

function getDestinationName(destId: string): string {
  if (!destId) return '-'
  return destinationOptions.value.find((d) => d.id === destId)?.name ?? truncateId(destId)
}

function statusTagType(status: string): 'success' | 'warning' | 'danger' | 'info' {
  const map: Record<string, 'success' | 'warning' | 'danger' | 'info'> = {
    pending: 'info',
    processing: 'warning',
    delivered: 'success',
    retry_wait: 'warning',
    failed: 'danger',
    dead_letter: 'danger',
  }
  return map[status] ?? 'info'
}

const canRetry = computed(() => {
  if (!detail.value) return false
  return detail.value.status === 'failed' || detail.value.status === 'dead_letter' || detail.value.status === 'retry_wait'
})

function formatResponseBody(body: string | null | undefined): string {
  if (!body) return '-'
  try {
    const parsed = JSON.parse(body)
    return JSON.stringify(parsed, null, 2)
  } catch {
    return body
  }
}

// Render body content based on format (text/markdown/html)
const renderedBodyHtml = computed(() => {
  if (!detail.value) return ''
  const body = detail.value.rendered_body ?? ''
  if (!body) return ''
  const format = (detail.value as Record<string, unknown>).rendered_format as string ?? 'text'
  if (format === 'html') return body
  if (format === 'markdown') return marked(body) as string
  return ''
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
    tableData.value = res.data?.data ?? []
    total.value = res.data?.total ?? 0
  } catch (e: unknown) {
    const msg = (e as { response?: { data?: { message?: string } } })?.response?.data?.message || t('deliveries.failedToLoadDeliveries')
    ElMessage.error(msg)
  } finally {
    loading.value = false
  }
}

async function fetchDestinationOptions() {
  try {
    const res = await listDestinations({ page: 1, size: 200 })
    destinationOptions.value = (res.data?.data ?? []).map(
      (d: Record<string, unknown>) => ({ id: String(d.id ?? ''), name: String(d.name ?? '') })
    )
  } catch {
    // silent
  }
}

async function openDetail(row: DeliveryData) {
  drawerVisible.value = true
  detailLoading.value = true
  detail.value = null
  deliveryAttempts.value = []
  try {
    const res = await getDelivery(row.id)
    const data = res.data as Record<string, unknown>

    // Backend GetDelivery returns: { delivery: {...}, delivery_attempts: [...] }
    if (data.delivery) {
      detail.value = data.delivery as DeliveryData
    } else {
      detail.value = data as unknown as DeliveryData
    }

    if (data.delivery_attempts && Array.isArray(data.delivery_attempts)) {
      deliveryAttempts.value = data.delivery_attempts as DeliveryAttemptData[]
    }
  } catch (e: unknown) {
    const msg = (e as { response?: { data?: { message?: string } } })?.response?.data?.message || t('deliveries.failedToLoadDetail')
    ElMessage.error(msg)
  } finally {
    detailLoading.value = false
  }
}

async function handleRetry() {
  if (!detail.value) return
  try {
    await retryDelivery(detail.value.id)
    ElMessage.success(t('deliveries.retryInitiated'))
    // Refresh detail by re-fetching the same row
    const row = { ...detail.value } as DeliveryData
    await openDetail(row)
  } catch (e: unknown) {
    const msg = (e as { response?: { data?: { message?: string } } })?.response?.data?.message || t('deliveries.retryFailed')
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

  .monospace-text {
    font-family: 'Courier New', Courier, monospace;
    font-size: 12px;
    word-break: break-all;
  }

  .detail-desc {
    margin-bottom: 12px;
  }

  .rendered-output {
    background-color: var(--el-fill-color-lighter, #f5f7fa);
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

  .rendered-body-rich {
    line-height: 1.6;

    :deep(h1), :deep(h2), :deep(h3), :deep(h4), :deep(h5), :deep(h6) {
      margin-top: 8px;
      margin-bottom: 4px;
    }

    :deep(p) {
      margin: 4px 0;
    }

    :deep(code) {
      background-color: rgba(0, 0, 0, 0.06);
      padding: 2px 6px;
      border-radius: 3px;
      font-family: 'Courier New', Courier, monospace;
      font-size: 13px;
    }

    :deep(pre) {
      background-color: rgba(0, 0, 0, 0.04);
      padding: 8px 12px;
      border-radius: 4px;
      overflow-x: auto;

      code {
        background: none;
        padding: 0;
      }
    }

    :deep(blockquote) {
      border-left: 4px solid #d0d0d0;
      padding-left: 12px;
      margin: 8px 0;
      color: #666;
    }

    :deep(ul), :deep(ol) {
      padding-left: 20px;
    }

    :deep(table) {
      border-collapse: collapse;
      margin: 8px 0;

      th, td {
        border: 1px solid #ddd;
        padding: 6px 12px;
      }

      th {
        background-color: #f0f0f0;
      }
    }

    :deep(img) {
      max-width: 100%;
    }

    :deep(a) {
      color: #409eff;
    }
  }

  .drawer-actions {
    margin-top: 20px;
    display: flex;
    justify-content: flex-end;
  }

  .attempt-detail {
    padding: 8px 16px;
  }

  .attempt-section {
    margin-bottom: 8px;
  }

  .attempt-label {
    font-weight: 600;
    font-size: 12px;
    color: var(--el-text-color-secondary, #909399);
    margin-bottom: 4px;
  }

  .attempt-response-body {
    background-color: var(--el-fill-color-lighter, #f5f7fa);
    padding: 8px 12px;
    border-radius: 4px;
    font-family: 'Courier New', Courier, monospace;
    font-size: 12px;
    white-space: pre-wrap;
    word-break: break-all;
    margin: 0;
    max-height: 300px;
    overflow-y: auto;
  }
}
</style>
