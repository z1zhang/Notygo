<template>
  <div class="settings-container" v-loading="loading">
    <el-row :gutter="20">
      <el-col :span="24">
        <el-card shadow="never" class="settings-card">
          <template #header>
            <div class="card-header">
              <el-icon><Monitor /></el-icon>
              <span>{{ $t('settings.systemInfo') }}</span>
            </div>
          </template>
          <el-descriptions :column="2" border>
            <el-descriptions-item :label="$t('settings.version')">{{ systemInfo.version || '-' }}</el-descriptions-item>
            <el-descriptions-item :label="$t('settings.uptime')">{{ systemInfo.uptime || '-' }}</el-descriptions-item>
            <el-descriptions-item :label="$t('settings.databasePath')">{{ systemInfo.databasePath || '-' }}</el-descriptions-item>
            <el-descriptions-item :label="$t('settings.status')">
              <el-tag :type="systemInfo.healthy ? 'success' : 'danger'" size="small">
                {{ systemInfo.healthy ? $t('settings.healthy') : $t('settings.unhealthy') }}
              </el-tag>
            </el-descriptions-item>
          </el-descriptions>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="20" class="settings-row">
      <el-col :span="12">
        <el-card shadow="never" class="settings-card">
          <template #header>
            <div class="card-header">
              <el-icon><Calendar /></el-icon>
              <span>{{ $t('settings.retentionSettings') }}</span>
            </div>
          </template>
          <el-descriptions :column="1" border>
            <el-descriptions-item :label="$t('settings.eventRetentionDays')">
              {{ retention.eventRetentionDays ?? '-' }}
              <el-tag size="small" type="info" class="env-tag">{{ $t('settings.env') }}</el-tag>
            </el-descriptions-item>
            <el-descriptions-item :label="$t('settings.deliveryAttemptRetentionDays')">
              {{ retention.deliveryAttemptRetentionDays ?? '-' }}
              <el-tag size="small" type="info" class="env-tag">{{ $t('settings.env') }}</el-tag>
            </el-descriptions-item>
          </el-descriptions>
          <div class="settings-note">
            <el-icon><InfoFilled /></el-icon>
            {{ $t('settings.retentionNote') }}
          </div>
        </el-card>
      </el-col>

      <el-col :span="12">
        <el-card shadow="never" class="settings-card">
          <template #header>
            <div class="card-header">
              <el-icon><Cpu /></el-icon>
              <span>{{ $t('settings.workerConfig') }}</span>
            </div>
          </template>
          <el-descriptions :column="1" border>
            <el-descriptions-item :label="$t('settings.processorConcurrency')">
              {{ workerConfig.processorConcurrency ?? '-' }}
              <el-tag size="small" type="info" class="env-tag">{{ $t('settings.env') }}</el-tag>
            </el-descriptions-item>
            <el-descriptions-item :label="$t('settings.deliveryWorkerConcurrency')">
              {{ workerConfig.deliveryWorkerConcurrency ?? '-' }}
              <el-tag size="small" type="info" class="env-tag">{{ $t('settings.env') }}</el-tag>
            </el-descriptions-item>
            <el-descriptions-item :label="$t('settings.leaseTimeout')">
              {{ workerConfig.leaseTimeout ?? '-' }}
              <el-tag size="small" type="info" class="env-tag">{{ $t('settings.env') }}</el-tag>
            </el-descriptions-item>
          </el-descriptions>
          <div class="settings-note">
            <el-icon><InfoFilled /></el-icon>
            {{ $t('settings.workerNote') }}
          </div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { getHealthz, getVersion } from '@/api/system'

const { t } = useI18n()

const loading = ref(false)

const systemInfo = reactive({
  version: '',
  uptime: '',
  databasePath: '',
  healthy: false,
})

const retention = reactive({
  eventRetentionDays: 0,
  deliveryAttemptRetentionDays: 0,
})

const workerConfig = reactive({
  processorConcurrency: 0,
  deliveryWorkerConcurrency: 0,
  leaseTimeout: '',
})

async function fetchSystemInfo() {
  loading.value = true
  try {
    const [healthRes, versionRes] = await Promise.allSettled([
      getHealthz(),
      getVersion(),
    ])

    if (healthRes.status === 'fulfilled') {
      const data = healthRes.value.data
      systemInfo.healthy = data?.status === 'ok' || data?.healthy === true
      systemInfo.uptime = data?.uptime ?? ''
      systemInfo.databasePath = data?.database_path ?? data?.db_path ?? ''
    } else {
      systemInfo.healthy = false
    }

    if (versionRes.status === 'fulfilled') {
      const data = versionRes.value.data
      systemInfo.version = data?.version ?? ''
      // Parse config info from version endpoint if available
      retention.eventRetentionDays = data?.event_retention_days ?? data?.config?.event_retention_days ?? 0
      retention.deliveryAttemptRetentionDays = data?.delivery_attempt_retention_days ?? data?.config?.delivery_attempt_retention_days ?? 0
      workerConfig.processorConcurrency = data?.processor_concurrency ?? data?.config?.processor_concurrency ?? 0
      workerConfig.deliveryWorkerConcurrency = data?.delivery_worker_concurrency ?? data?.config?.delivery_worker_concurrency ?? 0
      workerConfig.leaseTimeout = data?.lease_timeout ?? data?.config?.lease_timeout ?? ''
    }
  } catch (e: unknown) {
    const msg = (e as { response?: { data?: { message?: string } } })?.response?.data?.message || t('settings.failedToLoadSystemInfo')
    ElMessage.error(msg)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchSystemInfo()
})
</script>

<style lang="scss" scoped>
.settings-container {
  .settings-row {
    margin-top: 20px;
  }

  .settings-card {
    .card-header {
      display: flex;
      align-items: center;
      gap: 8px;
      font-weight: 600;
      font-size: 16px;
    }
  }

  .env-tag {
    margin-left: 8px;
  }

  .settings-note {
    margin-top: 12px;
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    color: #909399;
  }
}
</style>
