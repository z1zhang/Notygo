<template>
  <div class="sources-container">
    <div class="page-actions">
      <el-button type="primary" @click="openDialog()">
        <el-icon><Plus /></el-icon> New Source
      </el-button>
    </div>

    <el-card shadow="never" v-loading="loading">
      <el-table :data="tableData" stripe style="width: 100%">
        <el-table-column prop="name" label="Name" min-width="140" />
        <el-table-column prop="slug" label="Slug" min-width="120" />
        <el-table-column prop="auth_type" label="Auth Type" width="120">
          <template #default="{ row }">
            <el-tag size="small">{{ row.auth_type || 'none' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="enabled" label="Enabled" width="100">
          <template #default="{ row }">
            <el-switch
              v-model="row.enabled"
              @change="(val: boolean) => toggleEnabled(row, val)"
            />
          </template>
        </el-table-column>
        <el-table-column prop="mapping_id" label="Mapping" width="140">
          <template #default="{ row }">
            {{ getMappingName(row.mapping_id) }}
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
      :title="isEditing ? 'Edit Source' : 'New Source'"
      width="560px"
      destroy-on-close
    >
      <el-form
        ref="formRef"
        :model="form"
        :rules="formRules"
        label-width="130px"
        label-position="right"
      >
        <el-form-item label="Name" prop="name">
          <el-input v-model="form.name" placeholder="Source name" />
        </el-form-item>
        <el-form-item label="Auth Type" prop="auth_type">
          <el-select v-model="form.auth_type" placeholder="Select auth type">
            <el-option label="None" value="none" />
            <el-option label="Bearer" value="bearer" />
            <el-option label="HMAC" value="hmac" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="form.auth_type !== 'none'" label="Secret / Token" prop="secret">
          <el-input
            v-model="form.secret"
            type="password"
            show-password
            placeholder="Bearer token or HMAC secret"
          />
        </el-form-item>
        <el-form-item label="Max Body Bytes" prop="max_body_bytes">
          <el-input-number v-model="form.max_body_bytes" :min="0" :step="1024" />
        </el-form-item>
        <el-form-item label="Rate Limit" prop="rate_limit">
          <el-input
            v-model="form.rate_limit"
            placeholder='{"rate":10,"burst":20}'
          />
        </el-form-item>
        <el-form-item label="Mapping" prop="mapping_id">
          <el-select v-model="form.mapping_id" placeholder="Select mapping" clearable>
            <el-option
              v-for="m in mappingOptions"
              :key="m.id"
              :label="m.name"
              :value="m.id"
            />
          </el-select>
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
import { ref, reactive, onMounted } from 'vue'
import type { FormInstance, FormRules } from 'element-plus'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listSources, createSource, updateSource, deleteSource } from '@/api/sources'
import { listMappings } from '@/api/mappings'

interface SourceRow {
  id: string
  name: string
  slug: string
  auth_type: string
  enabled: boolean
  mapping_id: string
  [key: string]: unknown
}

interface MappingOption {
  id: string
  name: string
}

const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const isEditing = ref(false)
const editingId = ref('')

const tableData = ref<SourceRow[]>([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(20)

const mappingOptions = ref<MappingOption[]>([])

const formRef = ref<FormInstance>()
const form = reactive({
  name: '',
  auth_type: 'none',
  secret: '',
  max_body_bytes: 1048576,
  rate_limit: '',
  mapping_id: '',
  enabled: true,
})

const formRules: FormRules = {
  name: [{ required: true, message: 'Name is required', trigger: 'blur' }],
  auth_type: [{ required: true, message: 'Auth type is required', trigger: 'change' }],
}

function getMappingName(id: string): string {
  if (!id) return '-'
  const found = mappingOptions.value.find((m) => m.id === id)
  return found ? found.name : id
}

function resetForm() {
  form.name = ''
  form.auth_type = 'none'
  form.secret = ''
  form.max_body_bytes = 1048576
  form.rate_limit = ''
  form.mapping_id = ''
  form.enabled = true
}

function openDialog(row?: SourceRow) {
  resetForm()
  if (row) {
    isEditing.value = true
    editingId.value = row.id
    form.name = row.name ?? ''
    form.auth_type = row.auth_type ?? 'none'
    form.secret = ''
    form.max_body_bytes = (row.max_body_bytes as number) ?? 1048576
    form.rate_limit = typeof row.rate_limit === 'string' ? row.rate_limit : JSON.stringify(row.rate_limit ?? {})
    form.mapping_id = row.mapping_id ?? ''
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
    const res = await listSources({ page: currentPage.value, size: pageSize.value })
    tableData.value = res.data?.items ?? []
    total.value = res.data?.total ?? 0
  } catch (e: unknown) {
    const msg = (e as { response?: { data?: { message?: string } } })?.response?.data?.message || 'Failed to load sources'
    ElMessage.error(msg)
  } finally {
    loading.value = false
  }
}

async function fetchMappings() {
  try {
    const res = await listMappings({ page: 1, size: 200 })
    mappingOptions.value = (res.data?.items ?? []).map(
      (m: Record<string, unknown>) => ({ id: String(m.id ?? ''), name: String(m.name ?? '') })
    )
  } catch {
    // silent - mappings are optional reference data
  }
}

async function handleSave() {
  if (!formRef.value) return
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  saving.value = true
  try {
    const payload: Record<string, unknown> = {
      name: form.name,
      auth_type: form.auth_type,
      max_body_bytes: form.max_body_bytes,
      enabled: form.enabled,
      mapping_id: form.mapping_id || null,
    }
    if (form.auth_type !== 'none' && form.secret) {
      payload.secret = form.secret
    }
    if (form.rate_limit) {
      try {
        payload.rate_limit = JSON.parse(form.rate_limit)
      } catch {
        payload.rate_limit = form.rate_limit
      }
    }

    if (isEditing.value) {
      await updateSource(editingId.value, payload)
      ElMessage.success('Source updated')
    } else {
      await createSource(payload)
      ElMessage.success('Source created')
    }
    dialogVisible.value = false
    await fetchData()
  } catch (e: unknown) {
    const msg = (e as { response?: { data?: { message?: string } } })?.response?.data?.message || 'Failed to save source'
    ElMessage.error(msg)
  } finally {
    saving.value = false
  }
}

async function toggleEnabled(row: SourceRow, val: boolean) {
  try {
    await updateSource(row.id, { ...row, enabled: val })
    ElMessage.success(`Source ${val ? 'enabled' : 'disabled'}`)
  } catch (e: unknown) {
    row.enabled = !val
    const msg = (e as { response?: { data?: { message?: string } } })?.response?.data?.message || 'Failed to toggle source'
    ElMessage.error(msg)
  }
}

async function handleDelete(row: SourceRow) {
  try {
    await ElMessageBox.confirm(
      `Are you sure you want to delete source "${row.name}"?`,
      'Confirm Delete',
      { type: 'warning' }
    )
    await deleteSource(row.id)
    ElMessage.success('Source deleted')
    await fetchData()
  } catch {
    // cancelled or error already shown
  }
}

onMounted(() => {
  fetchData()
  fetchMappings()
})
</script>

<style lang="scss" scoped>
.sources-container {
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
}
</style>
