<template>
  <el-tooltip :content="id" :disabled="!overflowed" placement="top" :show-after="300">
    <span class="truncated-id" @click.stop="copyId" ref="wrapRef">
      <span class="truncated-id__text" ref="textRef">{{ id }}</span>
      <el-icon class="copy-icon" :size="12"><CopyDocument /></el-icon>
    </span>
  </el-tooltip>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { ElMessage } from 'element-plus'
import { CopyDocument } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const props = defineProps<{
  id: string
}>()

const wrapRef = ref<HTMLElement>()
const textRef = ref<HTMLElement>()
const overflowed = ref(false)

function checkOverflow() {
  if (textRef.value) {
    overflowed.value = textRef.value.scrollWidth > textRef.value.clientWidth
  }
}

let ro: ResizeObserver | undefined
onMounted(() => {
  checkOverflow()
  if (wrapRef.value) {
    ro = new ResizeObserver(checkOverflow)
    ro.observe(wrapRef.value)
  }
})
onBeforeUnmount(() => ro?.disconnect())

async function copyId() {
  try {
    await navigator.clipboard.writeText(props.id)
    ElMessage.success(t('common.copied'))
  } catch {
    ElMessage.error(t('common.copyFailed'))
  }
}
</script>

<style lang="scss" scoped>
.truncated-id {
  display: flex;
  align-items: center;
  min-width: 0; // allow flex child to shrink below content size
  cursor: pointer;
  font-size: inherit;

  &__text {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    min-width: 0;
  }

  .copy-icon {
    margin-left: 4px;
    flex-shrink: 0;
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
