<template>
  <div class="playground-container">
    <!-- Section 1: Raw Payload -->
    <el-card shadow="never" class="section-card">
      <template #header>
        <div class="section-header">
          <span class="section-title">{{ $t('playground.payload') }}</span>
          <div v-if="structureInfo" class="structure-badges">
            <el-tag :type="structureInfo.root_is_array ? 'warning' : 'success'" size="small">
              {{ structureInfo.root_is_array ? $t('playground.rootArray') : $t('playground.rootObject') }}
            </el-tag>
            <el-tag type="info" size="small">
              {{ $t('playground.itemCount') }}: {{ structureInfo.item_count }}
            </el-tag>
          </div>
        </div>
      </template>
      <el-input
        v-model="rawPayload"
        type="textarea"
        :rows="8"
        :placeholder="$t('playground.payloadPlaceholder')"
        spellcheck="false"
      />
      <div class="section-actions">
        <el-button type="primary" :loading="parseLoading" @click="handleAnalyze">
          <el-icon><Search /></el-icon> {{ $t('playground.analyzeJson') }}
        </el-button>
      </div>
    </el-card>

    <!-- Section 2: Structure Tree + Mapping Fields + Extraction Results -->
    <el-card v-if="structureInfo" shadow="never" class="section-card">
      <template #header>
        <span class="section-title">{{ $t('playground.mappingFields') }} / {{ $t('playground.extractionResults') }}</span>
      </template>

      <el-row :gutter="20">
        <!-- Left: Structure Tree -->
        <el-col :span="10">
          <div class="subsection-header">
            <span>{{ $t('playground.structure') }}</span>
          </div>
          <div class="tree-container">
            <div v-for="item in suggestedFields" :key="item.source_path + item.multi_result_mode" class="tree-leaf">
              <div
                class="tree-leaf__row"
                :class="{ 'is-added': addedPaths.has(item.source_path + (item.multi_result_mode || '')) }"
              >
                <span class="tree-leaf__path">{{ item.source_path }}</span>
                <el-tag :type="valueTypeTag(item.value_type)" size="small">{{ valueTypeLabel(item.value_type) }}</el-tag>
                <span class="tree-leaf__sample">{{ item.sample_value }}</span>
                <el-button
                  v-if="!addedPaths.has(item.source_path + (item.multi_result_mode || ''))"
                  type="primary"
                  text
                  size="small"
                  @click="addFieldFromSuggestion(item)"
                >
                  <el-icon><Plus /></el-icon>
                </el-button>
                <el-tag v-else type="success" size="small">✓</el-tag>
              </div>
            </div>
          </div>
        </el-col>

        <!-- Right: Mapping Fields -->
        <el-col :span="14">
          <div class="subsection-header">
            <span>{{ $t('playground.mappingFields') }} ({{ mappingFields.length }})</span>
            <el-button type="primary" text size="small" @click="addField">
              <el-icon><Plus /></el-icon> {{ $t('playground.addField') }}
            </el-button>
          </div>

          <div v-for="(field, index) in mappingFields" :key="index" class="field-card">
            <div class="field-card-header">
              <span class="field-card-title">{{ $t('mappings.fieldNo', { n: index + 1 }) }}</span>
              <el-button text type="danger" size="small" @click="removeField(index)">
                <el-icon><Delete /></el-icon> {{ $t('common.remove') }}
              </el-button>
            </div>

            <el-row :gutter="16">
              <el-col :span="12">
                <el-form-item :label="$t('mappings.sourcePath')" label-width="auto">
                  <el-input v-model="field.source_path" :placeholder="$t('mappings.sourcePathPlaceholder')" />
                </el-form-item>
              </el-col>
              <el-col :span="12">
                <el-form-item :label="$t('mappings.targetKey')" label-width="auto">
                  <el-input v-model="field.target_key" :placeholder="$t('mappings.targetKeyPlaceholder')" />
                </el-form-item>
              </el-col>
            </el-row>

            <el-row :gutter="16">
              <el-col :span="8">
                <el-form-item :label="$t('mappings.valueType')" label-width="auto">
                  <el-select v-model="field.value_type" style="width: 100%">
                    <el-option :label="$t('mappings.string')" value="string" />
                    <el-option :label="$t('mappings.number')" value="number" />
                    <el-option :label="$t('mappings.bool')" value="boolean" />
                    <el-option :label="$t('mappings.json')" value="json" />
                    <el-option :label="$t('mappings.array')" value="array" />
                  </el-select>
                </el-form-item>
              </el-col>
              <el-col :span="8">
                <el-form-item :label="$t('mappings.multiResult')" label-width="auto">
                  <el-select v-model="field.multi_result_mode" style="width: 100%" clearable>
                    <el-option :label="$t('mappings.first')" value="first" />
                    <el-option :label="$t('mappings.joinComma')" value="join(, )" />
                    <el-option :label="$t('mappings.array')" value="array" />
                    <el-option :label="$t('mappings.count')" value="count" />
                  </el-select>
                </el-form-item>
              </el-col>
              <el-col :span="8">
                <el-form-item :label="$t('mappings.default')" label-width="auto">
                  <el-input v-model="field.default_value" :placeholder="$t('mappings.optionalDefault')" />
                </el-form-item>
              </el-col>
            </el-row>

            <div v-if="field._sample_value" class="sample-value">
              <el-tag type="info" size="small">{{ $t('playground.sampleValue') }}: {{ field._sample_value }}</el-tag>
            </div>
          </div>

          <div v-if="mappingFields.length === 0" class="empty-hint">
            {{ $t('playground.clickToAdd') }}
          </div>
        </el-col>
      </el-row>

      <div class="section-actions">
        <el-button type="primary" :loading="mappingLoading" @click="handleApplyMapping" :disabled="mappingFields.length === 0">
          <el-icon><Connection /></el-icon> {{ $t('playground.applyMapping') }}
        </el-button>
        <el-button type="success" :loading="createMappingLoading" :disabled="mappingFields.length === 0" @click="handleCreateMapping">
          <el-icon><CirclePlus /></el-icon> {{ $t('playground.createMapping') }}
        </el-button>
      </div>

      <!-- Extraction Results -->
      <el-divider v-if="mappingResult || mappingValidationErrors.length > 0" content-position="left">
        {{ $t('playground.extractionResults') }}
      </el-divider>

      <div v-if="mappingValidationErrors.length > 0" class="validation-errors">
        <el-alert
          v-for="(err, i) in mappingValidationErrors"
          :key="i"
          :title="`${err.field}: ${err.code}`"
          :description="err.detail"
          type="warning"
          :closable="false"
          show-icon
          style="margin-bottom: 8px"
        />
      </div>

      <div v-if="mappingResult" class="result-box">
        <el-input
          :model-value="formatJson(mappingResult)"
          type="textarea"
          :rows="12"
          readonly
          spellcheck="false"
        />
      </div>
    </el-card>

    <!-- Section 3: Template Editor + Render Preview -->
    <el-card shadow="never" class="section-card">
      <template #header>
        <span class="section-title">{{ $t('playground.templateEditor') }} / {{ $t('playground.renderResult') }}</span>
      </template>

      <el-row :gutter="20">
        <!-- Left: Template Editor -->
        <el-col :span="12">
          <div class="subsection-header">
            <span>{{ $t('playground.templateEditor') }}</span>
          </div>

          <el-form label-width="90px" label-position="left">
            <el-form-item :label="$t('templates.format')">
              <el-select v-model="templateForm.format" style="width: 100%">
                <el-option :label="$t('templates.text')" value="text" />
                <el-option :label="$t('templates.markdown')" value="markdown" />
                <el-option :label="$t('templates.html')" value="html" />
              </el-select>
            </el-form-item>
            <el-form-item :label="$t('templates.titleTemplate')">
              <el-input
                v-model="templateForm.title_template"
                type="textarea"
                :rows="3"
                :placeholder="$t('templates.titlePlaceholder')"
              />
            </el-form-item>
            <el-form-item :label="$t('templates.bodyTemplate')">
              <el-input
                v-model="templateForm.body_template"
                type="textarea"
                :rows="10"
                :placeholder="$t('templates.bodyPlaceholder')"
              />
            </el-form-item>
          </el-form>
        </el-col>

        <!-- Right: Render Preview -->
        <el-col :span="12">
          <div class="subsection-header">
            <span>{{ $t('playground.renderResult') }}</span>
          </div>

          <div v-if="deliveryValidationErrors.length > 0" class="validation-errors">
            <el-alert
              v-for="(err, i) in deliveryValidationErrors"
              :key="i"
              :title="`${err.field}: ${err.code}`"
              :description="err.detail"
              type="warning"
              :closable="false"
              show-icon
              style="margin-bottom: 8px"
            />
          </div>

          <div v-if="deliveryResult">
            <h4>{{ $t('templates.title') }}</h4>
            <div class="preview-output preview-title">{{ deliveryResult.rendered_title }}</div>

            <h4 style="margin-top: 12px">{{ $t('templates.body') }}</h4>
            <div v-if="deliveryResult.format === 'html' || deliveryResult.format === 'markdown'" class="preview-output preview-body-rich" v-html="renderedDeliveryBody" />
            <pre v-else class="preview-output">{{ deliveryResult.rendered_body }}</pre>
          </div>
          <div v-else class="empty-hint">
            {{ $t('playground.noResults') }}
          </div>
        </el-col>
      </el-row>

      <div class="section-actions">
        <el-button type="primary" :loading="deliveryLoading" @click="handleRenderPreview" :disabled="mappingFields.length === 0">
          <el-icon><View /></el-icon> {{ $t('playground.renderPreview') }}
        </el-button>
        <el-button type="success" :loading="deliveryLoading" @click="handleFullPipeline" :disabled="mappingFields.length === 0">
          <el-icon><MagicStick /></el-icon> {{ $t('playground.fullPipeline') }}
        </el-button>
        <el-button type="warning" :loading="createTemplateLoading" :disabled="!templateForm.title_template && !templateForm.body_template" @click="handleCreateTemplate">
          <el-icon><CirclePlus /></el-icon> {{ $t('playground.createTemplate') }}
        </el-button>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import { Search, Plus, Delete, Connection, View, MagicStick, CirclePlus } from '@element-plus/icons-vue'
