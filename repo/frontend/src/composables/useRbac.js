import { computed } from 'vue'
import { useAuth } from './useAuth.js'

export const ROLES = {
  ADMIN:    'administrator',
  EDITOR:   'content_editor',
  REVIEWER: 'reviewer',
  MKT:      'marketing_manager',
  CRAWLER:  'crawler_operator',
  MEMBER:   'member',
}

export function useRbac() {
  const { user } = useAuth()
  const role = computed(() => user.value?.role || null)
  function hasAny(...roles) {
    if (!role.value) return false
    return roles.includes(role.value)
  }
  return { role, hasAny }
}
