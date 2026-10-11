import { ref, onMounted, onBeforeUnmount } from 'vue'

/**
 * Composable that makes an el-drawer resizable by dragging its left edge.
 * Call setup() after the drawer opens (use @open event), and add the
 * resize-handle div inside the drawer template.
 */
export function useResizableDrawer(initialWidth = 800, minWidth = 400, maxWidth = 1200) {
  const drawerWidth = ref(`${initialWidth}px`)
  let dragging = false
  let startX = 0
  let startWidth = 0

  function onMouseMove(e: MouseEvent) {
    if (!dragging) return
    // Drawer opens from the right, so dragging left = increasing width
    const delta = startX - e.clientX
    const newWidth = Math.min(maxWidth, Math.max(minWidth, startWidth + delta))
    drawerWidth.value = `${newWidth}px`
  }

  function onMouseUp() {
    dragging = false
    document.body.style.cursor = ''
    document.body.style.userSelect = ''
  }

  function onHandleMouseDown(e: MouseEvent) {
    e.preventDefault()
    dragging = true
    startX = e.clientX
    startWidth = parseInt(drawerWidth.value, 10)
    document.body.style.cursor = 'col-resize'
    document.body.style.userSelect = 'none'
  }

  onMounted(() => {
    document.addEventListener('mousemove', onMouseMove)
    document.addEventListener('mouseup', onMouseUp)
  })

  onBeforeUnmount(() => {
    document.removeEventListener('mousemove', onMouseMove)
    document.removeEventListener('mouseup', onMouseUp)
  })

  return {
    drawerWidth,
    onHandleMouseDown,
  }
}
