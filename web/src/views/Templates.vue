<template>
  <div class="templates-container">
    <div class="page-actions">
      <el-button type="primary" @click="openDialog()">
        <el-icon><Plus /></el-icon> {{ $t('templates.newTemplate') }}
      </el-button>
    </div>

    <el-card shadow="never" v-loading="loading">
      <el-table :data="tableData" stripe style="width: 100%">
        <el-table-column prop="name" :label="$t('common.name')" min-width="160" />
        <el-table-column prop="format" :label="$t('templates.format')" width="120">
          <template #default="{ row }">
            <el-tag size="small">{{ $t('enums.templateFormat.' + row.format, row.format) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="revision" :label="$t('mappings.revision')" width="100" />
        <el-table-column :label="$t('common.actions')" width="200" fixed="right">
          <template #default="{ row }">
            <el-button text type="primary" size="small" @click="openDialog(row)">{{ $t('common.edit') }}</el-button>
            <el-button text type="info" size="small" @click="openPreview(row)">{{ $t('templates.preview') }}</el-button>
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
      :title="isEditing ? $t('templates.editTemplate') : $t('templates.newTemplate')"
      width="720px"
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
          <el-input v-model="form.name" :placeholder="$t('templates.newTemplate')" />
        </el-form-item>
        <el-form-item :label="$t('templates.format')" prop="format">
          <el-select v-model="form.format" :placeholder="$t('templates.selectFormat')">
            <el-option :label="$t('templates.text')" value="text" />
            <el-option :label="$t('templates.markdown')" value="markdown" />
            <el-option :label="$t('templates.html')" value="html" />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('templates.titleTemplate')" prop="title_template">
          <el-input
            v-model="form.title_template"
            type="textarea"
            :rows="3"
            :placeholder="$t('templates.titlePlaceholder')"
          />
        </el-form-item>
        <el-form-item :label="$t('templates.bodyTemplate')" prop="body_template">
          <el-input
            v-model="form.body_template"
            type="textarea"
            :rows="10"
            :placeholder="$t('templates.bodyPlaceholder')"
            class="monospace-input"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="saving" @click="handleSave">{{ $t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="previewVisible"
      :title="$t('templates.templatePreview')"
      width="640px"
      destroy-on-close
    >
      <div v-loading="previewLoading">
        <el-form label-width="auto" label-position="left" class="preview-form">
          <el-form-item :label="$t('templates.sampleData')">
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
          <h4>{{ $t('templates.title') }}</h4>
          <div class="preview-output preview-title">{{ previewResult.title ?? '' }}</div>
          <h4 style="margin-top: 12px">{{ $t('templates.body') }}</h4>
          <!-- Rendered body: HTML for html/markdown, plain <pre> for text -->
          <div v-if="previewFormatType === 'html' || previewFormatType === 'markdown'" class="preview-output preview-body-rich" v-html="renderedPreviewBody" />
          <pre v-else class="preview-output">{{ previewResult.body ?? '' }}</pre>
        </div>
      </div>
      <template #footer>
        <el-button @click="previewVisible = false">{{ $t('common.close') }}</el-button>
        <el-button type="primary" :loading="previewLoading" @click="runPreview">{{ $t('templates.render') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import type { FormInstance, FormRules } from 'element-plus'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import {
  listTemplates,
  createTemplate,
  updateTemplate,
  deleteTemplate,
  previewTemplate,
} from '@/api/templates'

const { t } = useI18n()

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
const previewFormat = ref('text')
const previewSampleData = ref('{\n  "event": {\n    "type": "alert",\n    "message": "Test notification"\n  }\n}')
const previewResult = ref<{ title: string; body: string; format: string } | null>(null)

// Render preview body based on template format
const renderedPreviewBody = computed(() => {
  if (!previewResult.value) return ''
  const { body, format } = previewResult.value
  if (format === 'html') return body
  if (format === 'markdown') return marked(body) as string
  return ''
})

const previewFormatType = computed(() => previewResult.value?.format ?? previewFormat.value)

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
  name: [{ required: true, message: t('templates.nameRequired'), trigger: 'blur' }],
  format: [{ required: true, message: t('templates.formatRequired'), trigger: 'change' }],
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
  previewFormat.value = row.format ?? 'text'
  previewSampleData.value = '{\n  "event": {\n    "type": "alert",\n    "message": "Test notification"\n  }\n}'
  previewResult.value = null
  previewVisible.value = true
}

async function fetchData() {
  loading.value = true
  try {
    const res = await listTemplates({ page: currentPage.value, size: pageSize.value })
    tableData.value = res.data?.data ?? []
    total.value = res.data?.total ?? 0
  } catch {
    ElMessage.error(t('templates.failedToLoadTemplates'))
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
      ElMessage.success(t('templates.templateUpdated'))
    } else {
      await createTemplate(payload)
      ElMessage.success(t('templates.templateCreated'))
    }
    dialogVisible.value = false
    await fetchData()
  } catch {
    ElMessage.error(t('templates.failedToSaveTemplate'))
  } finally {
    saving.value = false
  }
}

async function handleDelete(row: TemplateRow) {
  try {
    await ElMessageBox.confirm(
      t('common.confirmDeleteMsg', { entity: t('nav.templates'), name: row.name }),
      t('common.confirmDelete'),
      { type: 'warning' }
    )
    await deleteTemplate(row.id)
    ElMessage.success(t('templates.templateDeleted'))
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
      ElMessage.error(t('templates.invalidJson'))
      return
    }
    const res = await previewTemplate({
      template_id: previewTemplateId.value,
      sample_data: sampleData,
    })
    previewResult.value = {
      title: res.data?.title ?? '',
      body: res.data?.body ?? '',
      format: res.data?.format ?? previewFormat.value,
    }
  } catch {
    ElMessage.error(t('templates.previewFailed'))
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
    font-size: 14px;
    white-space: pre-wrap;
    word-break: break-all;
    margin: 0;
  }

  .preview-title {
    font-family: 'Courier New', Courier, monospace;
    font-size: 13px;
  }

  .preview-body-rich {
    // Rich rendered content (HTML/Markdown)
    white-space: normal;
    word-break: break-word;
    line-height: 1.6;

    // Markdown styling
    :deep(h1), :deep(h2), :deep(h3), :deep(h4), :deep(h5), :deep(h6) {
      margin-top: 8px;
      margin-bottom: 4px;
    }

    :deep(p) {
      margin: 4px 0;
    }

    :deep(code) {
      background-color: rgba(0, 0, 0, 0.06);
      padding: 2px 6px;
      border-radius: 3px;
      font-family: 'Courier New', Courier, monospace;
      font-size: 13px;
    }

    :deep(pre) {
      background-color: rgba(0, 0, 0, 0.04);
      padding: 8px 12px;
      border-radius: 4px;
      overflow-x: auto;

      code {
        background: none;
        padding: 0;
      }
    }

    :deep(blockquote) {
      border-left: 4px solid #d0d0d0;
      padding-left: 12px;
      margin: 8px 0;
      color: #666;
    }

    :deep(ul), :deep(ol) {
      padding-left: 20px;
    }

    :deep(table) {
      border-collapse: collapse;
      margin: 8px 0;

      th, td {
        border: 1px solid #ddd;
        padding: 6px 12px;
      }

      th {
        background-color: #f0f0f0;
      }
    }

    :deep(img) {
      max-width: 100%;
    }

    :deep(a) {
      color: #409eff;
    }
  }
}
</style>
