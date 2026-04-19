import { describe, it, expect, vi, beforeEach } from 'vitest'

// We drive the real router against in-memory state for useAuth + useRbac so we
// exercise the guard (auth + role gating) without hitting the network.
const authState = vi.hoisted(() => ({
  ready: { value: true },
  isAuthenticated: { value: false },
  user: { value: null },
  checkSessionCalls: 0,
}))

vi.mock('../composables/useAuth.js', () => ({
  useAuth: () => ({
    ready: authState.ready,
    isAuthenticated: authState.isAuthenticated,
    user: authState.user,
  }),
  checkSession: async () => { authState.checkSessionCalls++ },
}))

// useRbac exports ROLES — use the real module; useAuth is the only dependency
// router/index.js pulls from ../composables that we need to control.

beforeEach(() => {
  authState.ready.value = true
  authState.isAuthenticated.value = false
  authState.user.value = null
  authState.checkSessionCalls = 0
})

describe('router/index.js', () => {
  it('exposes expected named routes for every admin area', async () => {
    const router = (await import('./index.js')).default
    const names = router.getRoutes().map(r => r.name).filter(Boolean)
    for (const n of [
      'search',
      'console.dashboard',
      'console.content',
      'console.pricing',
      'console.complaints',
      'console.crawl',
      'console.approvals',
      'console.audit',
      'console.monitoring',
      'console.settings',
    ]) {
      expect(names).toContain(n)
    }
  })

  it('anonymous /console navigation redirects to /', async () => {
    const router = (await import('./index.js')).default
    await router.push('/console')
    expect(router.currentRoute.value.name).toBe('search')
  })

  it('authenticated reviewer blocked from admin-only /console/audit falls back to dashboard', async () => {
    authState.isAuthenticated.value = true
    authState.user.value = { username: 'rv', role: 'reviewer' }
    const router = (await import('./index.js')).default
    await router.push('/console/audit')
    expect(router.currentRoute.value.name).toBe('console.dashboard')
  })

  it('authenticated admin can reach admin-only /console/audit', async () => {
    authState.isAuthenticated.value = true
    authState.user.value = { username: 'admin', role: 'administrator' }
    const router = (await import('./index.js')).default
    await router.push('/console/audit')
    expect(router.currentRoute.value.name).toBe('console.audit')
  })

  it('unknown paths redirect to the public search page', async () => {
    const router = (await import('./index.js')).default
    await router.push('/does-not-exist')
    expect(router.currentRoute.value.name).toBe('search')
  })
})
