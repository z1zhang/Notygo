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
        <el-table-column :label="$t('mappings.rules')" min-width="180">
          <template #default="{ row }">
            <span v-if="(row.rule_ids ?? []).length">
              <el-tag v-for="rId in (row.rule_ids as string[]).slice(0, 3)" :key="rId" style="margin-right: 4px">
                {{ lookupName(ruleOptions, rId) }}
              </el-tag>
              <el-tag v-if="(row.rule_ids as string[]).length > 3" type="info">
                +{{ (row.rule_ids as string[]).length - 3 }}
              </el-tag>
            </span>
            <span v-else>-</span>
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
        label-width="90px"
        label-position="left"
      >
        <el-form-item :label="$t('common.name')" prop="name">
          <el-input v-model="form.name" :placeholder="$t('mappings.mappingName')" />
        </el-form-item>

        <el-form-item :label="$t('mappings.rules')">
          <el-select
            v-model="form.rule_ids"
            multiple
            :placeholder="$t('mappings.selectRules')"
            style="width: 100%"
          >
            <el-option
              v-for="r in ruleOptions"
              :key="r.id"
              :label="r.name"
              :value="r.id"
            />
          </el-select>
        </el-form-item>

        <el-form-item :label="$t('mappings.continueOnMatch')">
          <el-switch v-model="form.continue_on_match" />
        </el-form-item>

        <!-- Smart Import (collapsible) -->
        <el-collapse v-model="smartImportExpanded" class="smart-import-collapse">
          <el-collapse-item name="smart">
            <template #title>
              <div class="smart-import-title">
                <el-icon><MagicStick /></el-icon>
                {{ $t('mappings.smartImport') }}
                <el-tag v-if="smartImportStructure" size="small" type="info" style="margin-left: 8px">
                  {{ smartImportSuggestions.length }} {{ $t('mappings.fieldsFound') }}
                </el-tag>
              </div>
            </template>

            <el-input
              v-model="smartImportJson"
              type="textarea"
              :rows="4"
              :placeholder="$t('mappings.pasteJsonPlaceholder')"
              spellcheck="false"
            />
            <div class="smart-import-actions">
              <el-button
                type="primary"
                size="small"
                :loading="smartImportLoading"
                :disabled="!smartImportJson.trim()"
                @click="handleSmartImport"
              >
                <el-icon><Search /></el-icon> {{ $t('mappings.analyzeJson') }}
              </el-button>
              <el-button
                v-if="smartImportSuggestions.length > 0"
                type="success"
                size="small"
                @click="addAllSuggestions"
              >
                <el-icon><Plus /></el-icon> {{ $t('mappings.addAllFields') }}
              </el-button>
            </div>

            <!-- Suggested fields list -->
            <div v-if="smartImportSuggestions.length > 0" class="suggested-fields-list">
              <div class="suggested-fields-header">
                <span>{{ $t('mappings.suggestedFields') }}</span>
                <div v-if="smartImportStructure" style="display: flex; gap: 6px">
                  <el-tag size="small" :type="smartImportStructure.root_is_array ? 'warning' : 'success'">
                    {{ smartImportStructure.root_is_array ? $t('playground.rootArray') : $t('playground.rootObject') }}
                  </el-tag>
                  <el-tag size="small" type="info">
                    {{ smartImportStructure.item_count }}
                  </el-tag>
                </div>
              </div>
              <div class="suggested-fields-scroll">
                <div
                  v-for="item in smartImportSuggestions"
                  :key="item.source_path + (item.multi_result_mode || '')"
                  class="suggested-field-row"
                  :class="{ 'is-added': addedPaths.has(item.source_path + (item.multi_result_mode || '')) }"
                >
                  <span class="suggested-field-path" :title="item.source_path">{{ item.source_path }}</span>
                  <el-tag :type="valueTypeTag(item.value_type)" size="small">{{ item.value_type }}</el-tag>
                  <span class="suggested-field-sample" :title="item.sample_value">{{ item.sample_value }}</span>
                  <el-button
                    v-if="!addedPaths.has(item.source_path + (item.multi_result_mode || ''))"
                    type="primary"
                    text
                    size="small"
                    @click="addFromSuggestion(item)"
                  >
                    <el-icon><Plus /></el-icon>
                  </el-button>
                  <el-tag v-else type="success" size="small">&#10003;</el-tag>
                </div>
              </div>
            </div>
          </el-collapse-item>
        </el-collapse>

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
                  <el-option :label="$t('mappings.joinComma')" value="join(, )" />
                  <el-option :label="$t('mappings.array')" value="array" />
                  <el-option :label="$t('mappings.count')" value="count" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item :prop="`fields.${index}.transform`" :label="$t('mappings.transform')">
                <el-select v-model="field.transform" :placeholder="$t('mappings.selectTransform')" clearable style="width: 100%">
                  <el-option :label="$t('mappings.lower')" value="to_lower" />
                  <el-option :label="$t('mappings.upper')" value="to_upper" />
                  <el-option :label="$t('mappings.trim')" value="trim" />
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
import { ref, reactive, computed, onMounted } from 'vue'
import type { FormInstance, FormRules } from 'element-plus'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { Delete, Plus, Search, MagicStick } from '@element-plus/icons-vue'
import { listMappings, createMapping, updateMapping, deleteMapping } from '@/api/mappings'
import { useCrudPage } from '@/composables/useCrudPage'
import { useOptions } from '@/composables/useOptions'
import { lookupName } from '@/utils/lookup'
import { getErrorMessage } from '@/utils/helpers'
import { previewParse } from '@/api/preview'
import type { MappingResponse, MappingEntry } from '@/api/types'
import type { SuggestedField, StructureInfo } from '@/api/preview'

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

