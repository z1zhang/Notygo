<template>
  <div class="sources-container">
    <div class="page-actions">
      <el-button type="primary" @click="openDialog()">
        <el-icon><Plus /></el-icon> {{ $t('sources.newSource') }}
      </el-button>
    </div>

    <el-card shadow="never" v-loading="loading">
      <el-table :data="tableData" stripe style="width: 100%">
        <el-table-column prop="name" :label="$t('common.name')" min-width="140" />
        <el-table-column prop="slug" :label="$t('sources.slug')" min-width="260">
          <template #default="{ row }">
            <span class="copyable" @click.stop="copyWebhookUrl(row.slug)">
              {{ row.slug }}
              <el-icon class="copy-icon" :size="12"><CopyDocument /></el-icon>
            </span>
          </template>
        </el-table-column>
        <el-table-column prop="auth_type" :label="$t('sources.authType')" width="120">
          <template #default="{ row }">
            <el-tag>{{ $t('enums.authType.' + (row.auth_type || 'none')) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="enabled" :label="$t('common.enabled')" width="100">
          <template #default="{ row }">
            <el-switch
              v-model="row.enabled"
              @change="(val: boolean) => toggleEnabled(row, val)"
            />
          </template>
        </el-table-column>
        <el-table-column prop="mapping_id" :label="$t('sources.mapping')" min-width="140">
          <template #default="{ row }">
            {{ row.mapping_id ? lookupName(mappingOptions, row.mapping_id) : '-' }}
          </template>
        </el-table-column>
        <el-table-column :label="$t('common.actions')" width="180" fixed="right">
          <template #default="{ row }">
            <div class="action-buttons">
              <el-button type="primary" @click="openDialog(row)">{{ $t('common.edit') }}</el-button>
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
      :title="isEditing ? $t('sources.editSource') : $t('sources.newSource')"
      width="680px"
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
          <el-input v-model="form.name" :placeholder="$t('common.name')" />
        </el-form-item>
        <el-form-item :label="$t('sources.authType')" prop="auth_type">
          <el-select v-model="form.auth_type" :placeholder="$t('sources.selectAuthType')">
            <el-option :label="$t('sources.none')" value="none" />
            <el-option :label="$t('sources.bearer')" value="bearer" />
            <el-option :label="$t('sources.hmac')" value="hmac" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="form.auth_type !== 'none'" :label="$t('sources.secretToken')" prop="secret">
          <el-input
            v-model="form.secret"
            type="password"
            show-password
            :placeholder="isEditing ? $t('sources.secretEditPlaceholder') : $t('sources.secretPlaceholder')"
          >
            <template #append>
              <el-button @click="generateSecret">{{ $t('sources.generateSecret') }}</el-button>
            </template>
          </el-input>
        </el-form-item>
        <el-form-item :label="$t('sources.maxBodyBytes')" prop="max_body_bytes">
          <el-input-number v-model="form.max_body_bytes" :min="0" :step="128" />
        </el-form-item>
        <el-form-item :label="$t('sources.rateLimit')">
          <div style="display: flex; gap: 16px; width: 100%">
            <div style="flex: 1">
              <div style="font-size: 12px; color: var(--el-text-color-regular); margin-bottom: 4px">{{ $t('sources.rateLimitRate') }}</div>
              <el-input-number
                v-model="form.rate_limit_rate"
                :min="0"
                :step="1"
                controls-position="right"
                style="width: 100%"
              />
            </div>
            <div style="flex: 1">
              <div style="font-size: 12px; color: var(--el-text-color-regular); margin-bottom: 4px">{{ $t('sources.rateLimitBurst') }}</div>
              <el-input-number
                v-model="form.rate_limit_burst"
                :min="0"
                :step="1"
                controls-position="right"
                style="width: 100%"
              />
            </div>
          </div>
          <div style="font-size: 12px; color: var(--el-text-color-placeholder); margin-top: 4px; line-height: 1.4">
            {{ $t('sources.rateLimitHint') }}
          </div>
        </el-form-item>
        <el-form-item :label="$t('sources.mapping')" prop="mapping_id">
          <el-select v-model="form.mapping_id" :placeholder="$t('sources.selectMapping')" clearable>
            <el-option
              v-for="m in mappingOptions"
              :key="m.id"
              :label="m.name"
              :value="m.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('common.enabled')">
          <el-switch v-model="form.enabled" />
        </el-form-item>
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
import { CopyDocument } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'
import { createSource, updateSource, deleteSource, listSources } from '@/api/sources'
import type { SourceResponse } from '@/api/types'
import { getErrorMessage, copyToClipboard } from '@/utils/helpers'
import { lookupName } from '@/utils/lookup'
import { useCrudPage } from '@/composables/useCrudPage'
import { useOptions } from '@/composables/useOptions'

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
  closeDialog,
} = useCrudPage<SourceResponse>({
  listApi: listSources,
  deleteApi: (id) => deleteSource(id),
  entityI18nKey: 'nav.sources',
})

const { mappingOptions, fetchMappingOptions } = useOptions()

const formRef = ref<FormInstance>()
const form = reactive({
  name: '',
  auth_type: 'none',
  secret: '',
  max_body_bytes: 1024, // displayed as KB internally, converted to bytes on save
  rate_limit_rate: 0,
  rate_limit_burst: 0,
  mapping_id: '',
  enabled: true,
})

const formRules: FormRules = {
  name: [{ required: true, message: () => t('sources.nameRequired'), trigger: 'blur' }],
  auth_type: [{ required: true, message: () => t('sources.authTypeRequired'), trigger: 'change' }],
}

function resetForm() {
  form.name = ''
  form.auth_type = 'none'
  form.secret = ''
  form.max_body_bytes = 1024
  form.rate_limit_rate = 0
  form.rate_limit_burst = 0
  form.mapping_id = ''
  form.enabled = true
}

function openDialog(row?: SourceResponse) {
  resetForm()
  if (row) {
    isEditing.value = true
    editingId.value = row.id
    form.name = row.name ?? ''
    form.auth_type = row.auth_type ?? 'none'
    form.secret = ''
    form.max_body_bytes = Math.round((row.max_body_bytes ?? 1048576) / 1024)
    // Parse rate_limit JSON string into separate rate/burst fields
    if (row.rate_limit) {
      try {
        const rl = JSON.parse(row.rate_limit)
        form.rate_limit_rate = rl.rate ?? 0
        form.rate_limit_burst = rl.burst ?? 0
      } catch {
        form.rate_limit_rate = 0
        form.rate_limit_burst = 0
      }
    } else {
      form.rate_limit_rate = 0
      form.rate_limit_burst = 0
    }
    form.mapping_id = row.mapping_id ?? ''
    form.enabled = row.enabled ?? true
  } else {
    isEditing.value = false
    editingId.value = ''
  }
  dialogVisible.value = true
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
      max_body_bytes: form.max_body_bytes * 1024, // convert KB → bytes
      enabled: form.enabled,
      mapping_id: form.mapping_id || null,
    }

    // Secret: field name must be "secret_ref" (matches backend UpdateSourceRequest)
    // Only include if user entered a new value (editing: leave blank = keep existing)
    if (form.auth_type !== 'none' && form.secret) {
      payload.secret_ref = form.secret
    }

    // Rate limit: assemble rate/burst into JSON string for backend
    if (form.rate_limit_rate > 0 || form.rate_limit_burst > 0) {
      payload.rate_limit = JSON.stringify({
        rate: form.rate_limit_rate || 0,
        burst: form.rate_limit_burst || 0,
      })
    }

    if (isEditing.value) {
      await updateSource(editingId.value, payload)
      ElMessage.success(t('sources.sourceUpdated'))
    } else {
      await createSource(payload)
      ElMessage.success(t('sources.sourceCreated'))
    }
    dialogVisible.value = false
    await fetchData()
  } catch (e: unknown) {
    ElMessage.error(getErrorMessage(e, t('sources.failedToSaveSource')))
  } finally {
    saving.value = false
  }
}

