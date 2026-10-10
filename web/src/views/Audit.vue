<template>
  <div class="audit-container">
    <div class="page-filters">
      <el-select v-model="filters.action" placeholder="Action" clearable style="width: 160px" @change="fetchData">
        <el-option label="Create" value="create" />
        <el-option label="Update" value="update" />
        <el-option label="Delete" value="delete" />
        <el-option label="Login" value="login" />
        <el-option label="Replay" value="replay" />
        <el-option label="Retry" value="retry" />
      </el-select>
      <el-select v-model="filters.target_type" placeholder="Target Type" clearable style="width: 160px" @change="fetchData">
        <el-option label="Source" value="source" />
        <el-option label="Mapping" value="mapping" />
        <el-option label="Rule" value="rule" />
        <el-option label="Template" value="template" />
        <el-option label="Destination" value="destination" />
        <el-option label="Event" value="event" />
        <el-option label="Delivery" value="delivery" />
      </el-select>
      <el-date-picker
        v-model="dateRange"
        type="daterange"
        range-separator="to"
        start-placeholder="Start date"
        end-placeholder="End date"
        format="YYYY-MM-DD"
        value-format="YYYY-MM-DD"
        style="width: 280px"
        @change="fetchData"
      />
    </div>

    <el-card shadow="never" v-loading="loading">
      <el-table :data="tableData" stripe style="width: 100%">
        <el-table-column prop="created_at" label="Time" width="180">
          <template #default="{ row }">
            {{ formatTime(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column prop="operator" label="Operator" width="120" />
        <el-table-column prop="action" label="Action" width="100">
          <template #default="{ row }">
            <el-tag :type="actionTagType(row.action)" size="small">{{ row.action }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="target_type" label="Target Type" width="120" />
        <el-table-column label="Target ID" width="160">
          <template #default="{ row }">
            <span class="truncate-id">{{ truncateId(row.target_id) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="summary" label="Summary" show-overflow-tooltip />
      </el-table>

      <div class="pagination-wrap" v-if="total > 0">
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          :total="total"
          :page-sizes="[10, 20, 50, 100]"
          layout="total, sizes, prev, pager, next"
          @change="fetchData"
        />
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { listAuditLogs } from '@/api/audit'

interface AuditRow {
  id: string
  created_at: string
  operator: string
  action: string
  target_type: string
  target_id: string
  summary: string
}

const loading = ref(false)
const tableData = ref<AuditRow[]>([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(20)

const dateRange = ref<[string, string] | null>(null)

const filters = reactive({
  action: '',
  target_type: '',
})

function truncateId(id: string): string {
  if (!id) return '-'
  return id.length > 12 ? id.substring(0, 12) + '...' : id
}

function formatTime(val: string): string {
  if (!val) return '-'
  return new Date(val).toLocaleString()
}

function actionTagType(action: string): 'success' | 'warning' | 'danger' | 'info' {
  const map: Record<string, 'success' | 'warning' | 'danger' | 'info'> = {
    create: 'success',
    update: 'warning',
    delete: 'danger',
    login: 'info',
    replay: 'warning',
    retry: 'warning',
  }
  return map[action] ?? 'info'
}

async function fetchData() {
  loading.value = true
  try {
    const params: Record<string, unknown> = {
      page: currentPage.value,
      size: pageSize.value,
    }
    if (filters.action) params.action = filters.action
    if (filters.target_type) params.target_type = filters.target_type
    if (dateRange.value && dateRange.value[0]) {
      params.start_time = dateRange.value[0]
      params.end_time = dateRange.value[1]
    }
    const res = await listAuditLogs(params)
    tableData.value = res.data?.items ?? []
    total.value = res.data?.total ?? 0
  } catch (e: unknown) {
    const msg = (e as { response?: { data?: { message?: string } } })?.response?.data?.message || 'Failed to load audit logs'
    ElMessage.error(msg)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchData()
})
</script>

<style lang="scss" scoped>
.audit-container {
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
  }
}
</style>
