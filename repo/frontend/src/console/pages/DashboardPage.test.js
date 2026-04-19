import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ref } from 'vue'

const api = vi.hoisted(() => ({ get: vi.fn() }))
const user = vi.hoisted(() => ({ value: null }))

vi.mock('../api.js', () => ({ get: api.get }))
vi.mock('../../composables/useRbac.js', async (orig) => {
  const real = await orig()
  return { ...real, useRbac: () => ({ hasAny: (...roles) => roles.includes(user.value?.role) }) }
})
vi.mock('../../composables/useAuth.js', () => ({
  useAuth: () => ({ user: ref(user.value) }),
}))

beforeEach(() => {
  api.get.mockReset()
  user.value = { role: 'administrator' }
})

describe('console/DashboardPage.vue', () => {
  it('renders backend health status and node count for admin', async () => {
    api.get
      .mockResolvedValueOnce({ status: 'ok', db: 'up' })       // /health
      .mockResolvedValueOnce({ items: [{ id: 1 }, { id: 2 }] }) // /crawl/nodes
      .mockResolvedValueOnce({ items: [{ batch_id: 'b' }] })    // /approvals
    const { default: DashboardPage } = await import('./DashboardPage.vue')
    const w = mount(DashboardPage)
    await flushPromises()
    expect(w.text()).toContain('ok')
    expect(w.text()).toContain('up')
    expect(w.text()).toContain('Crawl Nodes')
    expect(w.text()).toContain('Pending Approvals')
  })

  it('omits pending-approvals card for non-admin roles', async () => {
    user.value = { role: 'content_editor' }
    api.get
      .mockResolvedValueOnce({ status: 'ok', db: 'up' })
      .mockResolvedValueOnce({ items: [] })
    const { default: DashboardPage } = await import('./DashboardPage.vue')
    const w = mount(DashboardPage)
    await flushPromises()
    expect(w.text()).not.toContain('Pending Approvals')
  })
})
