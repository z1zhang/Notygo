<template>
  <div class="templates-container">
    <div class="page-actions">
      <el-button type="primary" @click="openDialog()">
        <el-icon><Plus /></el-icon> New Template
      </el-button>
    </div>

    <el-card shadow="never" v-loading="loading">
      <el-table :data="tableData" stripe style="width: 100%">
        <el-table-column prop="name" label="Name" min-width="160" />
        <el-table-column prop="format" label="Format" width="120">
          <template #default="{ row }">
            <el-tag size="small">{{ row.format }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="revision" label="Revision" width="100" />
        <el-table-column label="Actions" width="200" fixed="right">
          <template #default="{ row }">
            <el-button text type="primary" size="small" @click="openDialog(row)">Edit</el-button>
            <el-button text type="info" size="small" @click="openPreview(row)">Preview</el-button>
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
      :title="isEditing ? 'Edit Template' : 'New Template'"
      width="640px"
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
          <el-input v-model="form.name" placeholder="Template name" />
        </el-form-item>
        <el-form-item label="Format" prop="format">
          <el-select v-model="form.format" placeholder="Select format">
            <el-option label="Text" value="text" />
            <el-option label="Markdown" value="markdown" />
            <el-option label="HTML" value="html" />
          </el-select>
        </el-form-item>
        <el-form-item label="Title Template" prop="title_template">
          <el-input
            v-model="form.title_template"
            type="textarea"
            :rows="3"
            placeholder="Title template using Go syntax, e.g. {{ .event.type }}"
          />
        </el-form-item>
        <el-form-item label="Body Template" prop="body_template">
          <el-input
            v-model="form.body_template"
            type="textarea"
            :rows="10"
            placeholder="Body template"
            class="monospace-input"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">Cancel</el-button>
        <el-button type="primary" :loading="saving" @click="handleSave">Save</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="previewVisible"
      title="Template Preview"
      width="640px"
      destroy-on-close
    >
      <div v-loading="previewLoading">
        <el-form label-width="100px" label-position="right" class="preview-form">
          <el-form-item label="Sample Data">
            <el-input
              v-model="previewSampleData"
              type="textarea"
              :rows="4"
              placeholder='{"event": {"type": "alert", "message": "test"}}'
              class="monospace-input"
            />
          </el-form-item>
        </el-form>
        <el-divider />
        <div v-if="previewResult">
          <h4>Title</h4>
          <pre class="preview-output">{{ previewResult.title ?? '' }}</pre>
          <h4 style="margin-top: 12px">Body</h4>
          <pre class="preview-output">{{ previewResult.body ?? '' }}</pre>
        </div>
      </div>
      <template #footer>
        <el-button @click="previewVisible = false">Close</el-button>
        <el-button type="primary" :loading="previewLoading" @click="runPreview">Render</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import type { FormInstance, FormRules } from 'element-plus'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  listTemplates,
  createTemplate,
  updateTemplate,
  deleteTemplate,
  previewTemplate,
} from '@/api/templates'

interface TemplateRow {
  id: string
  name: string
  format: string
  revision: number
  title_template: string
  body_template: string
  [key: string]: unknown
}

const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const isEditing = ref(false)
const editingId = ref('')

const previewVisible = ref(false)
const previewLoading = ref(false)
const previewTemplateId = ref('')
const previewSampleData = ref('{\n  "event": {\n    "type": "alert",\n    "message": "Test notification"\n  }\n}')
const previewResult = ref<{ title: string; body: string } | null>(null)

const tableData = ref<TemplateRow[]>([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(20)

const formRef = ref<FormInstance>()
const form = reactive({
  name: '',
  format: 'text',
  title_template: '',
  body_template: '',
})

const formRules: FormRules = {
  name: [{ required: true, message: 'Name is required', trigger: 'blur' }],
  format: [{ required: true, message: 'Format is required', trigger: 'change' }],
}

function resetForm() {
  form.name = ''
  form.format = 'text'
  form.title_template = ''
  form.body_template = ''
}

function openDialog(row?: TemplateRow) {
  resetForm()
  if (row) {
    isEditing.value = true
    editingId.value = row.id
    form.name = row.name ?? ''
    form.format = row.format ?? 'text'
    form.title_template = row.title_template ?? ''
    form.body_template = row.body_template ?? ''
  } else {
    isEditing.value = false
    editingId.value = ''
  }
  dialogVisible.value = true
}

function openPreview(row: TemplateRow) {
  previewTemplateId.value = row.id
  previewSampleData.value = '{\n  "event": {\n    "type": "alert",\n    "message": "Test notification"\n  }\n}'
  previewResult.value = null
  previewVisible.value = true
}

async function fetchData() {
  loading.value = true
  try {
    const res = await listTemplates({ page: currentPage.value, size: pageSize.value })
    tableData.value = res.data?.items ?? []
    total.value = res.data?.total ?? 0
  } catch (e: unknown) {
    const msg = (e as { response?: { data?: { message?: string } } })?.response?.data?.message || 'Failed to load templates'
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
      format: form.format,
      title_template: form.title_template,
      body_template: form.body_template,
    }
    if (isEditing.value) {
      await updateTemplate(editingId.value, payload)
      ElMessage.success('Template updated')
    } else {
      await createTemplate(payload)
      ElMessage.success('Template created')
    }
    dialogVisible.value = false
    await fetchData()
  } catch (e: unknown) {
    const msg = (e as { response?: { data?: { message?: string } } })?.response?.data?.message || 'Failed to save template'
    ElMessage.error(msg)
  } finally {
    saving.value = false
  }
}

async function handleDelete(row: TemplateRow) {
  try {
    await ElMessageBox.confirm(
      `Are you sure you want to delete template "${row.name}"?`,
      'Confirm Delete',
      { type: 'warning' }
    )
    await deleteTemplate(row.id)
    ElMessage.success('Template deleted')
    await fetchData()
  } catch {
    // cancelled
  }
}

async function runPreview() {
  previewLoading.value = true
  previewResult.value = null
  try {
    let sampleData: Record<string, unknown>
    try {
      sampleData = JSON.parse(previewSampleData.value)
    } catch {
      ElMessage.error('Invalid JSON in sample data')
      return
    }
    const res = await previewTemplate({
      template_id: previewTemplateId.value,
      sample_data: sampleData,
    })
    previewResult.value = {
      title: res.data?.title ?? '',
      body: res.data?.body ?? '',
    }
  } catch (e: unknown) {
    const msg = (e as { response?: { data?: { message?: string } } })?.response?.data?.message || 'Preview failed'
    ElMessage.error(msg)
  } finally {
    previewLoading.value = false
  }
}

onMounted(() => {
  fetchData()
})
</script>

<style lang="scss" scoped>
.templates-container {
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

  :deep(.monospace-input .el-textarea__inner) {
    font-family: 'Courier New', Courier, monospace;
    font-size: 13px;
  }

  .preview-form {
    margin-bottom: 8px;
  }

  .preview-output {
    background-color: #f5f7fa;
    padding: 12px;
    border-radius: 4px;
    font-family: 'Courier New', Courier, monospace;
    font-size: 13px;
    white-space: pre-wrap;
    word-break: break-all;
    margin: 0;
  }
}
</style>
