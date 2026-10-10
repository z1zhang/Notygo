<template>
  <div class="destinations-container">
    <div class="page-actions">
      <el-button type="primary" @click="openDialog()">
        <el-icon><Plus /></el-icon> New Destination
      </el-button>
    </div>

    <el-card shadow="never" v-loading="loading">
      <el-table :data="tableData" stripe style="width: 100%">
        <el-table-column prop="name" label="Name" min-width="160" />
        <el-table-column prop="type" label="Type" width="140">
          <template #default="{ row }">
            <el-tag size="small">{{ row.type }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="enabled" label="Enabled" width="100">
          <template #default="{ row }">
            <el-switch v-model="row.enabled" @change="(val: boolean) => toggleEnabled(row, val)" />
          </template>
        </el-table-column>
        <el-table-column label="Actions" width="160" fixed="right">
          <template #default="{ row }">
            <el-button text type="primary" size="small" @click="openDialog(row)">Edit</el-button>
            <el-button text type="danger" size="small" @click="handleDelete(row)">Delete</el-button>
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

    <el-dialog
      v-model="dialogVisible"
      :title="isEditing ? 'Edit Destination' : 'New Destination'"
      width="600px"
      destroy-on-close
    >
      <el-form
        ref="formRef"
        :model="form"
        :rules="formRules"
        label-width="150px"
        label-position="right"
      >
        <el-form-item label="Name" prop="name">
          <el-input v-model="form.name" placeholder="Destination name" />
        </el-form-item>
        <el-form-item label="Type" prop="type">
          <el-select v-model="form.type" placeholder="Select type">
            <el-option label="DingTalk" value="dingtalk" />
            <el-option label="Feishu (Lark)" value="feishu" />
            <el-option label="WeCom" value="wecom" />
            <el-option label="Email" value="email" />
            <el-option label="HTTP Webhook" value="http_webhook" />
          </el-select>
        </el-form-item>
        <el-alert
          v-if="typeHelpText"
          :title="typeHelpText"
          type="info"
          :closable="false"
          show-icon
          class="type-help"
        />
        <el-form-item label="Config JSON" prop="config">
          <el-input
            v-model="form.config"
            type="textarea"
            :rows="5"
            placeholder="Destination configuration as JSON"
            class="monospace-input"
          />
        </el-form-item>
        <el-form-item label="Secret" prop="secret">
          <el-input
            v-model="form.secret"
            type="password"
            show-password
            placeholder="API secret or token"
          />
        </el-form-item>
        <el-form-item label="Timeout (ms)" prop="timeout_ms">
          <el-input-number v-model="form.timeout_ms" :min="1000" :step="1000" />
        </el-form-item>
        <el-form-item label="Circuit Breaker" prop="circuit_breaker">
          <el-input
            v-model="form.circuit_breaker"
            type="textarea"
            :rows="3"
            placeholder='{"failure_threshold":5,"success_threshold":2,"timeout_seconds":60}'
            class="monospace-input"
          />
        </el-form-item>
        <el-form-item label="Enabled">
          <el-switch v-model="form.enabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">Cancel</el-button>
        <el-button type="primary" :loading="saving" @click="handleSave">Save</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import type { FormInstance, FormRules } from 'element-plus'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  listDestinations,
  createDestination,
  updateDestination,
  deleteDestination,
} from '@/api/destinations'

interface DestinationRow {
  id: string
  name: string
  type: string
  enabled: boolean
  [key: string]: unknown
}

const typeHelpMap: Record<string, string> = {
  dingtalk: 'DingTalk robot webhook. Config: { "webhook_url": "https://...", "msg_type": "markdown" }',
  feishu: 'Feishu/Lark bot webhook. Config: { "webhook_url": "https://...", "msg_type": "interactive" }',
  wecom: 'WeCom robot webhook. Config: { "webhook_url": "https://...", "msg_type": "markdown" }',
  email: 'Email delivery via SMTP. Config: { "smtp_host": "...", "smtp_port": 587, "from": "...", "to": ["..."] }',
  http_webhook: 'Generic HTTP webhook. Config: { "url": "https://...", "method": "POST", "headers": {} }',
}

const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const isEditing = ref(false)
const editingId = ref('')