import { previewParse, previewMapping, previewDelivery } from '@/api/preview'
import { createMapping } from '@/api/mappings'
import { createTemplate } from '@/api/templates'
import { getErrorMessage } from '@/utils/helpers'
import type { MappingEntry } from '@/api/types'
import type { StructureInfo, ValidationError, SuggestedField } from '@/api/preview'

const { t } = useI18n()

// ── Section 1: Raw Payload ──────────────────────────────

const rawPayload = ref('')
const parseLoading = ref(false)
const structureInfo = ref<StructureInfo | null>(null)
const suggestedFields = ref<SuggestedField[]>([])

// ── Section 2: Mapping Fields ────────────────────────────

interface FieldRow {
  source_path: string
  target_key: string
  value_type: string
  multi_result_mode: string | null
  default_value: string
  _sample_value: string
}

const mappingFields = ref<FieldRow[]>([])
const mappingLoading = ref(false)
const mappingResult = ref<Record<string, unknown> | null>(null)
const mappingValidationErrors = ref<ValidationError[]>([])
const createMappingLoading = ref(false)

// Track which source_path+mode combos are already added
const addedPaths = computed(() => {
  const s = new Set<string>()
  for (const f of mappingFields.value) {
    s.add(f.source_path + (f.multi_result_mode || ''))
  }
  return s
})

