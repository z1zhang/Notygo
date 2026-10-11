<template>
  <div class="rules-container">
    <div class="page-actions">
      <el-button type="primary" @click="openDialog()">
        <el-icon><Plus /></el-icon> {{ $t('rules.newRule') }}
      </el-button>
    </div>

    <el-card shadow="never" v-loading="loading">
      <el-table :data="tableData" stripe style="width: 100%">
        <el-table-column prop="name" :label="$t('common.name')" min-width="160" />
        <el-table-column prop="priority" :label="$t('rules.priority')" width="100" />
        <el-table-column prop="enabled" :label="$t('common.enabled')" width="100">
          <template #default="{ row }">
            <el-switch v-model="row.enabled" @change="(val: boolean) => toggleEnabled(row, val)" />
          </template>
        </el-table-column>
        <el-table-column prop="template_id" :label="$t('rules.template')" width="140">
          <template #default="{ row }">
            {{ getTemplateName(row.template_id) }}
          </template>
        </el-table-column>
        <el-table-column :label="$t('rules.destinations')" min-width="140">
          <template #default="{ row }">
            <template v-if="(row.destination_ids ?? []).length">
              <el-tag v-for="dId in (row.destination_ids as string[]).slice(0, 3)" :key="dId" size="small" style="margin-right: 4px">
                {{ getDestinationName(dId) }}
              </el-tag>
              <el-tag v-if="(row.destination_ids as string[]).length > 3" size="small" type="info">
                +{{ (row.destination_ids as string[]).length - 3 }}
              </el-tag>
            </template>
            <span v-else>-</span>
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
      :title="isEditing ? $t('rules.editRule') : $t('rules.newRule')"
      width="820px"
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
          <el-input v-model="form.name" :placeholder="$t('rules.ruleName')" />
        </el-form-item>

        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item :label="$t('rules.priority')" prop="priority">
              <el-input-number v-model="form.priority" :min="0" :max="9999" style="width: 100%" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item :label="$t('common.enabled')">
              <el-switch v-model="form.enabled" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item :label="$t('rules.continueOnMatch')">
              <el-switch v-model="form.continue_on_match" />
            </el-form-item>
          </el-col>
        </el-row>

        <el-divider content-position="left">{{ $t('rules.conditions') }}</el-divider>

        <el-form-item :label="$t('rules.conditionMode')">
          <el-radio-group v-model="form.condition_mode">
            <el-radio value="and">{{ $t('rules.andMode') }}</el-radio>
            <el-radio value="or">{{ $t('rules.orMode') }}</el-radio>
          </el-radio-group>
        </el-form-item>

        <div v-for="(cond, index) in form.conditions" :key="index" class="condition-card">
          <div class="condition-card-header">
            <span class="condition-card-title">{{ $t('rules.conditionNo', { n: index + 1 }) }}</span>
            <el-button text type="danger" size="small" @click="removeCondition(index)">
              <el-icon><Delete /></el-icon> {{ $t('common.remove') }}
            </el-button>
          </div>

          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item
                :prop="`conditions.${index}.field`"
                :rules="[{ required: true, message: t('rules.fieldRequired'), trigger: 'blur' }]"
                :label="$t('rules.field')"
              >
                <el-input v-model="cond.field" :placeholder="$t('rules.fieldPlaceholder')" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item :prop="`conditions.${index}.operator`" :label="$t('rules.operator')">
                <el-select v-model="cond.operator" :placeholder="$t('rules.selectOperator')" style="width: 100%">
                  <el-option-group :label="$t('rules.equality')">
                    <el-option :label="$t('rules.equals')" value="eq" />
                    <el-option :label="$t('rules.notEquals')" value="neq" />
                  </el-option-group>
                  <el-option-group :label="$t('rules.stringGroup')">
                    <el-option :label="$t('rules.contains')" value="contains" />
                    <el-option :label="$t('rules.notContains')" value="not_contains" />
                    <el-option :label="$t('rules.regexMatch')" value="regex" />
                  </el-option-group>
                  <el-option-group :label="$t('rules.numericGroup')">
                    <el-option :label="$t('rules.greaterThan')" value="gt" />
                    <el-option :label="$t('rules.lessThan')" value="lt" />
                    <el-option :label="$t('rules.greaterOrEqual')" value="gte" />
                    <el-option :label="$t('rules.lessOrEqual')" value="lte" />
                  </el-option-group>
                  <el-option-group :label="$t('rules.existenceGroup')">
                    <el-option :label="$t('rules.exists')" value="exists" />
                    <el-option :label="$t('rules.notExists')" value="not_exists" />
                  </el-option-group>
                </el-select>
              </el-form-item>
            </el-col>
          </el-row>

          <el-row :gutter="16" v-if="!isExistenceOp(cond.operator)">
            <el-col :span="12">
              <el-form-item :prop="`conditions.${index}.value`" :label="$t('rules.value')">
                <el-input v-model="cond.value" :placeholder="getValuePlaceholder(cond.operator)" />
              </el-form-item>
            </el-col>
          </el-row>
        </div>

        <el-button type="primary" text @click="addCondition">
          <el-icon><Plus /></el-icon> {{ $t('rules.addCondition') }}
        </el-button>

        <el-divider content-position="left">{{ $t('rules.actionsSection') }}</el-divider>

        <el-form-item :label="$t('rules.template')" prop="template_id">
          <el-select v-model="form.template_id" :placeholder="$t('rules.selectTemplate')" clearable style="width: 100%">
            <el-option
              v-for="tpl in templateOptions"
              :key="tpl.id"
              :label="tpl.name"
              :value="tpl.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('rules.destinations')" prop="destination_ids">
          <el-select
            v-model="form.destination_ids"
            multiple
            :placeholder="$t('rules.selectDestinations')"
            style="width: 100%"
          >
            <el-option
              v-for="d in destinationOptions"
              :key="d.id"
              :label="d.name"
              :value="d.id"
            />
          </el-select>
        </el-form-item>

        <el-divider content-position="left">{{ $t('rules.deduplication') }}</el-divider>

        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item :label="$t('rules.windowSec')" label-width="110px">
              <el-input-number v-model="form.dedupe_window" :min="0" :max="86400" :placeholder="$t('rules.windowPlaceholder')" style="width: 100%" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item :label="$t('rules.keyFields')" label-width="110px">
              <el-select
                v-model="form.dedupe_key_fields"
                multiple
                filterable
                allow-create
                default-first-option
                :placeholder="$t('rules.keyFieldsPlaceholder')"
                style="width: 100%"
              >
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item :label="$t('rules.dedupeAction')" label-width="110px">
              <el-select v-model="form.dedupe_action" :placeholder="$t('rules.actionPlaceholder')" clearable style="width: 100%">
                <el-option :label="$t('rules.skip')" value="skip" />
                <el-option :label="$t('rules.queue')" value="queue" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
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
import { Delete } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'
import { listRules, createRule, updateRule, deleteRule } from '@/api/rules'
import { listTemplates } from '@/api/templates'
import { listDestinations } from '@/api/destinations'

