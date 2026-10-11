/**
 * Status tag type mappings for consistent display across views.
 */

const eventStatusMap: Record<string, string> = {
  received: 'info',
  processing: 'warning',
  completed: 'success',
  failed: 'danger',
}

const deliveryStatusMap: Record<string, string> = {
  pending: 'info',
  processing: 'warning',
  delivered: 'success',
  retry_wait: 'warning',
  failed: 'danger',
  dead_letter: 'danger',
}

const auditActionMap: Record<string, string> = {
  create: 'success',
  update: 'primary',
  delete: 'danger',
  login: '',
  replay: 'warning',
  retry: 'warning',
}

export function eventStatusTagType(status: string): string {
  return eventStatusMap[status] ?? 'info'
}

export function deliveryStatusTagType(status: string): string {
  return deliveryStatusMap[status] ?? 'info'
}

export function actionTagType(action: string): string {
  return auditActionMap[action] ?? ''
}