// ── Section 3: Template / Delivery ──────────────────────

const templateForm = reactive({
  format: 'markdown',
  title_template: '',
  body_template: '',
})

const deliveryLoading = ref(false)
const deliveryResult = ref<{ rendered_title: string; rendered_body: string; format: string } | null>(null)
const deliveryValidationErrors = ref<ValidationError[]>([])
const createTemplateLoading = ref(false)

const renderedDeliveryBody = computed(() => {
  if (!deliveryResult.value) return ''
  const { rendered_body, format } = deliveryResult.value
  if (format === 'html') return rendered_body
  if (format === 'markdown') return marked(rendered_body) as string
  return ''
})

// ── Helpers ─────────────────────────────────────────────

function formatJson(obj: Record<string, unknown>): string {
  try {
    return JSON.stringify(obj, null, 2)
  } catch {
    return String(obj)
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

function valueTypeLabel(vt: string): string {
  switch (vt) {
    case 'string': return t('mappings.string')
    case 'number': return t('mappings.number')
    case 'boolean': return t('mappings.bool')
    case 'json': return t('mappings.json')
    case 'array': return t('mappings.array')
    default: return vt
  }
}

function createEmptyField(): FieldRow {
  return {
    source_path: '',
    target_key: '',
    value_type: 'string',
    multi_result_mode: null,
    default_value: '',
    _sample_value: '',
  }
}

function addField() {
  mappingFields.value.push(createEmptyField())
}

function removeField(index: number) {
  mappingFields.value.splice(index, 1)
}

function addFieldFromSuggestion(sf: SuggestedField) {
  mappingFields.value.push({
    source_path: sf.source_path,
    target_key: sf.target_key,
    value_type: sf.value_type || 'string',
    multi_result_mode: sf.multi_result_mode || null,
    default_value: '',
    _sample_value: sf.sample_value || '',
  })
}

function buildMappingEntries(): MappingEntry[] {
  return mappingFields.value
    .filter((f) => f.source_path || f.target_key)
    .map((f) => ({
      source_path: f.source_path,
      target_key: f.target_key,
      value_type: f.value_type || 'string',
      multi_result_mode: f.multi_result_mode || null,
      default_value: f.default_value || null,
      required: false,
    }))
}

// ── Actions ─────────────────────────────────────────────

async function handleAnalyze() {
  if (!rawPayload.value.trim()) {
    ElMessage.warning(t('playground.payload'))
    return
  }
  // Validate JSON first
  try {
    JSON.parse(rawPayload.value)
  } catch {
    ElMessage.error(t('templates.invalidJson'))
    return
  }

  parseLoading.value = true
  structureInfo.value = null
  suggestedFields.value = []
  try {
    const res = await previewParse(rawPayload.value)
    structureInfo.value = res.data.structure
    suggestedFields.value = res.data.suggested_fields || []

    // Clear previous results
    mappingResult.value = null
    mappingValidationErrors.value = []
    deliveryResult.value = null
    deliveryValidationErrors.value = []

    ElMessage.success(t('playground.structure'))
  } catch (e: unknown) {
    ElMessage.error(getErrorMessage(e, t('common.error')))
  } finally {
    parseLoading.value = false
  }
}

async function handleApplyMapping() {
  if (!rawPayload.value.trim()) {
    ElMessage.warning(t('playground.payload'))
    return
  }
  const fields = buildMappingEntries()
  if (fields.length === 0) {
    ElMessage.warning(t('playground.mappingFields'))
    return
  }

  mappingLoading.value = true
  mappingResult.value = null
  mappingValidationErrors.value = []
  try {
    const res = await previewMapping(rawPayload.value, fields)
    mappingResult.value = res.data.parsed_fields
    mappingValidationErrors.value = res.data.validation_errors ?? []
  } catch (e: unknown) {
    ElMessage.error(getErrorMessage(e, t('common.error')))
  } finally {
    mappingLoading.value = false
  }
}

async function handleRenderPreview() {
  if (!rawPayload.value.trim()) {
    ElMessage.warning(t('playground.payload'))
    return
  }
  const fields = buildMappingEntries()
  if (fields.length === 0) {
    ElMessage.warning(t('playground.mappingFields'))
    return
  }

  deliveryLoading.value = true
  deliveryResult.value = null
  deliveryValidationErrors.value = []
  try {
    const res = await previewDelivery(rawPayload.value, fields, {
      title_template: templateForm.title_template,
      body_template: templateForm.body_template,
      format: templateForm.format,
    })
    deliveryResult.value = {
      rendered_title: res.data.rendered_title,
      rendered_body: res.data.rendered_body,
      format: res.data.format,
    }
    deliveryValidationErrors.value = res.data.validation_errors ?? []
  } catch (e: unknown) {
    ElMessage.error(getErrorMessage(e, t('common.error')))
  } finally {
    deliveryLoading.value = false
  }
}

async function handleFullPipeline() {
  await handleApplyMapping()
  if (mappingResult.value) {
    await handleRenderPreview()
  }
}

// ── Quick Create ────────────────────────────────────────

async function handleCreateMapping() {
  const fields = buildMappingEntries()
  if (fields.length === 0) {
    ElMessage.warning(t('playground.mappingFields'))
    return
  }

  try {
    const { value } = await ElMessageBox.prompt(
      t('playground.mappingNamePrompt'),
      t('playground.createMapping'),
      {
        confirmButtonText: t('common.confirm'),
        cancelButtonText: t('common.cancel'),
        inputPattern: /\S+/,
        inputErrorMessage: t('mappings.nameRequired'),
      },
    )

    createMappingLoading.value = true
    await createMapping({ name: value, fields })
    ElMessage.success(t('playground.mappingCreated'))
  } catch (e: unknown) {
    // User cancelled or API error
    if (e !== 'cancel' && e !== 'close') {
      ElMessage.error(getErrorMessage(e, t('common.error')))
    }
  } finally {
    createMappingLoading.value = false
  }
}

async function handleCreateTemplate() {
  if (!templateForm.title_template && !templateForm.body_template) {
    ElMessage.warning(t('playground.templateEditor'))
    return
  }

  try {
    const { value } = await ElMessageBox.prompt(
      t('playground.templateNamePrompt'),
      t('playground.createTemplate'),
      {
        confirmButtonText: t('common.confirm'),
        cancelButtonText: t('common.cancel'),
        inputPattern: /\S+/,
        inputErrorMessage: t('templates.nameRequired'),
      },
    )

    createTemplateLoading.value = true
    await createTemplate({
      name: value,
      format: templateForm.format,
      title_template: templateForm.title_template,
      body_template: templateForm.body_template,
    })
    ElMessage.success(t('playground.templateCreated'))
  } catch (e: unknown) {
    // User cancelled or API error
    if (e !== 'cancel' && e !== 'close') {
      ElMessage.error(getErrorMessage(e, t('common.error')))
    }
  } finally {
    createTemplateLoading.value = false
  }
}
</script>

<style lang="scss" scoped>
.playground-container {

  .section-card {
    margin-bottom: 16px;
  }

  .section-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .section-title {
    font-weight: 600;
    font-size: 15px;
    color: var(--el-text-color-primary, #303133);
  }

  .structure-badges {
    display: flex;
    gap: 8px;
  }

  .section-actions {
    margin-top: 12px;
    display: flex;
    gap: 8px;
  }

  .subsection-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 12px;
    font-weight: 600;
    font-size: 14px;
    color: var(--el-text-color-primary, #303133);
  }

  // ── Structure Tree (flat list of suggested fields) ──
  .tree-container {
    max-height: 500px;
    overflow-y: auto;
    border: 1px solid var(--el-border-color-lighter, #ebeef5);
    border-radius: 4px;
    background-color: var(--el-fill-color-lighter, #fafafa);
  }

  .tree-leaf {
    &__row {
      display: flex;
      align-items: center;
      padding: 6px 10px;
      gap: 8px;
      font-size: 13px;
      border-bottom: 1px solid var(--el-border-color-extra-light, #f2f2f2);

      &:hover {
        background-color: var(--el-fill-color, #f0f0f0);
      }

      &.is-added {
        opacity: 0.5;
      }
    }

    &__path {
      font-weight: 500;
      color: var(--el-color-primary, #409eff);
      min-width: 120px;
      flex-shrink: 0;
    }

    &__sample {
      color: var(--el-text-color-regular, #606266);
      font-size: 12px;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      flex: 1;
      min-width: 0;
    }
  }

  // ── Field cards ──
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

    .sample-value {
      margin-top: 4px;
    }
  }

  .validation-errors {
    margin-bottom: 12px;
  }

  .result-box {
    :deep(.el-textarea__inner) {
      font-family: inherit;
    }
  }

  .empty-hint {
    color: var(--el-text-color-placeholder, #a8abb2);
    font-size: 14px;
    padding: 40px 0;
    text-align: center;
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
    font-size: 13px;
  }

  .preview-body-rich {
    white-space: normal;
    word-break: break-word;
    line-height: 1.6;

    :deep(h1), :deep(h2), :deep(h3), :deep(h4), :deep(h5), :deep(h6) {
      margin-top: 8px;
      margin-bottom: 4px;
    }

    :deep(p) { margin: 4px 0; }

    :deep(code) {
      background-color: rgba(0, 0, 0, 0.06);
      padding: 2px 6px;
      border-radius: 3px;
      font-size: 13px;
    }

    :deep(pre) {
      background-color: rgba(0, 0, 0, 0.04);
      padding: 8px 12px;
      border-radius: 4px;
      overflow-x: auto;

      code { background: none; padding: 0; }
    }

    :deep(blockquote) {
      border-left: 4px solid #d0d0d0;
      padding-left: 12px;
      margin: 8px 0;
      color: #666;
    }

    :deep(ul), :deep(ol) { padding-left: 20px; }

    :deep(table) {
      border-collapse: collapse;
      margin: 8px 0;
      th, td { border: 1px solid #ddd; padding: 6px 12px; }
      th { background-color: #f0f0f0; }
    }

    :deep(img) { max-width: 100%; }
    :deep(a) { color: #409eff; }
  }
}
</style>