function generateSecret() {
  const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789'
  const length = 32
  const array = new Uint8Array(length)
  crypto.getRandomValues(array)
  let result = ''
  for (let i = 0; i < length; i++) {
    result += chars[array[i] % chars.length]
  }
  form.secret = result
}

async function toggleEnabled(row: SourceResponse, val: boolean) {
  try {
    // Only send the "enabled" field — do NOT spread the entire row
    // (which includes encrypted secret_ref that would get re-encrypted)
    await updateSource(row.id, { enabled: val })
    ElMessage.success(val ? t('sources.sourceEnabled') : t('sources.sourceDisabled'))
  } catch (e: unknown) {
    row.enabled = !val
    ElMessage.error(getErrorMessage(e, t('sources.failedToToggleSource')))
  }
}

function buildWebhookUrl(slug: string): string {
  // In production the frontend and API share the same origin;
  // in dev, Vite proxies /api to the backend, so location.origin works.
  const base = window.location.origin
  return `${base}/api/v1/ingest/${slug}`
}

async function copyWebhookUrl(slug: string) {
  try {
    await copyToClipboard(buildWebhookUrl(slug))
    ElMessage.success(t('common.copied'))
  } catch {
    ElMessage.error(t('common.copyFailed'))
  }
}

onMounted(() => {
  fetchData()
  fetchMappingOptions()
})
</script>

<style lang="scss" scoped>
.copyable {
  cursor: pointer;

  .copy-icon {
    margin-left: 4px;
    color: var(--el-text-color-placeholder, #a8abb2);
    opacity: 0;
    transition: color 0.2s, opacity 0.2s;
  }

  &:hover .copy-icon {
    opacity: 1;
    color: var(--el-color-primary, #409eff);
  }
}
</style>
