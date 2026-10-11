<template>
  <div class="mappings-container">
    <div class="page-actions">
      <el-button type="primary" @click="openDialog()">
        <el-icon><Plus /></el-icon> {{ $t('mappings.newMapping') }}
      </el-button>
    </div>

    <el-card shadow="never" v-loading="loading">
      <el-table :data="tableData" stripe style="width: 100%">
        <el-table-column prop="name" :label="$t('common.name')" min-width="160" />
        <el-table-column prop="revision" :label="$t('mappings.revision')" width="100" />
        <el-table-column :label="$t('mappings.fieldsCount')" width="120">
          <template #default="{ row }">
            <el-tag size="small">{{ (row.fields ?? []).length }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="$t('common.actions')" width="160" fixed="right">
          <template #default="{ row }">
            <el-button text type="primary" size="small" @click="openDialog(row)">{{ $t('common.edit') }}</el-button>
            <el-button text type="danger" size="small" @click="handleDelete(row)">{{ $t('common.delete') }}</el-button>
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
      :title="isEditing ? $t('mappings.editMapping') : $t('mappings.newMapping')"
      width="800px"
      destroy-on-close
    >
      <el-form
        ref="formRef"
        :model="form"
        :rules="formRules"
        label-width="auto"
        label-position="left"
      >
        <el-form-item :label="$t('common.name')" prop="name">
          <el-input v-model="form.name" :placeholder="$t('mappings.mappingName')" />
        </el-form-item>

        <el-divider content-position="left">{{ $t('mappings.fields') }}</el-divider>

        <div v-for="(field, index) in form.fields" :key="index" class="field-card">
          <div class="field-card-header">
            <span class="field-card-title">{{ $t('mappings.fieldNo', { n: index + 1 }) }}</span>
            <el-button text type="danger" size="small" @click="removeField(index)">
              <el-icon><Delete /></el-icon> {{ $t('common.remove') }}
            </el-button>
          </div>

          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item
                :prop="`fields.${index}.source_path`"
                :rules="[{ required: true, message: t('mappings.sourcePathRequired'), trigger: 'blur' }]"
                :label="$t('mappings.sourcePath')"
              >
                <el-input v-model="field.source_path" :placeholder="$t('mappings.sourcePathPlaceholder')" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item
                :prop="`fields.${index}.target_key`"
                :rules="[{ required: true, message: t('mappings.targetKeyRequired'), trigger: 'blur' }]"
                :label="$t('mappings.targetKey')"
              >
                <el-input v-model="field.target_key" :placeholder="$t('mappings.targetKeyPlaceholder')" />
              </el-form-item>
            </el-col>
          </el-row>

          <el-row :gutter="16">
            <el-col :span="8">
              <el-form-item :prop="`fields.${index}.value_type`" :label="$t('mappings.valueType')">
                <el-select v-model="field.value_type" :placeholder="$t('mappings.selectType')" style="width: 100%">
                  <el-option :label="$t('mappings.string')" value="string" />
                  <el-option :label="$t('mappings.number')" value="number" />
                  <el-option :label="$t('mappings.bool')" value="bool" />
                  <el-option :label="$t('mappings.json')" value="json" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item :prop="`fields.${index}.multi_result_mode`" :label="$t('mappings.multiResult')">
                <el-select v-model="field.multi_result_mode" :placeholder="$t('mappings.selectMode')" style="width: 100%">
                  <el-option :label="$t('mappings.first')" value="first" />
                  <el-option :label="$t('mappings.join')" value="join" />
                  <el-option :label="$t('mappings.array')" value="array" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item :prop="`fields.${index}.transform`" :label="$t('mappings.transform')">
                <el-select v-model="field.transform" :placeholder="$t('mappings.selectTransform')" clearable style="width: 100%">
                  <el-option :label="$t('mappings.lower')" value="lower" />
                  <el-option :label="$t('mappings.upper')" value="upper" />
                  <el-option :label="$t('mappings.trim')" value="trim" />
                  <el-option :label="$t('mappings.regex')" value="regex" />
                </el-select>
              </el-form-item>
            </el-col>
          </el-row>

          <el-row :gutter="16">
            <el-col :span="8">
              <el-form-item :prop="`fields.${index}.default_value`" :label="$t('mappings.default')">
                <el-input v-model="field.default_value" :placeholder="$t('mappings.optionalDefault')" />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item :prop="`fields.${index}.description`" :label="$t('mappings.description')">
                <el-input v-model="field.description" :placeholder="$t('mappings.fieldDescription')" />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item :label="$t('mappings.required')">
                <el-switch v-model="field.required" />
              </el-form-item>
            </el-col>
          </el-row>
        </div>

        <el-button type="primary" text @click="addField">
          <el-icon><Plus /></el-icon> {{ $t('mappings.addField') }}
        </el-button>
      </el-form>

      <template #footer>
        <el-button @click="dialogVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="saving" @click="handleSave">{{ $t('common.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import type { FormInstance, FormRules } from 'element-plus'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { Delete } from '@element-plus/icons-vue'
import { listMappings, createMapping, updateMapping, deleteMapping } from '@/api/mappings'

const { t } = useI18n()

interface FieldRow {
  source_path: string
  target_key: string
  value_type: string
  default_value: string
  required: boolean
  transform: string
  multi_result_mode: string
  description: string
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
  name: [{ required: true, message: () => t('mappings.nameRequired'), trigger: 'blur' }],
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
    description: '',
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

// toNullableStr returns null for empty strings (matches backend *string with omitempty)
function toNullableStr(val: string): string | null {
  return val || null
}

async function fetchData() {
  loading.value = true
  try {
    const res = await listMappings({ page: currentPage.value, size: pageSize.value })
    tableData.value = res.data?.data ?? []
    total.value = res.data?.total ?? 0
  } catch (e: unknown) {
    const msg = (e as { response?: { data?: { message?: string } } })?.response?.data?.message || t('mappings.failedToLoadMappings')
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
      fields: form.fields
        .filter((f) => f.source_path || f.target_key)
        .map((f) => ({
          source_path: f.source_path,
          target_key: f.target_key,
          value_type: f.value_type || 'string',
          default_value: toNullableStr(f.default_value),
          required: f.required,
          transform: toNullableStr(f.transform),
          multi_result_mode: toNullableStr(f.multi_result_mode),
          description: toNullableStr(f.description),
        })),
    }
    if (isEditing.value) {
      await updateMapping(editingId.value, payload)
      ElMessage.success(t('mappings.mappingUpdated'))
    } else {
      await createMapping(payload)
      ElMessage.success(t('mappings.mappingCreated'))
    }
    dialogVisible.value = false
    await fetchData()
  } catch (e: unknown) {
    const msg = (e as { response?: { data?: { message?: string } } })?.response?.data?.message || t('mappings.failedToSaveMapping')
    ElMessage.error(msg)
  } finally {
    saving.value = false
  }
}

async function handleDelete(row: MappingRow) {
  try {
    await ElMessageBox.confirm(
      t('common.confirmDeleteMsg', { entity: t('nav.mappings'), name: row.name }),
      t('common.confirmDelete'),
      { type: 'warning' }
    )
    await deleteMapping(row.id)
    ElMessage.success(t('mappings.mappingDeleted'))
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

  .field-card {
    margin-bottom: 12px;
    padding: 16px;
    background-color: var(--el-fill-color-lighter, #fafafa);
    border: 1px solid var(--el-border-color-lighter, #ebeef5);
    border-radius: 8px;

    .field-card-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 8px;
    }

    .field-card-title {
      font-weight: 600;
      font-size: 14px;
      color: var(--el-text-color-primary, #303133);
    }
  }
}
</style>
