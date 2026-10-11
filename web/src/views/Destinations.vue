<template>
  <div class="destinations-container">
    <div class="page-actions">
      <el-button type="primary" @click="openDialog()">
        <el-icon><Plus /></el-icon> {{ $t('destinations.newDestination') }}
      </el-button>
    </div>

    <el-card shadow="never" v-loading="loading">
      <el-table :data="tableData" stripe style="width: 100%">
        <el-table-column prop="name" :label="$t('common.name')" min-width="160" />
        <el-table-column prop="type" :label="$t('common.type')" width="140">
          <template #default="{ row }">
            <el-tag>{{ $t('enums.destType.' + row.type, row.type) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="enabled" :label="$t('common.enabled')" width="100">
          <template #default="{ row }">
            <el-switch v-model="row.enabled" @change="(val: boolean) => toggleEnabled(row, val)" />
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
      :title="isEditing ? $t('destinations.editDestination') : $t('destinations.newDestination')"
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
          <el-input v-model="form.name" :placeholder="$t('destinations.destinationName')" />
        </el-form-item>
        <el-form-item :label="$t('common.type')" prop="type">
          <el-select v-model="form.type" :placeholder="$t('destinations.selectType')" @change="onTypeChange">
            <el-option :label="$t('destinations.dingtalk')" value="dingtalk" />
            <el-option :label="$t('destinations.feishu')" value="feishu" />
            <el-option :label="$t('destinations.wecom')" value="wecom" />
            <el-option :label="$t('destinations.email')" value="email" />
            <el-option :label="$t('destinations.httpWebhook')" value="http_webhook" />
          </el-select>
        </el-form-item>

        <!-- DingTalk / Feishu / WeCom: Webhook URL + MsgType -->
        <template v-if="['dingtalk', 'feishu', 'wecom'].includes(form.type)">
          <el-form-item :label="$t('destinations.webhookUrl')" prop="webhook_url">
            <el-input v-model="typeConfig.webhook_url" placeholder="https://oapi.dingtalk.com/robot/send?access_token=..." />
          </el-form-item>
          <el-form-item :label="$t('destinations.msgType')">
            <el-select v-model="typeConfig.msg_type">
              <template v-if="form.type === 'dingtalk'">
                <el-option :label="$t('enums.msgType.markdown')" value="markdown" />
                <el-option :label="$t('enums.msgType.actionCard')" value="actionCard" />
              </template>
              <template v-else-if="form.type === 'feishu'">
                <el-option :label="$t('enums.msgType.interactive')" value="interactive" />
                <el-option :label="$t('enums.msgType.text')" value="text" />
                <el-option :label="$t('enums.msgType.post')" value="post" />
              </template>
              <template v-else-if="form.type === 'wecom'">
                <el-option :label="$t('enums.msgType.markdown')" value="markdown" />
                <el-option :label="$t('enums.msgType.text')" value="text" />
                <el-option :label="$t('enums.msgType.image')" value="image" />
                <el-option :label="$t('enums.msgType.news')" value="news" />
              </template>
            </el-select>
          </el-form-item>
        </template>

        <!-- Email: SMTP config -->
        <template v-if="form.type === 'email'">
          <el-form-item :label="$t('destinations.smtpHost')" prop="smtp_host">
            <el-input v-model="typeConfig.smtp_host" placeholder="smtp.example.com" />
          </el-form-item>
          <el-form-item :label="$t('destinations.smtpPort')">
            <el-input-number v-model="typeConfig.smtp_port" :min="1" :max="65535" :step="1" />
          </el-form-item>
          <el-form-item :label="$t('destinations.useTls')">
            <el-switch v-model="typeConfig.use_tls" />
          </el-form-item>
          <el-form-item :label="$t('destinations.fromAddress')" prop="from">
            <el-input v-model="typeConfig.from" placeholder="noreply@example.com" />
          </el-form-item>
          <el-form-item :label="$t('destinations.toAddress')" prop="to">
            <el-input
              v-model="typeConfig.to"
              type="textarea"
              :rows="3"
              :placeholder="$t('destinations.toPlaceholder')"
            />
          </el-form-item>
          <el-form-item :label="$t('destinations.ccAddress')">
            <el-input
              v-model="typeConfig.cc"
              type="textarea"
              :rows="2"
              :placeholder="$t('destinations.ccPlaceholder')"
            />
          </el-form-item>
        </template>

        <!-- HTTP Webhook config -->
        <template v-if="form.type === 'http_webhook'">
          <el-form-item :label="$t('destinations.targetUrl')" prop="target_url">
            <el-input v-model="typeConfig.url" placeholder="https://example.com/webhook" />
          </el-form-item>
          <el-form-item :label="$t('destinations.httpMethod')">
            <el-select v-model="typeConfig.method">
              <el-option label="POST" value="POST" />
              <el-option label="GET" value="GET" />
              <el-option label="PUT" value="PUT" />
              <el-option label="PATCH" value="PATCH" />
            </el-select>
          </el-form-item>
          <el-form-item :label="$t('destinations.headers')">
            <div class="headers-list">
              <div v-for="(h, idx) in typeConfig.headers" :key="idx" class="header-row">
                <el-input v-model="h.key" :placeholder="$t('destinations.headerKey')" class="header-input" />
                <el-input v-model="h.value" :placeholder="$t('destinations.headerValue')" class="header-input" />
                <el-button text type="danger" @click="removeHeader(idx)">
                  <el-icon><Delete /></el-icon>
                </el-button>
              </div>
              <el-button type="primary" text @click="addHeader">
                <el-icon><Plus /></el-icon> {{ $t('destinations.addHeader') }}
              </el-button>
            </div>
          </el-form-item>
          <el-form-item :label="$t('destinations.bodyTemplate')">
            <el-input
              v-model="typeConfig.body_template"
              type="textarea"
              :rows="4"
            />
          </el-form-item>
        </template>

        <el-divider />

        <el-form-item :label="$t('destinations.secret')" prop="secret_ref">
          <el-input
            v-model="form.secret_ref"
            type="password"
            show-password
            :placeholder="isEditing ? $t('destinations.secretEditPlaceholder') : $t('destinations.secretPlaceholder')"
          />
        </el-form-item>
        <el-form-item :label="$t('destinations.timeoutMs')" prop="timeout_ms">
          <el-input-number v-model="form.timeout_ms" :min="1000" :step="1000" />
        </el-form-item>
        <el-form-item :label="$t('destinations.circuitBreaker')" prop="circuit_breaker">
          <el-input
            v-model="form.circuit_breaker"
            type="textarea"
            :rows="3"
            placeholder='{"failure_threshold":5,"success_threshold":2,"timeout_seconds":60}'
            class="monospace-input"
          />
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
import { Delete } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'
import {
  listDestinations,
  createDestination,
  updateDestination,
  deleteDestination,
} from '@/api/destinations'
import type { DestinationResponse } from '@/api/types'
import { getErrorMessage } from '@/utils/helpers'
import { useCrudPage } from '@/composables/useCrudPage'

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
} = useCrudPage<DestinationResponse>({
  listApi: listDestinations,
  deleteApi: (id) => deleteDestination(id),
  entityI18nKey: 'nav.destinations',
})

const originalSecretRef = ref('')

const formRef = ref<FormInstance>()
const form = reactive({
  name: '',
  type: 'http_webhook',
  secret_ref: '',
  timeout_ms: 10000,
  circuit_breaker: '',
  enabled: true,
})

const formRules: FormRules = {
  name: [{ required: true, message: () => t('destinations.nameRequired'), trigger: 'blur' }],
  type: [{ required: true, message: () => t('destinations.typeRequired'), trigger: 'change' }],
}

// Type-specific config fields
interface HeaderEntry {
  key: string
  value: string
}

const typeConfig = reactive({
  // DingTalk / Feishu / WeCom
  webhook_url: '',
  msg_type: '',
  // Email
  smtp_host: '',
  smtp_port: 465,
  use_tls: true,
  from: '',
  to: '',
  cc: '',
  // HTTP Webhook
  url: '',
  method: 'POST',
  headers: [] as HeaderEntry[],
  body_template: '',
})

const defaultMsgType: Record<string, string> = {
  dingtalk: 'markdown',
  feishu: 'interactive',
  wecom: 'markdown',
}

function resetTypeConfig() {
  typeConfig.webhook_url = ''
  typeConfig.msg_type = defaultMsgType[form.type] || ''
  typeConfig.smtp_host = ''
  typeConfig.smtp_port = 465
  typeConfig.use_tls = true
  typeConfig.from = ''
  typeConfig.to = ''
  typeConfig.cc = ''
  typeConfig.url = ''
  typeConfig.method = 'POST'
  typeConfig.headers = []
  typeConfig.body_template = ''
}

function onTypeChange() {
  resetTypeConfig()
}

function addHeader() {
  typeConfig.headers.push({ key: '', value: '' })
}

function removeHeader(index: number) {
  typeConfig.headers.splice(index, 1)
}

// Parse backend config JSON into typeConfig fields
function parseConfigToTypeConfig(config: unknown) {
  const cfg = (typeof config === 'string' ? JSON.parse(config) : config) as Record<string, unknown>
  if (!cfg || typeof cfg !== 'object') return

  if (['dingtalk', 'feishu', 'wecom'].includes(form.type)) {
    typeConfig.webhook_url = String(cfg.webhook_url ?? '')
    typeConfig.msg_type = String(cfg.msg_type ?? defaultMsgType[form.type] ?? '')
  } else if (form.type === 'email') {
    typeConfig.smtp_host = String(cfg.smtp_host ?? '')
    typeConfig.smtp_port = Number(cfg.smtp_port ?? 465)
    typeConfig.use_tls = cfg.use_tls !== false
    typeConfig.from = String(cfg.from ?? '')
    // to can be string or array
    if (Array.isArray(cfg.to)) {
      typeConfig.to = (cfg.to as string[]).join('\n')
    } else {
      typeConfig.to = String(cfg.to ?? '')
    }
    if (Array.isArray(cfg.cc)) {
      typeConfig.cc = (cfg.cc as string[]).join('\n')
    } else {
      typeConfig.cc = String(cfg.cc ?? '')
    }
  } else if (form.type === 'http_webhook') {
    typeConfig.url = String(cfg.url ?? '')
    typeConfig.method = String(cfg.method ?? 'POST')
    // headers can be object or absent
    if (cfg.headers && typeof cfg.headers === 'object') {
      typeConfig.headers = Object.entries(cfg.headers as Record<string, string>).map(
        ([key, value]) => ({ key, value: String(value) })
      )
    } else {
      typeConfig.headers = []
    }
    typeConfig.body_template = String(cfg.body_template ?? '')
  }
}

// Build config JSON from typeConfig fields
function buildConfigFromTypeConfig(): Record<string, unknown> {
  if (['dingtalk', 'feishu', 'wecom'].includes(form.type)) {
    const config: Record<string, unknown> = {
      webhook_url: typeConfig.webhook_url,
    }
    if (typeConfig.msg_type) {
      config.msg_type = typeConfig.msg_type
    }
    return config
  } else if (form.type === 'email') {
    const toList = typeConfig.to
      .split(/[\n,]/)
      .map((s: string) => s.trim())
      .filter(Boolean)
    const ccList = typeConfig.cc
      .split(/[\n,]/)
      .map((s: string) => s.trim())
      .filter(Boolean)
    const config: Record<string, unknown> = {
      smtp_host: typeConfig.smtp_host,
      smtp_port: typeConfig.smtp_port,
      use_tls: typeConfig.use_tls,
      from: typeConfig.from,
      to: toList,
    }
    if (ccList.length > 0) {
      config.cc = ccList
    }
    return config
  } else if (form.type === 'http_webhook') {
    const headersObj: Record<string, string> = {}
    for (const h of typeConfig.headers) {
      if (h.key.trim()) {
        headersObj[h.key.trim()] = h.value
      }
    }
    const config: Record<string, unknown> = {
      url: typeConfig.url,
      method: typeConfig.method,
    }
    if (Object.keys(headersObj).length > 0) {
      config.headers = headersObj
    }
    if (typeConfig.body_template) {
      config.body_template = typeConfig.body_template
    }
    return config
  }
  return {}
}

function resetForm() {
  form.name = ''
  form.type = 'http_webhook'
  form.secret_ref = ''
  form.timeout_ms = 10000
  form.circuit_breaker = ''
  form.enabled = true
  resetTypeConfig()
}

function openDialog(row?: DestinationResponse) {
  resetForm()
  if (row) {
    isEditing.value = true
    editingId.value = row.id
    form.name = row.name ?? ''
    form.type = row.type ?? 'http_webhook'
    form.secret_ref = (row.secret_ref as string) ?? ''
    originalSecretRef.value = form.secret_ref
    form.timeout_ms = row.timeout_ms ?? 10000
    form.circuit_breaker =
      typeof row.circuit_breaker === 'string'
        ? row.circuit_breaker
        : JSON.stringify(row.circuit_breaker ?? {}, null, 2)
    form.enabled = row.enabled ?? true
    // Parse existing config into typeConfig
    try {
      parseConfigToTypeConfig(row.config)
    } catch {
      // If config can't be parsed, leave typeConfig at defaults
    }
  } else {
    isEditing.value = false
    editingId.value = ''
    originalSecretRef.value = ''
  }
  dialogVisible.value = true
}

async function handleSave() {
  if (!formRef.value) return
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  saving.value = true
  try {
    let cbParsed: unknown = null
    if (form.circuit_breaker) {
      try {
        cbParsed = JSON.parse(form.circuit_breaker)
      } catch {
        ElMessage.error(t('destinations.invalidCbJson'))
        return
      }
    }

    const config = buildConfigFromTypeConfig()

    const payload: Record<string, unknown> = {
      name: form.name,
      type: form.type,
      config,
      timeout_ms: form.timeout_ms,
      circuit_breaker: cbParsed,
      enabled: form.enabled,
    }
    if (form.secret_ref && form.secret_ref !== originalSecretRef.value) {
      payload.secret_ref = form.secret_ref
    }
    if (isEditing.value) {
      await updateDestination(editingId.value, payload)
      ElMessage.success(t('destinations.destinationUpdated'))
    } else {
      await createDestination(payload)
      ElMessage.success(t('destinations.destinationCreated'))
    }
    dialogVisible.value = false
    await fetchData()
  } catch (e: unknown) {
    ElMessage.error(getErrorMessage(e, t('destinations.failedToSaveDestination')))
  } finally {
    saving.value = false
  }
}

async function toggleEnabled(row: DestinationResponse, val: boolean) {
  try {
    await updateDestination(row.id, { enabled: val })
    ElMessage.success(val ? t('destinations.destinationEnabled') : t('destinations.destinationDisabled'))
  } catch (e: unknown) {
    row.enabled = !val
    ElMessage.error(getErrorMessage(e, t('destinations.failedToToggleDestination')))
  }
}

onMounted(() => {
  fetchData()
})
</script>

<style lang="scss" scoped>
.destinations-container {
  .headers-list {
    width: 100%;

    .header-row {
      display: flex;
      align-items: center;
      gap: 8px;
      margin-bottom: 8px;

      .header-input {
        flex: 1;
      }
    }
  }
}
</style>
