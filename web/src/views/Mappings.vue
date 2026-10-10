<template>
  <div class="mappings-container">
    <div class="page-actions">
      <el-button type="primary" @click="openDialog()">
        <el-icon><Plus /></el-icon> New Mapping
      </el-button>
    </div>

    <el-card shadow="never" v-loading="loading">
      <el-table :data="tableData" stripe style="width: 100%">
        <el-table-column prop="name" label="Name" min-width="160" />
        <el-table-column prop="revision" label="Revision" width="100" />
        <el-table-column label="Fields Count" width="120">
          <template #default="{ row }">
            <el-tag size="small">{{ (row.fields ?? []).length }}</el-tag>
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
      :title="isEditing ? 'Edit Mapping' : 'New Mapping'"
      width="720px"
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
          <el-input v-model="form.name" placeholder="Mapping name" />
        </el-form-item>

        <el-divider content-position="left">Fields</el-divider>

        <div v-for="(field, index) in form.fields" :key="index" class="field-row">
          <el-row :gutter="8" align="middle">
            <el-col :span="5">
              <el-form-item
                :prop="`fields.${index}.source_path`"
                :rules="[{ required: true, message: 'Required', trigger: 'blur' }]"
                label=""
              >
                <el-input v-model="field.source_path" placeholder="Source Path" />
              </el-form-item>
            </el-col>
            <el-col :span="4">
              <el-form-item :prop="`fields.${index}.target_key`" label="">
                <el-input v-model="field.target_key" placeholder="Target Key" />
              </el-form-item>
            </el-col>
            <el-col :span="4">
              <el-form-item :prop="`fields.${index}.value_type`" label="">
                <el-select v-model="field.value_type" placeholder="Type">
                  <el-option label="String" value="string" />
                  <el-option label="Number" value="number" />
                  <el-option label="Bool" value="bool" />
                  <el-option label="JSON" value="json" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="3">
              <el-form-item :prop="`fields.${index}.default_value`" label="">
                <el-input v-model="field.default_value" placeholder="Default" />
              </el-form-item>
            </el-col>
            <el-col :span="3">
              <el-form-item label="">
                <el-switch v-model="field.required" active-text="Req" />
              </el-form-item>
            </el-col>
            <el-col :span="3">
              <el-form-item :prop="`fields.${index}.transform`" label="">
                <el-select v-model="field.transform" placeholder="Transform" clearable>
                  <el-option label="None" value="" />
                  <el-option label="Lower" value="lower" />
                  <el-option label="Upper" value="upper" />
                  <el-option label="Trim" value="trim" />
                  <el-option label="Regex" value="regex" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="1">
              <el-button
                text
                type="danger"
                :icon="Delete"
                @click="removeField(index)"
              />
            </el-col>
          </el-row>
        </div>

        <el-button type="primary" text @click="addField">
          <el-icon><Plus /></el-icon> Add Field
        </el-button>
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
import { Delete } from '@element-plus/icons-vue'
import { listMappings, createMapping, updateMapping, deleteMapping } from '@/api/mappings'

interface FieldRow {
  source_path: string
  target_key: string
  value_type: string
  default_value: string
  required: boolean
  transform: string
  multi_result_mode: string
}

interface MappingRow {
  id: string
  name: string
  revision: number
  fields: FieldRow[]
  [key: string]: unknown
}

const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const isEditing = ref(false)
const editingId = ref('')

const tableData = ref<MappingRow[]>([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(20)

const formRef = ref<FormInstance>()
const form = reactive({
  name: '',
  fields: [] as FieldRow[],
})

const formRules: FormRules = {
  name: [{ required: true, message: 'Name is required', trigger: 'blur' }],
}

function createEmptyField(): FieldRow {
  return {
    source_path: '',
    target_key: '',
    value_type: 'string',
    default_value: '',
    required: false,
    transform: '',
    multi_result_mode: 'first',
  }
}

function addField() {
  form.fields.push(createEmptyField())
}

function removeField(index: number) {
  form.fields.splice(index, 1)
}

function resetForm() {
  form.name = ''
  form.fields = [createEmptyField()]
}

function openDialog(row?: MappingRow) {
  resetForm()
  if (row) {
    isEditing.value = true
    editingId.value = row.id
    form.name = row.name ?? ''
    form.fields = (row.fields ?? []).length
      ? (row.fields as FieldRow[]).map((f) => ({ ...createEmptyField(), ...f }))
      : [createEmptyField()]
  } else {
    isEditing.value = false
    editingId.value = ''
  }
  dialogVisible.value = true
}

async function fetchData() {
  loading.value = true
  try {
    const res = await listMappings({ page: currentPage.value, size: pageSize.value })
    tableData.value = res.data?.items ?? []
    total.value = res.data?.total ?? 0
  } catch (e: unknown) {
    const msg = (e as { response?: { data?: { message?: string } } })?.response?.data?.message || 'Failed to load mappings'
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
    const payload: Record<string, unknown> = {
      name: form.name,
      fields: form.fields.filter((f) => f.source_path || f.target_key),
    }
    if (isEditing.value) {
      await updateMapping(editingId.value, payload)
      ElMessage.success('Mapping updated')
    } else {
      await createMapping(payload)
      ElMessage.success('Mapping created')
    }
    dialogVisible.value = false
    await fetchData()
  } catch (e: unknown) {
    const msg = (e as { response?: { data?: { message?: string } } })?.response?.data?.message || 'Failed to save mapping'
    ElMessage.error(msg)
  } finally {
    saving.value = false
  }
}

async function handleDelete(row: MappingRow) {
  try {
    await ElMessageBox.confirm(
      `Are you sure you want to delete mapping "${row.name}"?`,
      'Confirm Delete',
      { type: 'warning' }
    )
    await deleteMapping(row.id)
    ElMessage.success('Mapping deleted')
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
.mappings-container {
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

  .field-row {
    margin-bottom: 4px;
    padding: 8px;
    background-color: #fafafa;
    border-radius: 4px;
  }
}
</style>
