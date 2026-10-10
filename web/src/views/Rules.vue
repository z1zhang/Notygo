<template>
  <div class="rules-container">
    <div class="page-actions">
      <el-button type="primary" @click="openDialog()">
        <el-icon><Plus /></el-icon> New Rule
      </el-button>
    </div>

    <el-card shadow="never" v-loading="loading">
      <el-table :data="tableData" stripe style="width: 100%">
        <el-table-column prop="name" label="Name" min-width="160" />
        <el-table-column prop="priority" label="Priority" width="100" />
        <el-table-column prop="enabled" label="Enabled" width="100">
          <template #default="{ row }">
            <el-switch v-model="row.enabled" @change="(val: boolean) => toggleEnabled(row, val)" />
          </template>
        </el-table-column>
        <el-table-column prop="template_id" label="Template" width="140">
          <template #default="{ row }">
            {{ getTemplateName(row.template_id) }}
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
      :title="isEditing ? 'Edit Rule' : 'New Rule'"
      width="720px"
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
          <el-input v-model="form.name" placeholder="Rule name" />
        </el-form-item>
        <el-form-item label="Priority" prop="priority">
          <el-input-number v-model="form.priority" :min="0" :max="9999" />
        </el-form-item>
        <el-form-item label="Enabled">
          <el-switch v-model="form.enabled" />
        </el-form-item>
        <el-form-item label="Condition Mode">
          <el-radio-group v-model="form.condition_mode">
            <el-radio value="and">AND</el-radio>
            <el-radio value="or">OR</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="Continue on Match">
          <el-switch v-model="form.continue_on_match" />
        </el-form-item>

        <el-divider content-position="left">Conditions</el-divider>

        <div v-for="(cond, index) in form.conditions" :key="index" class="condition-row">
          <el-row :gutter="8" align="middle">
            <el-col :span="7">
              <el-form-item
                :prop="`conditions.${index}.field`"
                :rules="[{ required: true, message: 'Required', trigger: 'blur' }]"
                label=""
              >
                <el-input v-model="cond.field" placeholder="Field" />
              </el-form-item>
            </el-col>
            <el-col :span="6">
              <el-form-item :prop="`conditions.${index}.operator`" label="">
                <el-select v-model="cond.operator" placeholder="Operator">
                  <el-option label="Equals" value="eq" />
                  <el-option label="Not Equals" value="neq" />
                  <el-option label="Contains" value="contains" />
                  <el-option label="Starts With" value="starts_with" />
                  <el-option label="Ends With" value="ends_with" />
                  <el-option label="Regex" value="regex" />
                  <el-option label="Greater Than" value="gt" />
                  <el-option label="Less Than" value="lt" />
                  <el-option label="Exists" value="exists" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item :prop="`conditions.${index}.value`" label="">
                <el-input v-model="cond.value" placeholder="Value" />
              </el-form-item>
            </el-col>
            <el-col :span="2">
              <el-button text type="danger" :icon="Delete" @click="removeCondition(index)" />
            </el-col>
          </el-row>
        </div>

        <el-button type="primary" text @click="addCondition">
          <el-icon><Plus /></el-icon> Add Condition
        </el-button>

        <el-divider />

        <el-form-item label="Template" prop="template_id">
          <el-select v-model="form.template_id" placeholder="Select template" clearable>
            <el-option
              v-for="t in templateOptions"
              :key="t.id"
              :label="t.name"
              :value="t.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="Destinations" prop="destination_ids">
          <el-select
            v-model="form.destination_ids"
            multiple
            placeholder="Select destinations"
          >
            <el-option
              v-for="d in destinationOptions"
              :key="d.id"
              :label="d.name"
              :value="d.id"
            />
          </el-select>
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
import { Delete } from '@element-plus/icons-vue'
import { listRules, createRule, updateRule, deleteRule } from '@/api/rules'
import { listTemplates } from '@/api/templates'
import { listDestinations } from '@/api/destinations'

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
})

const formRules: FormRules = {
  name: [{ required: true, message: 'Name is required', trigger: 'blur' }],
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
  return templateOptions.value.find((t) => t.id === id)?.name ?? id
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
    form.conditions = (row.conditions ?? []).length
      ? (row.conditions as ConditionRow[]).map((c) => ({ ...createEmptyCondition(), ...c }))
      : [createEmptyCondition()]
    form.template_id = row.template_id ?? ''
    form.destination_ids = (row.destination_ids ?? []) as string[]
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
    tableData.value = res.data?.items ?? []
    total.value = res.data?.total ?? 0
  } catch (e: unknown) {
    const msg = (e as { response?: { data?: { message?: string } } })?.response?.data?.message || 'Failed to load rules'
    ElMessage.error(msg)
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
    templateOptions.value = (templatesRes.data?.items ?? []).map(
      (t: Record<string, unknown>) => ({ id: String(t.id ?? ''), name: String(t.name ?? '') })
    )
    destinationOptions.value = (destinationsRes.data?.items ?? []).map(
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
    const payload: Record<string, unknown> = {
      name: form.name,
      priority: form.priority,
      enabled: form.enabled,
      condition_mode: form.condition_mode,
      continue_on_match: form.continue_on_match,
      conditions: form.conditions.filter((c) => c.field),
      template_id: form.template_id || null,
      destination_ids: form.destination_ids,
    }
    if (isEditing.value) {
      await updateRule(editingId.value, payload)
      ElMessage.success('Rule updated')
    } else {
      await createRule(payload)
      ElMessage.success('Rule created')
    }
    dialogVisible.value = false
    await fetchData()
  } catch (e: unknown) {
    const msg = (e as { response?: { data?: { message?: string } } })?.response?.data?.message || 'Failed to save rule'
    ElMessage.error(msg)
  } finally {
    saving.value = false
  }
}

async function toggleEnabled(row: RuleRow, val: boolean) {
  try {
    await updateRule(row.id, { ...row, enabled: val })
    ElMessage.success(`Rule ${val ? 'enabled' : 'disabled'}`)
  } catch (e: unknown) {
    row.enabled = !val
    const msg = (e as { response?: { data?: { message?: string } } })?.response?.data?.message || 'Failed to toggle rule'
    ElMessage.error(msg)
  }
}

async function handleDelete(row: RuleRow) {
  try {
    await ElMessageBox.confirm(
      `Are you sure you want to delete rule "${row.name}"?`,
      'Confirm Delete',
      { type: 'warning' }
    )
    await deleteRule(row.id)
    ElMessage.success('Rule deleted')
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

  .condition-row {
    margin-bottom: 4px;
    padding: 8px;
    background-color: #fafafa;
    border-radius: 4px;
  }
}
</style>
