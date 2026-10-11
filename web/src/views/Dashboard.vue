<template>
  <div v-loading="loading" class="dashboard-container">
    <el-row :gutter="20" class="stat-row">
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-icon stat-icon--blue">
            <el-icon :size="32"><Bell /></el-icon>
          </div>
          <el-statistic :title="$t('dashboard.totalEvents')" :value="stats.totalEvents" />
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-icon stat-icon--orange">
            <el-icon :size="32"><Loading /></el-icon>
          </div>
          <el-statistic :title="$t('dashboard.processing')" :value="stats.processing" />
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-icon stat-icon--green">
            <el-icon :size="32"><Promotion /></el-icon>
          </div>
          <el-statistic :title="$t('dashboard.pendingDeliveries')" :value="stats.pendingDeliveries" />
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-icon stat-icon--red">
            <el-icon :size="32"><CircleClose /></el-icon>
          </div>
          <el-statistic :title="$t('dashboard.failedDeliveries')" :value="stats.failedDeliveries" />
        </el-card>
      </el-col>
    </el-row>

    <el-card shadow="never" class="errors-card">
      <template #header>
        <div class="card-header">
          <span>{{ $t('dashboard.recentErrors') }}</span>
          <el-button text type="primary" @click="fetchRecentErrors">{{ $t('dashboard.refresh') }}</el-button>
        </div>
      </template>
      <el-table :data="recentErrors" stripe style="width: 100%">
        <el-table-column prop="type" :label="$t('common.type')" width="120">
          <template #default="{ row }">
            <el-tag :type="row.type === 'event' ? 'warning' : 'danger'" size="small">
              {{ $t('enums.errorType.' + row.type, row.type) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="id" :label="$t('common.id')" min-width="180">
          <template #default="{ row }">
            <TruncatedId :id="row.id" />
          </template>
        </el-table-column>
        <el-table-column prop="source" :label="$t('dashboard.source')" min-width="140" />
        <el-table-column prop="status" :label="$t('common.status')" width="120">
          <template #default="{ row }">
            <StatusTag :kind="row.type === 'event' ? 'event' : 'delivery'" :status="row.status" />
          </template>
        </el-table-column>
        <el-table-column :label="$t('common.error')">
          <template #default="{ row }">
            <el-tooltip :content="row.error" placement="top" :show-after="300">
              <span class="cell-ellipsis">{{ row.error }}</span>
            </el-tooltip>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" :label="$t('common.time')" min-width="180">
          <template #default="{ row }">
            {{ formatTime(row.created_at) }}
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { listEvents } from '@/api/events'
import { listDeliveries } from '@/api/deliveries'
import { formatTime } from '@/utils/time'
import { getErrorMessage } from '@/utils/helpers'
import StatusTag from '@/components/StatusTag.vue'
import TruncatedId from '@/components/TruncatedId.vue'
import type { EventResponse, DeliveryResponse } from '@/api/types'

const { t } = useI18n()

interface ErrorRow {
  type: string
  id: string
  source: string
  status: string
  error: string
  created_at: string
}

const loading = ref(false)
const recentErrors = ref<ErrorRow[]>([])

const stats = reactive({
  totalEvents: 0,
  processing: 0,
  pendingDeliveries: 0,
  failedDeliveries: 0,
})

async function fetchStats() {
  try {
    const [totalRes, processingRes, pendingRes, failedRes] = await Promise.all([
      listEvents({ page: 1, size: 1 }),
      listEvents({ page: 1, size: 1, status: 'processing' }),
      listDeliveries({ page: 1, size: 1, status: 'pending' }),
      listDeliveries({ page: 1, size: 1, status: 'failed' }),
    ])
    stats.totalEvents = totalRes.data?.total ?? 0
    stats.processing = processingRes.data?.total ?? 0
    stats.pendingDeliveries = pendingRes.data?.total ?? 0
    stats.failedDeliveries = failedRes.data?.total ?? 0
  } catch (e: unknown) {
    ElMessage.error(getErrorMessage(e, t('common.failedToLoad', { entity: t('nav.dashboard') })))
  }
}

async function fetchRecentErrors() {
  try {
    const [eventsRes, deliveriesRes] = await Promise.all([
      listEvents({ page: 1, size: 5, status: 'failed' }),
      listDeliveries({ page: 1, size: 5, status: 'failed' }),
    ])
    const eventErrors: ErrorRow[] = (eventsRes.data?.data ?? []).map(
      (e: EventResponse) => ({
        type: 'event',
        id: e.id,
        source: e.source_id,
        status: e.status,
        error: e.error_summary ?? e.error_code ?? '',
        created_at: e.received_at ?? '',
      })
    )
    const deliveryErrors: ErrorRow[] = (deliveriesRes.data?.data ?? []).map(
      (d: DeliveryResponse) => ({
        type: 'delivery',
        id: d.id,
        source: d.destination_id,
        status: d.status,
        error: d.last_error_summary ?? d.last_error_code ?? '',
        created_at: d.created_at ?? d.next_attempt_at ?? '',
      })
    )
    const all = [...eventErrors, ...deliveryErrors]
    all.sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
    recentErrors.value = all.slice(0, 10)
  } catch (e: unknown) {
    ElMessage.error(getErrorMessage(e, t('common.failedToLoad', { entity: t('dashboard.recentErrors') })))
  }
}

onMounted(async () => {
  loading.value = true
  try {
    await Promise.all([fetchStats(), fetchRecentErrors()])
  } finally {
    loading.value = false
  }
})
</script>

<style lang="scss" scoped>
.dashboard-container {
  .stat-row {
    margin-bottom: 20px;
  }

  .stat-card {
    :deep(.el-card__body) {
      display: flex;
      align-items: center;
      gap: 16px;
      padding: 20px;
    }
  }

  .stat-icon {
    width: 56px;
    height: 56px;
    border-radius: 12px;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;

    &--blue {
      background-color: #ecf5ff;
      color: #409eff;
    }
    &--orange {
      background-color: #fdf6ec;
      color: #e6a23c;
    }
    &--green {
      background-color: #f0f9eb;
      color: #67c23a;
    }
    &--red {
      background-color: #fef0f0;
      color: #f56c6c;
    }
  }

  .card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-weight: 600;
  }
}
</style>