const { t } = useI18n()

interface ConditionRow {
  field: string
  operator: string
  value: string
}

interface RuleRow {
  id: string
  name: string
  priority: number
  enabled: boolean
  condition_mode: string
  continue_on_match: boolean
  conditions: ConditionRow[]
  template_id: string
  destination_ids: string[]
  dedupe_config?: { window_seconds: number; key_fields: string[]; action: string } | null
  [key: string]: unknown
}

interface NameOption {
  id: string
  name: string
}

const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const isEditing = ref(false)
const editingId = ref('')

const tableData = ref<RuleRow[]>([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(20)

const templateOptions = ref<NameOption[]>([])
const destinationOptions = ref<NameOption[]>([])

const formRef = ref<FormInstance>()
const form = reactive({
  name: '',
  priority: 0,
  enabled: true,
  condition_mode: 'and',
  continue_on_match: false,
  conditions: [] as ConditionRow[],
  template_id: '',
  destination_ids: [] as string[],
  // dedupe
  dedupe_window: 0,
  dedupe_key_fields: [] as string[],
  dedupe_action: '',
})

const formRules: FormRules = {
  name: [{ required: true, message: t('rules.nameRequired'), trigger: 'blur' }],
}

function isExistenceOp(op: string): boolean {
  return op === 'exists' || op === 'not_exists'
}

function getValuePlaceholder(op: string): string {
  if (op === 'regex') return t('rules.regexPlaceholder')
  if (['gt', 'lt', 'gte', 'lte'].includes(op)) return t('rules.numericPlaceholder')
  return t('rules.comparisonPlaceholder')
}

function createEmptyCondition(): ConditionRow {
  return { field: '', operator: 'eq', value: '' }
}

function addCondition() {
  form.conditions.push(createEmptyCondition())
}

function removeCondition(index: number) {
  form.conditions.splice(index, 1)
}

function getTemplateName(id: string): string {
  if (!id) return '-'
  return templateOptions.value.find((tp) => tp.id === id)?.name ?? id
}

function getDestinationName(id: string): string {
  if (!id) return id
  return destinationOptions.value.find((d) => d.id === id)?.name ?? id.slice(0, 8)
}

function resetForm() {
  form.name = ''
  form.priority = 0
  form.enabled = true
  form.condition_mode = 'and'
  form.continue_on_match = false
  form.conditions = [createEmptyCondition()]
  form.template_id = ''
  form.destination_ids = []
  form.dedupe_window = 0
  form.dedupe_key_fields = []
  form.dedupe_action = ''
}

function openDialog(row?: RuleRow) {
  resetForm()
  if (row) {
    isEditing.value = true
    editingId.value = row.id
    form.name = row.name ?? ''
    form.priority = row.priority ?? 0
    form.enabled = row.enabled ?? true
    form.condition_mode = row.condition_mode ?? 'and'
    form.continue_on_match = row.continue_on_match ?? false

    // Backend stores conditions as ConditionGroup { operator, conditions: [...] }
    // Extract the flat conditions array from the group
    const rawConds = row.conditions as unknown
    if (Array.isArray(rawConds)) {
      // Legacy flat array format
      form.conditions = (rawConds as ConditionRow[]).map((c) => ({ ...createEmptyCondition(), ...c }))
    } else if (rawConds && typeof rawConds === 'object') {
      // ConditionGroup format: { operator, conditions: [...] }
      const group = rawConds as { operator?: string; conditions?: ConditionRow[] }
      const groupConds = group.conditions ?? []
      form.conditions = groupConds.length
        ? groupConds.map((c) => ({ ...createEmptyCondition(), ...c }))
        : [createEmptyCondition()]
      if (group.operator) {
        form.condition_mode = group.operator
      }
    } else {
      form.conditions = [createEmptyCondition()]
    }

    form.template_id = row.template_id ?? ''
    form.destination_ids = (row.destination_ids ?? []) as string[]

    // Dedupe config
    if (row.dedupe_config) {
      form.dedupe_window = row.dedupe_config.window_seconds ?? 0
      form.dedupe_key_fields = row.dedupe_config.key_fields ?? []
      form.dedupe_action = row.dedupe_config.action ?? ''
    }
  } else {
    isEditing.value = false
    editingId.value = ''
  }
  dialogVisible.value = true
}

async function fetchData() {
  loading.value = true
  try {
    const res = await listRules({ page: currentPage.value, size: pageSize.value })
    tableData.value = res.data?.data ?? []
    total.value = res.data?.total ?? 0
  } catch {
    ElMessage.error(t('rules.failedToLoadRules'))
  } finally {
    loading.value = false
  }
}

async function fetchOptions() {
  try {
    const [templatesRes, destinationsRes] = await Promise.all([
      listTemplates({ page: 1, size: 200 }),
      listDestinations({ page: 1, size: 200 }),
    ])
    templateOptions.value = (templatesRes.data?.data ?? []).map(
      (tp: Record<string, unknown>) => ({ id: String(tp.id ?? ''), name: String(tp.name ?? '') })
    )
    destinationOptions.value = (destinationsRes.data?.data ?? []).map(
      (d: Record<string, unknown>) => ({ id: String(d.id ?? ''), name: String(d.name ?? '') })
    )
  } catch {
    // silent
  }
}

async function handleSave() {
  if (!formRef.value) return
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  saving.value = true
  try {
    // Build the ConditionGroup structure that the backend expects:
    // { operator: "and"|"or", conditions: [...] }
    const conditionGroup = {
      operator: form.condition_mode,
      conditions: form.conditions
        .filter((c) => c.field)
        .map((c) => ({
          field: c.field,
          operator: c.operator,
          value: isExistenceOp(c.operator) ? '' : c.value,
        })),
    }

    // Build dedupe config
    const dedupeConfig = form.dedupe_window > 0
      ? {
          window_seconds: form.dedupe_window,
          key_fields: form.dedupe_key_fields,
          action: form.dedupe_action || 'skip',
        }
      : null

    const payload: Record<string, unknown> = {
      name: form.name,
      priority: form.priority,
      enabled: form.enabled,
      condition_mode: form.condition_mode,
      continue_on_match: form.continue_on_match,
      conditions: conditionGroup,
      template_id: form.template_id || null,
      destination_ids: form.destination_ids,
      dedupe_config: dedupeConfig,
    }
    if (isEditing.value) {
      await updateRule(editingId.value, payload)
      ElMessage.success(t('rules.ruleUpdated'))
    } else {
      await createRule(payload)
      ElMessage.success(t('rules.ruleCreated'))
    }
    dialogVisible.value = false
    await fetchData()
  } catch {
    ElMessage.error(t('rules.failedToSaveRule'))
  } finally {
    saving.value = false
  }
}

async function toggleEnabled(row: RuleRow, val: boolean) {
  try {
    await updateRule(row.id, { enabled: val })
    ElMessage.success(val ? t('rules.ruleEnabled') : t('rules.ruleDisabled'))
  } catch {
    row.enabled = !val
    ElMessage.error(t('rules.failedToToggleRule'))
  }
}

async function handleDelete(row: RuleRow) {
  try {
    await ElMessageBox.confirm(
      t('common.confirmDeleteMsg', { entity: t('nav.rules'), name: row.name }),
      t('common.confirmDelete'),
      { type: 'warning' }
    )
    await deleteRule(row.id)
    ElMessage.success(t('rules.ruleDeleted'))
    await fetchData()
  } catch {
    // cancelled
  }
}

onMounted(() => {
  fetchData()
  fetchOptions()
})
</script>

<style lang="scss" scoped>
.rules-container {
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

  .condition-card {
    margin-bottom: 12px;
    padding: 16px;
    background-color: var(--el-fill-color-lighter, #fafafa);
    border: 1px solid var(--el-border-color-lighter, #ebeef5);
    border-radius: 8px;

    .condition-card-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 8px;
    }

    .condition-card-title {
      font-weight: 600;
      font-size: 14px;
      color: var(--el-text-color-primary, #303133);
    }
  }
}
</style>
