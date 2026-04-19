import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

const api = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
}))

vi.mock('../api.js', () => ({
  get: api.get,
  post: api.post,
}))

beforeEach(() => {
  api.get.mockReset()
  api.post.mockReset()
  // Stub window.confirm so the Restore flow can proceed in tests.
  window.confirm = vi.fn(() => true)
})

describe('RevisionsPage.vue', () => {
  it('fetches supported-entities on mount and populates the dropdown', async () => {
    api.get.mockResolvedValueOnce({ items: ['dynasty', 'author', 'poem'], retention_days: 30 })
    const { default: Page } = await import('./RevisionsPage.vue')
    const w = mount(Page)
    await flushPromises()
    expect(api.get).toHaveBeenCalledWith('/revisions/supported-entities')
    const options = w.findAll('select option').map(o => o.text())
    expect(options).toContain('dynasty')
    expect(options).toContain('poem')
    expect(w.text()).toContain('30 days')
  })

  it('loading with a valid entity populates the history table', async () => {
    api.get.mockResolvedValueOnce({ items: ['dynasty'], retention_days: 30 })
    api.get.mockResolvedValueOnce({ items: [
      { id: 10, action: 'update', actor_role: 'administrator', created_at: '2026-04-18T12:00:00Z', restorable: true, before: { name: 'old' }, after: { name: 'new' } },
      { id: 9,  action: 'create', actor_role: 'administrator', created_at: '2026-04-18T10:00:00Z', restorable: true, after: { name: 'old' } },
    ] })
    const { default: Page } = await import('./RevisionsPage.vue')
    const w = mount(Page)
    await flushPromises()

    await w.find('input[type="number"]').setValue('42')
    await w.find('.lookup form').trigger('submit.prevent')
    await flushPromises()

    expect(api.get.mock.calls[1][1]).toEqual({
      entity_type: 'dynasty',
      entity_id: 42,
      limit: 100,
    })
    const rows = w.findAll('tbody tr')
    expect(rows).toHaveLength(2)
    expect(rows[0].text()).toContain('update')
    expect(rows[0].find('.restore').exists()).toBe(true)
  })

  it('clicking Restore POSTs /revisions/:id/restore and reloads', async () => {
    api.get.mockResolvedValueOnce({ items: ['dynasty'], retention_days: 30 })
    api.get.mockResolvedValueOnce({ items: [
      { id: 10, action: 'update', actor_role: 'administrator', created_at: 't', restorable: true, before: { name: 'old' } },
    ] })
    api.post.mockResolvedValueOnce({
      restored_revision_id: 10, entity_type: 'dynasty', entity_id: 42, action_restored: 'update',
    })
    api.get.mockResolvedValueOnce({ items: [] }) // reload after restore

    const { default: Page } = await import('./RevisionsPage.vue')
    const w = mount(Page)
    await flushPromises()
    await w.find('input[type="number"]').setValue('42')
    await w.find('.lookup form').trigger('submit.prevent')
    await flushPromises()

    await w.find('.restore').trigger('click')
    await flushPromises()
    expect(api.post).toHaveBeenCalledWith('/revisions/10/restore')
    expect(w.find('[data-test="rev-ok"]').exists()).toBe(true)
  })

  it('shows the backend error when restore fails', async () => {
    api.get.mockResolvedValueOnce({ items: ['dynasty'], retention_days: 30 })
    api.get.mockResolvedValueOnce({ items: [
      { id: 99, action: 'delete', actor_role: 'administrator', created_at: 't', restorable: true, before: { id: 7, name: 'x' } },
    ] })
    api.post.mockRejectedValueOnce({ response: { data: { error: 'beyond retention window' } } })
    const { default: Page } = await import('./RevisionsPage.vue')
    const w = mount(Page)
    await flushPromises()
    await w.find('input[type="number"]').setValue('7')
    await w.find('.lookup form').trigger('submit.prevent')
    await flushPromises()
    await w.find('.restore').trigger('click')
    await flushPromises()
    expect(w.find('[data-test="rev-error"]').text()).toContain('beyond retention window')
  })
})
