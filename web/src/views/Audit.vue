<template>
  <div class="audit-container">
    <div class="page-filters">
      <el-select v-model="filters.action" :placeholder="$t('audit.action')" clearable style="width: 160px" @change="fetchData">
        <el-option :label="$t('audit.create')" value="create" />
        <el-option :label="$t('audit.update')" value="update" />
        <el-option :label="$t('audit.delete')" value="delete" />
        <el-option :label="$t('audit.login')" value="login" />
        <el-option :label="$t('audit.replay')" value="replay" />
        <el-option :label="$t('audit.retry')" value="retry" />
      </el-select>
      <el-select v-model="filters.target_type" :placeholder="$t('audit.targetType')" clearable style="width: 160px" @change="fetchData">
        <el-option :label="$t('nav.sources')" value="source" />
        <el-option :label="$t('nav.mappings')" value="mapping" />
        <el-option :label="$t('nav.rules')" value="rule" />
        <el-option :label="$t('nav.templates')" value="template" />
        <el-option :label="$t('nav.destinations')" value="destination" />
        <el-option :label="$t('nav.events')" value="event" />
        <el-option :label="$t('nav.deliveries')" value="delivery" />
      </el-select>
      <el-date-picker
        v-model="dateRange"
        type="daterange"
        :range-separator="$t('audit.to')"
        :start-placeholder="$t('audit.startDate')"
        :end-placeholder="$t('audit.endDate')"
        format="YYYY-MM-DD"
        value-format="YYYY-MM-DD"
        style="width: 280px"
        @change="fetchData"
      />
    </div>

    <el-card shadow="never" v-loading="loading">
      <el-table :data="tableData" stripe style="width: 100%">
        <el-table-column prop="created_at" :label="$t('common.time')" width="180">
          <template #default="{ row }">
            {{ formatTime(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column prop="operator" :label="$t('audit.operator')" width="120" />
        <el-table-column prop="action" :label="$t('audit.action')" width="100">
          <template #default="{ row }">
            <el-tag :type="actionTagType(row.action)" size="small">{{ $t('audit.' + row.action, row.action) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="target_type" :label="$t('audit.targetType')" width="120">
          <template #default="{ row }">
            {{ $t('enums.auditTargetType.' + row.target_type, row.target_type) }}
          </template>
        </el-table-column>
        <el-table-column :label="$t('audit.targetId')" width="160">
          <template #default="{ row }">
            <span class="truncate-id">{{ truncateId(row.target_id) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="summary" :label="$t('audit.summary')" show-overflow-tooltip />
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
import { useI18n } from 'vue-i18n'
import { listAuditLogs } from '@/api/audit'

const { t } = useI18n()

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
    tableData.value = res.data?.data ?? []
    total.value = res.data?.total ?? 0
  } catch (e: unknown) {
    const msg = (e as { response?: { data?: { message?: string } } })?.response?.data?.message || t('audit.failedToLoadAuditLogs')
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
