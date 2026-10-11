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
        <el-table-column prop="created_at" :label="$t('common.time')" min-width="180">
          <template #default="{ row }">
            {{ formatTime(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column prop="operator" :label="$t('audit.operator')" width="120" />
        <el-table-column prop="action" :label="$t('audit.action')" width="100">
          <template #default="{ row }">
            <StatusTag kind="audit" :status="row.action" />
          </template>
        </el-table-column>
        <el-table-column prop="target_type" :label="$t('audit.targetType')" width="120">
          <template #default="{ row }">
            {{ $t('enums.auditTargetType.' + row.target_type, row.target_type) }}
          </template>
        </el-table-column>
        <el-table-column :label="$t('audit.targetId')" min-width="160">
          <template #default="{ row }">
            <TruncatedId :id="row.target_id" />
          </template>
        </el-table-column>
        <el-table-column :label="$t('audit.summary')">
          <template #default="{ row }">
            <el-tooltip :content="row.summary" placement="top" :show-after="300">
              <span class="cell-ellipsis">{{ row.summary }}</span>
            </el-tooltip>
          </template>
        </el-table-column>
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
import { useI18n } from 'vue-i18n'
import { listAuditLogs } from '@/api/audit'
import { formatTime } from '@/utils/time'
import { getErrorMessage } from '@/utils/helpers'
import { ElMessage } from 'element-plus'
import StatusTag from '@/components/StatusTag.vue'
import TruncatedId from '@/components/TruncatedId.vue'
import type { AuditLogResponse } from '@/api/types'

const { t } = useI18n()

const loading = ref(false)
const tableData = ref<AuditLogResponse[]>([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(20)

const dateRange = ref<[string, string] | null>(null)

const filters = reactive({
  action: '',
  target_type: '',
})

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
    ElMessage.error(getErrorMessage(e, t('audit.failedToLoadAuditLogs')))
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
  // page-filters, pagination-wrap, truncate-id — provided by global.scss
}
</style>
