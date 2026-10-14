<template>
  <div class="events-container">
    <div class="page-filters">
      <el-select v-model="filters.status" :placeholder="$t('common.status')" clearable style="width: 160px" @change="fetchData">
        <el-option :label="$t('events.received')" value="received" />
        <el-option :label="$t('events.processing')" value="processing" />
        <el-option :label="$t('events.completed')" value="completed" />
        <el-option :label="$t('events.deduped')" value="deduped" />
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
        <el-table-column :label="$t('common.id')" min-width="220">
          <template #default="{ row }">
            <TruncatedId :id="row.id" />
          </template>
        </el-table-column>
        <el-table-column :label="$t('events.source')" min-width="140">
          <template #default="{ row }">
            {{ lookupName(sourceOptions, row.source_id) }}
          </template>
        </el-table-column>
        <el-table-column prop="status" :label="$t('common.status')" min-width="120">
          <template #default="{ row }">
            <StatusTag kind="event" :status="row.status" />
          </template>
        </el-table-column>
        <el-table-column :label="$t('events.receivedAt')" min-width="180">
          <template #default="{ row }">
            {{ formatTime(row.received_at) }}
          </template>
        </el-table-column>
        <el-table-column :label="$t('common.error')">
          <template #default="{ row }">
            <el-tooltip :content="row.error_summary || row.error_code || '-'" placement="top" :show-after="300">
              <span class="cell-ellipsis">{{ row.error_summary || row.error_code || '-' }}</span>
            </el-tooltip>
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
      :size="drawerWidth"
      destroy-on-close
    >
      <div class="drawer-resize-handle" @mousedown="onHandleMouseDown" />
      <div v-loading="detailLoading">
        <div v-if="detail">
          <el-descriptions :column="2" border class="detail-desc">
            <el-descriptions-item :label="$t('common.id')" :span="2">
              {{ detail.id }}
            </el-descriptions-item>
            <el-descriptions-item :label="$t('common.status')">
              <StatusTag kind="event" :status="detail.status" />
            </el-descriptions-item>
            <el-descriptions-item :label="$t('events.source')">
              {{ lookupName(sourceOptions, detail.source_id) }}
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
          />

          <el-divider content-position="left">{{ $t('events.parsedFields') }}</el-divider>
          <el-table v-if="parsedFields.length" :data="parsedFields" border>
            <el-table-column prop="key" :label="$t('events.key')" min-width="120" />
            <el-table-column :label="$t('events.value')" min-width="200">
              <template #default="{ row }">
                <el-tooltip :content="row.value" placement="top" :show-after="300">
                  <span class="cell-ellipsis">{{ row.value }}</span>
                </el-tooltip>
                <el-icon class="copy-icon-hover" :size="12" @click.stop="copyText(row.value)"><CopyDocument /></el-icon>
              </template>
            </el-table-column>
          </el-table>
          <el-empty v-else :description="$t('events.noParsedFields')" :image-size="40" />

          <el-divider content-position="left">{{ $t('events.eventRun') }}</el-divider>
          <div v-if="latestRun">
            <el-descriptions :column="2" border>
              <el-descriptions-item :label="$t('events.runType')">{{ latestRun.run_type }}</el-descriptions-item>
              <el-descriptions-item :label="$t('common.status')">
                <StatusTag kind="event" :status="latestRun.status" />
              </el-descriptions-item>
              <el-descriptions-item :label="$t('events.startedAt')">{{ formatTime(latestRun.started_at) }}</el-descriptions-item>
              <el-descriptions-item :label="$t('events.finishedAt')">{{ latestRun.finished_at ? formatTime(latestRun.finished_at) : '-' }}</el-descriptions-item>
            </el-descriptions>

            <div v-if="latestRun.error_summary" style="margin-top: 8px">
              <el-text type="danger">{{ latestRun.error_code }}: {{ latestRun.error_summary }}</el-text>
            </div>
          </div>
          <el-empty v-else :description="$t('events.noEventRun')" :image-size="40" />

          <el-divider content-position="left">{{ $t('events.matchedRules') }}</el-divider>
          <el-table v-if="matchedRules.length" :data="matchedRules" border>
            <el-table-column prop="name" :label="$t('events.rule')" min-width="140" />
            <el-table-column prop="priority" :label="$t('rules.priority')" min-width="90" />
            <el-table-column :label="$t('rules.destinations')" min-width="160">
              <template #default="{ row }">
                <el-tag v-for="dId in (row.destinations ?? []).slice(0, 2)" :key="dId" style="margin-right: 4px">
                  {{ lookupName(destinationOptions, dId) }}
                </el-tag>
                <el-tag v-if="(row.destinations ?? []).length > 2" type="info">
                  +{{ row.destinations.length - 2 }}
                </el-tag>
              </template>
            </el-table-column>
          </el-table>
          <el-empty v-else :description="$t('events.noMatchedRules')" :image-size="40" />

          <el-divider content-position="left">{{ $t('events.relatedDeliveries') }}</el-divider>
          <el-table v-if="relatedDeliveries.length" :data="relatedDeliveries" border>
            <el-table-column :label="$t('common.id')" min-width="220">
              <template #default="{ row }">
                <TruncatedId :id="row.id" />
              </template>
            </el-table-column>
            <el-table-column :label="$t('events.destination')" min-width="160">
              <template #default="{ row }">
                <el-tag>{{ lookupName(destinationOptions, row.destination_id) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="status" :label="$t('common.status')" min-width="110">
              <template #default="{ row }">
                <StatusTag kind="delivery" :status="row.status" />
              </template>
            </el-table-column>
            <el-table-column prop="attempt_count" :label="$t('events.attempts')" min-width="90" />
            <el-table-column :label="$t('events.createdAt')" min-width="160">
              <template #default="{ row }">
                {{ formatTime(row.created_at) }}
              </template>
            </el-table-column>
            <el-table-column :label="$t('events.updatedAt')" min-width="160">
              <template #default="{ row }">
                {{ formatTime(row.updated_at) }}
              </template>
            </el-table-column>
          </el-table>
          <el-empty v-else :description="$t('events.noDeliveries')" :image-size="40" />

          <div class="drawer-actions">
            <el-button type="warning" @click="handleReplay">
              <el-icon><RefreshRight /></el-icon> {{ $t('events.replay') }}
            </el-button>
          </div>
        </div>
      </div>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { RefreshRight, CopyDocument } from '@element-plus/icons-vue'
import { listEvents, getEvent, replayEvent } from '@/api/events'
import { listDeliveries } from '@/api/deliveries'
import { formatTime } from '@/utils/time'
import { getErrorMessage, copyToClipboard } from '@/utils/helpers'
import { lookupName } from '@/utils/lookup'
import { useOptions } from '@/composables/useOptions'
import { useResizableDrawer } from '@/composables/useResizableDrawer'
import TruncatedId from '@/components/TruncatedId.vue'
import StatusTag from '@/components/StatusTag.vue'
import type { EventResponse, EventRunResponse, RuleSnapshot } from '@/api/types'

const { t } = useI18n()

const { drawerWidth, onHandleMouseDown } = useResizableDrawer(800)

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
}

const loading = ref(false)
const detailLoading = ref(false)
const drawerVisible = ref(false)

// Detail state
const detail = ref<EventResponse | null>(null)
const latestRun = ref<EventRunResponse | null>(null)
const relatedDeliveries = ref<DeliveryRow[]>([])

const tableData = ref<EventResponse[]>([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(20)

const { sourceOptions, destinationOptions, fetchSourceOptions, fetchDestinationOptions } = useOptions()

const filters = reactive({
  status: '',
  source_id: '',
})

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
    ElMessage.error(getErrorMessage(e, t('events.failedToLoadEvents')))
  } finally {
    loading.value = false
  }
}

async function openDetail(row: EventResponse) {
  drawerVisible.value = true
  detailLoading.value = true
  detail.value = null
  latestRun.value = null
  relatedDeliveries.value = []
  try {
    const res = await getEvent(row.id)
    const data = res.data as unknown as Record<string, unknown>

    // Backend GetEvent returns: { event: {...}, latest_run: {...} }
    if (data.event) {
      detail.value = data.event as EventResponse
    } else {
      // Fallback: if the API returns the event directly
      detail.value = data as unknown as EventResponse
    }

    if (data.latest_run) {
      latestRun.value = data.latest_run as EventRunResponse
    }

    // Fetch related deliveries
    const delRes = await listDeliveries({ page: 1, size: 50, event_id: row.id })
    relatedDeliveries.value = (delRes.data?.data ?? []) as DeliveryRow[]
  } catch (e: unknown) {
    ElMessage.error(getErrorMessage(e, t('events.failedToLoadDetail')))
  } finally {
    detailLoading.value = false
  }
}

async function copyText(text: string) {
  try {
    await copyToClipboard(text)
    ElMessage.success(t('common.copied'))
  } catch {
    ElMessage.error(t('common.copyFailed'))
  }
}

async function handleReplay() {
  if (!detail.value) return
  try {
    await replayEvent(detail.value.id)
    ElMessage.success(t('events.replayInitiated'))
  } catch (e: unknown) {
    ElMessage.error(getErrorMessage(e, t('events.replayFailed')))
  }
}

onMounted(() => {
  fetchData()
  fetchSourceOptions()
  fetchDestinationOptions()
})
</script>

<style lang="scss" scoped>
.events-container {
  .detail-desc {
    margin-bottom: 12px;
  }

  .drawer-actions {
    margin-top: 20px;
    display: flex;
    justify-content: flex-end;
  }

  .drawer-resize-handle {
    position: absolute;
    left: 0;
    top: 0;
    bottom: 0;
    width: 4px;
    cursor: col-resize;
    z-index: 10;

    &:hover,
    &:active {
      background-color: var(--el-color-primary, #409eff);
    }
  }

  .copy-icon-hover {
    flex-shrink: 0;
    margin-left: 4px;
    color: var(--el-text-color-placeholder, #a8abb2);
    cursor: pointer;
    opacity: 0;
    transition: color 0.2s, opacity 0.2s;
  }

  // Show copy icon on hover of the row cell
  .el-table__row:hover .copy-icon-hover {
    opacity: 1;
  }

  .copy-icon-hover:hover {
    color: var(--el-color-primary, #409eff);
  }
}
</style>
