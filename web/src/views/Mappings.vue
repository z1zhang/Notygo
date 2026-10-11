<template>
  <div class="mappings-container">
    <div class="page-actions">
      <el-button type="primary" @click="openCreateDialog(); resetForm()">
        <el-icon><Plus /></el-icon> {{ $t('mappings.newMapping') }}
      </el-button>
    </div>

    <el-card shadow="never" v-loading="loading">
      <el-table :data="tableData" stripe style="width: 100%">
        <el-table-column prop="name" :label="$t('common.name')" min-width="160" />
        <el-table-column prop="revision" :label="$t('mappings.revision')" width="100" />
        <el-table-column :label="$t('mappings.fieldsCount')" width="120">
          <template #default="{ row }">
            <el-tag>{{ (row.fields ?? []).length }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="$t('common.actions')" width="180" fixed="right">
          <template #default="{ row }">
            <div class="action-buttons">
              <el-button type="primary" @click="openEditDialog(row.id); fillForm(row)">{{ $t('common.edit') }}</el-button>
              <el-button type="danger" @click="handleDelete(row.id, row.name)">{{ $t('common.delete') }}</el-button>
            </div>
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
            <el-button text type="danger" @click="removeField(index)">
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
        <el-button @click="closeDialog">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="saving" @click="handleSave">{{ $t('common.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import type { FormInstance, FormRules } from 'element-plus'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { Delete } from '@element-plus/icons-vue'
import { listMappings, createMapping, updateMapping, deleteMapping } from '@/api/mappings'
import { useCrudPage } from '@/composables/useCrudPage'
import { getErrorMessage } from '@/utils/helpers'
import type { MappingResponse, MappingEntry } from '@/api/types'

const { t } = useI18n()

const {
  loading,
  saving,
  dialogVisible,
  isEditing,
  editingId,
  tableData,
  total,
  currentPage,
  pageSize,
  fetchData,
  handleDelete,
  openCreateDialog,
  openEditDialog,
  closeDialog,
} = useCrudPage<MappingResponse>({
  listApi: listMappings,
  deleteApi: (id) => deleteMapping(id),
  entityI18nKey: 'nav.mappings',
})

// Use MappingEntry as the form field type; local mutable version for the form
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
  form.fields = []
}

function fillForm(row: MappingResponse) {
  form.name = row.name ?? ''
  form.fields = (row.fields ?? []).length
    ? (row.fields as MappingEntry[]).map((f) => ({ ...createEmptyField(), ...f }))
    : []
}

// toNullableStr returns null for empty strings (matches backend *string with omitempty)
function toNullableStr(val: string): string | null {
  return val || null
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
    closeDialog()
    await fetchData()
  } catch (e: unknown) {
    ElMessage.error(getErrorMessage(e, t('mappings.failedToSaveMapping')))
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  fetchData()
})
</script>

<style lang="scss" scoped>
.mappings-container {
  // page-actions, pagination-wrap, action-buttons — provided by global.scss

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