const tableData = ref<DestinationRow[]>([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(20)

const formRef = ref<FormInstance>()
const form = reactive({
  name: '',
  type: 'http_webhook',
  config: '',
  secret: '',
  timeout_ms: 10000,
  circuit_breaker: '',
  enabled: true,
})

const formRules: FormRules = {
  name: [{ required: true, message: 'Name is required', trigger: 'blur' }],
  type: [{ required: true, message: 'Type is required', trigger: 'change' }],
}

const typeHelpText = computed(() => typeHelpMap[form.type] ?? '')

function resetForm() {
  form.name = ''
  form.type = 'http_webhook'
  form.config = ''
  form.secret = ''
  form.timeout_ms = 10000
  form.circuit_breaker = ''
  form.enabled = true
}

function openDialog(row?: DestinationRow) {
  resetForm()
  if (row) {
    isEditing.value = true
    editingId.value = row.id
    form.name = row.name ?? ''
    form.type = row.type ?? 'http_webhook'
    form.config = typeof row.config === 'string' ? row.config : JSON.stringify(row.config ?? {}, null, 2)
    form.secret = ''
    form.timeout_ms = (row.timeout_ms as number) ?? 10000
    form.circuit_breaker =
      typeof row.circuit_breaker === 'string'
        ? row.circuit_breaker
        : JSON.stringify(row.circuit_breaker ?? {}, null, 2)
    form.enabled = row.enabled ?? true
  } else {
    isEditing.value = false
    editingId.value = ''
  }
  dialogVisible.value = true
}

async function fetchData() {
  loading.value = true
  try {
    const res = await listDestinations({ page: currentPage.value, size: pageSize.value })
    tableData.value = res.data?.items ?? []
    total.value = res.data?.total ?? 0
  } catch (e: unknown) {
    const msg = (e as { response?: { data?: { message?: string } } })?.response?.data?.message || 'Failed to load destinations'
    ElMessage.error(msg)
  } finally {
    loading.value = false
  }
}

async function handleSave() {
  if (!formRef.value) return
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  saving.value = true
  try {
    let configParsed: unknown = {}
    try {
      configParsed = form.config ? JSON.parse(form.config) : {}
    } catch {
      ElMessage.error('Invalid JSON in Config')
      return
    }
    let cbParsed: unknown = null
    if (form.circuit_breaker) {
      try {
        cbParsed = JSON.parse(form.circuit_breaker)
      } catch {
        ElMessage.error('Invalid JSON in Circuit Breaker Config')
        return
      }
    }

    const payload: Record<string, unknown> = {
      name: form.name,
      type: form.type,
      config: configParsed,
      timeout_ms: form.timeout_ms,
      circuit_breaker: cbParsed,
      enabled: form.enabled,
    }
    if (form.secret) {
      payload.secret = form.secret
    }
    if (isEditing.value) {
      await updateDestination(editingId.value, payload)
      ElMessage.success('Destination updated')
    } else {
      await createDestination(payload)
      ElMessage.success('Destination created')
    }
    dialogVisible.value = false
    await fetchData()
  } catch (e: unknown) {
    const msg = (e as { response?: { data?: { message?: string } } })?.response?.data?.message || 'Failed to save destination'
    ElMessage.error(msg)
  } finally {
    saving.value = false
  }
}

async function toggleEnabled(row: DestinationRow, val: boolean) {
  try {
    await updateDestination(row.id, { ...row, enabled: val })
    ElMessage.success(`Destination ${val ? 'enabled' : 'disabled'}`)
  } catch (e: unknown) {
    row.enabled = !val
    const msg = (e as { response?: { data?: { message?: string } } })?.response?.data?.message || 'Failed to toggle destination'
    ElMessage.error(msg)
  }
}

async function handleDelete(row: DestinationRow) {
  try {
    await ElMessageBox.confirm(
      `Are you sure you want to delete destination "${row.name}"?`,
      'Confirm Delete',
      { type: 'warning' }
    )
    await deleteDestination(row.id)
    ElMessage.success('Destination deleted')
    await fetchData()
  } catch {
    // cancelled
  }
}

onMounted(() => {
  fetchData()
})
</script>

<style lang="scss" scoped>
.destinations-container {
  .page-actions {
    margin-bottom: 16px;
    display: flex;
    justify-content: flex-end;
  }

  .pagination-wrap {
    margin-top: 16px;
    display: flex;
    justify-content: flex-end;
  }

  .type-help {
    margin-bottom: 16px;
    margin-left: 150px;
  }

  :deep(.monospace-input .el-textarea__inner) {
    font-family: 'Courier New', Courier, monospace;
    font-size: 13px;
  }
}
</style>
