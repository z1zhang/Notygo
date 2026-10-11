<template>
  <el-tag :type="tagType" size="small">{{ label }}</el-tag>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { eventStatusTagType, deliveryStatusTagType, actionTagType } from '@/utils/status'

const props = defineProps<{
  /** 'event' | 'delivery' | 'audit' */
  kind: 'event' | 'delivery' | 'audit'
  status: string
}>()

const { t } = useI18n()

const tagType = computed(() => {
  switch (props.kind) {
    case 'event': return eventStatusTagType(props.status)
    case 'delivery': return deliveryStatusTagType(props.status)
    case 'audit': return actionTagType(props.status)
    default: return ''
  }
})

const label = computed(() => {
  switch (props.kind) {
    case 'event': return t('enums.eventStatus.' + props.status, props.status)
    case 'delivery': return t('enums.deliveryStatus.' + props.status, props.status)
    case 'audit': return t('audit.' + props.status, props.status)
    default: return props.status
  }
})
</script>