const { ruleOptions, fetchRuleOptions } = useOptions()

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
  rule_ids: [] as string[],
  continue_on_match: false,
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
  form.rule_ids = []
  form.continue_on_match = false
  form.fields = []
  // Reset smart import state
  smartImportJson.value = ''
  smartImportStructure.value = null
  smartImportSuggestions.value = []
  smartImportExpanded.value = []
}

function fillForm(row: MappingResponse) {
  form.name = row.name ?? ''
  form.rule_ids = (row.rule_ids ?? []) as string[]
  form.continue_on_match = row.continue_on_match ?? false
  form.fields = (row.fields ?? []).length
    ? (row.fields as MappingEntry[]).map((f) => ({
        source_path: f.source_path,
        target_key: f.target_key,
        value_type: f.value_type,
        default_value: f.default_value ?? '',
        required: f.required,
        transform: f.transform ?? '',
        multi_result_mode: f.multi_result_mode ?? 'first',
        description: f.description ?? '',
      }))
    : []
  // Reset smart import state on edit
  smartImportJson.value = ''
  smartImportStructure.value = null
  smartImportSuggestions.value = []
  smartImportExpanded.value = []
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
      rule_ids: form.rule_ids,
      continue_on_match: form.continue_on_match,
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

// ── Smart Import ─────────────────────────────────────

const smartImportExpanded = ref<string[]>([])
const smartImportJson = ref('')
const smartImportLoading = ref(false)
const smartImportStructure = ref<StructureInfo | null>(null)
const smartImportSuggestions = ref<SuggestedField[]>([])

// Track which source_path+mode combos are already in form.fields
// Normalize: 'first' is the default mode, equivalent to null/empty
const addedPaths = computed(() => {
  const s = new Set<string>()
  for (const f of form.fields) {
    const mode = f.multi_result_mode && f.multi_result_mode !== 'first' ? f.multi_result_mode : ''
    s.add(f.source_path + mode)
  }
  return s
})

function addFromSuggestion(sf: SuggestedField) {
  form.fields.push({
    source_path: sf.source_path,
    target_key: sf.target_key,
    value_type: sf.value_type || 'string',
    default_value: '',
    required: false,
    transform: '',
    multi_result_mode: sf.multi_result_mode || 'first',
    description: '',
  })
}

function addAllSuggestions() {
  for (const sf of smartImportSuggestions.value) {
    const key = sf.source_path + (sf.multi_result_mode || '')
    if (!addedPaths.value.has(key)) {
      addFromSuggestion(sf)
    }
  }
}

function valueTypeTag(vt: string): string {
  switch (vt) {
    case 'string': return ''
    case 'number': return 'warning'
    case 'boolean': return 'success'
    case 'array': return 'danger'
    default: return 'info'
  }
}

async function handleSmartImport() {
  if (!smartImportJson.value.trim()) return
  try { JSON.parse(smartImportJson.value) } catch {
    ElMessage.error(t('templates.invalidJson'))
    return
  }
  smartImportLoading.value = true
  try {
    const res = await previewParse(smartImportJson.value)
    smartImportStructure.value = res.data.structure
    smartImportSuggestions.value = res.data.suggested_fields || []
  } catch (e: unknown) {
    ElMessage.error(getErrorMessage(e, t('common.error')))
  } finally {
    smartImportLoading.value = false
  }
}

onMounted(() => {
  fetchData()
  fetchRuleOptions()
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

  // ── Smart Import ──

  .smart-import-collapse {
    margin-bottom: 12px;
    border: none;

    :deep(.el-collapse-item__header) {
      background-color: var(--el-fill-color-lighter, #fafafa);
      padding: 0 12px;
      border-radius: 6px;
      font-weight: 600;
      height: 36px;
      line-height: 36px;
    }

    :deep(.el-collapse-item__wrap) {
      border: none;
    }

    :deep(.el-collapse-item__content) {
      padding: 12px 0 0 0;
    }
  }

  .smart-import-title {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .smart-import-actions {
    margin-top: 8px;
    display: flex;
    gap: 8px;
  }

  .suggested-fields-list {
    margin-top: 12px;
  }

  .suggested-fields-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-weight: 600;
    font-size: 13px;
    margin-bottom: 8px;
  }

  .suggested-fields-scroll {
    max-height: 240px;
    overflow-y: auto;
    border: 1px solid var(--el-border-color-lighter, #ebeef5);
    border-radius: 4px;
    background-color: var(--el-fill-color-lighter, #fafafa);
  }

  .suggested-field-row {
    display: flex;
    align-items: center;
    padding: 5px 10px;
    gap: 8px;
    font-size: 13px;
    border-bottom: 1px solid var(--el-border-color-extra-light, #f2f2f2);

    &:hover {
      background-color: var(--el-fill-color, #f0f0f0);
    }

    &.is-added {
      opacity: 0.5;
    }

    &:last-child {
      border-bottom: none;
    }
  }

  .suggested-field-path {
    font-weight: 500;
    color: var(--el-color-primary, #409eff);
    min-width: 80px;
    max-width: 200px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    flex-shrink: 0;
  }

  .suggested-field-sample {
    color: var(--el-text-color-regular, #606266);
    font-size: 12px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    flex: 1;
    min-width: 0;
  }
}
</style>
