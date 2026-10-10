<template>
  <div class="settings-container" v-loading="loading">
    <el-row :gutter="20">
      <el-col :span="24">
        <el-card shadow="never" class="settings-card">
          <template #header>
            <div class="card-header">
              <el-icon><Monitor /></el-icon>
              <span>System Info</span>
            </div>
          </template>
          <el-descriptions :column="2" border>
            <el-descriptions-item label="Version">{{ systemInfo.version || '-' }}</el-descriptions-item>
            <el-descriptions-item label="Uptime">{{ systemInfo.uptime || '-' }}</el-descriptions-item>
            <el-descriptions-item label="Database Path">{{ systemInfo.databasePath || '-' }}</el-descriptions-item>
            <el-descriptions-item label="Status">
              <el-tag :type="systemInfo.healthy ? 'success' : 'danger'" size="small">
                {{ systemInfo.healthy ? 'Healthy' : 'Unhealthy' }}
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
              <span>Retention Settings</span>
            </div>
          </template>
          <el-descriptions :column="1" border>
            <el-descriptions-item label="Event Retention Days">
              {{ retention.eventRetentionDays ?? '-' }}
              <el-tag size="small" type="info" class="env-tag">ENV</el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="Delivery Attempt Retention Days">
              {{ retention.deliveryAttemptRetentionDays ?? '-' }}
              <el-tag size="small" type="info" class="env-tag">ENV</el-tag>
            </el-descriptions-item>
          </el-descriptions>
          <div class="settings-note">
            <el-icon><InfoFilled /></el-icon>
            Retention settings are configured via environment variables and cannot be changed from the UI.
          </div>
        </el-card>
      </el-col>

      <el-col :span="12">
        <el-card shadow="never" class="settings-card">
          <template #header>
            <div class="card-header">
              <el-icon><Cpu /></el-icon>
              <span>Worker Config</span>
            </div>
          </template>
          <el-descriptions :column="1" border>
            <el-descriptions-item label="Processor Concurrency">
              {{ workerConfig.processorConcurrency ?? '-' }}
              <el-tag size="small" type="info" class="env-tag">ENV</el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="Delivery Worker Concurrency">
              {{ workerConfig.deliveryWorkerConcurrency ?? '-' }}
              <el-tag size="small" type="info" class="env-tag">ENV</el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="Lease Timeout">
              {{ workerConfig.leaseTimeout ?? '-' }}
              <el-tag size="small" type="info" class="env-tag">ENV</el-tag>
            </el-descriptions-item>
          </el-descriptions>
          <div class="settings-note">
            <el-icon><InfoFilled /></el-icon>
            Worker configuration is set via environment variables at startup.
          </div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { getHealthz, getVersion } from '@/api/system'

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
    const msg = (e as { response?: { data?: { message?: string } } })?.response?.data?.message || 'Failed to load system info'
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
