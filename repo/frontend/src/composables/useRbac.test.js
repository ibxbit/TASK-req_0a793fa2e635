import { describe, it, expect } from 'vitest'
import { ROLES, useRbac } from './useRbac.js'

describe('useRbac', () => {
  it('exposes all backend role constants', () => {
    expect(ROLES.ADMIN).toBe('administrator')
    expect(ROLES.EDITOR).toBe('content_editor')
    expect(ROLES.REVIEWER).toBe('reviewer')
    expect(ROLES.MKT).toBe('marketing_manager')
    expect(ROLES.CRAWLER).toBe('crawler_operator')
  })

  it('hasAny returns false when user is null', () => {
    const { hasAny, role } = useRbac()
    expect(role.value).toBeNull()
    expect(hasAny(ROLES.ADMIN)).toBe(false)
  })
})
