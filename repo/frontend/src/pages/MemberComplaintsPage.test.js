import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'

const { mockApiGet } = vi.hoisted(() => ({ mockApiGet: vi.fn() }))

vi.mock('../offline/api.js', () => ({
  apiGet: (url, opts) => mockApiGet(url, opts),
  apiWrite: vi.fn(),
}))

beforeEach(() => mockApiGet.mockReset())

const COMPLAINT = {
  id: 1,
  subject: 'Spam content',
  target_type: 'poem',
  arbitration_code: 'submitted',
  resolution: null,
  resolved_at: null,
}

describe('MemberComplaintsPage.vue', () => {
  it('shows loading indicator while fetching, then the list', async () => {
    // Hold the promise open so we can assert the loading state mid-flight.
    let resolveLoad
    mockApiGet.mockReturnValueOnce(new Promise(r => { resolveLoad = r }))

    const { default: Page } = await import('./MemberComplaintsPage.vue')
    const w = mount(Page)

    // After the first DOM flush, loading should be visible.
    await nextTick()
    expect(w.find('[data-test="loading"]').exists()).toBe(true)
    expect(w.find('[data-test="list"]').exists()).toBe(false)

    // Resolve the fetch and let the DOM settle.
    resolveLoad({ data: { items: [COMPLAINT] }, fromCache: false })
    await flushPromises()

    expect(w.find('[data-test="loading"]').exists()).toBe(false)
    expect(w.find('[data-test="list"]').exists()).toBe(true)
  })

  it('renders subject, target_type, and arbitration_code for each complaint', async () => {
    mockApiGet.mockResolvedValueOnce({
      data: { items: [COMPLAINT] },
      fromCache: false,
    })
    const { default: Page } = await import('./MemberComplaintsPage.vue')
    const w = mount(Page)
    await flushPromises()

    expect(w.text()).toContain('Spam content')
    expect(w.text()).toContain('poem')
    expect(w.text()).toContain('submitted')
  })

  it('renders resolution and resolved_at when present', async () => {
    mockApiGet.mockResolvedValueOnce({
      data: { items: [{
        id: 2,
        subject: 'Dispute',
        target_type: 'review',
        arbitration_code: 'resolved_upheld',
        resolution: 'upheld',
        resolved_at: '2026-01-15T10:00:00Z',
      }]},
      fromCache: false,
    })
    const { default: Page } = await import('./MemberComplaintsPage.vue')
    const w = mount(Page)
    await flushPromises()

    expect(w.text()).toContain('Resolution: upheld')
    expect(w.text()).toContain('Resolved:')
  })

  it('shows empty state when the member has no complaints', async () => {
    mockApiGet.mockResolvedValueOnce({ data: { items: [] }, fromCache: false })
    const { default: Page } = await import('./MemberComplaintsPage.vue')
    const w = mount(Page)
    await flushPromises()

    expect(w.find('[data-test="empty"]').exists()).toBe(true)
    expect(w.find('[data-test="list"]').exists()).toBe(false)
    // Match actual template text
    expect(w.text()).toContain('You have not submitted any complaints yet')
  })

  it('shows error state when the fetch rejects', async () => {
    mockApiGet.mockRejectedValueOnce(new Error('network failure'))
    const { default: Page } = await import('./MemberComplaintsPage.vue')
    const w = mount(Page)
    await flushPromises()

    expect(w.find('[data-test="error"]').exists()).toBe(true)
    expect(w.text()).toContain('network failure')
    expect(w.find('[data-test="list"]').exists()).toBe(false)
  })

  it('shows cache-note banner when data is served from offline cache', async () => {
    mockApiGet.mockResolvedValueOnce({ data: { items: [] }, fromCache: true })
    const { default: Page } = await import('./MemberComplaintsPage.vue')
    const w = mount(Page)
    await flushPromises()

    expect(w.find('[data-test="cache-note"]').exists()).toBe(true)
    expect(w.text()).toContain('cached')
  })

  it('calls apiGet with the correct URL and cache key', async () => {
    mockApiGet.mockResolvedValueOnce({ data: { items: [] }, fromCache: false })
    const { default: Page } = await import('./MemberComplaintsPage.vue')
    mount(Page)
    await flushPromises()

    expect(mockApiGet).toHaveBeenCalledOnce()
    expect(mockApiGet).toHaveBeenCalledWith(
      '/api/v1/complaints/mine',
      expect.objectContaining({ cacheKey: 'complaints:mine' }),
    )
  })

  it('renders multiple complaints in document order', async () => {
    mockApiGet.mockResolvedValueOnce({
      data: { items: [
        { id: 1, subject: 'First',  target_type: 'poem', arbitration_code: 'submitted',    resolution: null, resolved_at: null },
        { id: 2, subject: 'Second', target_type: 'user', arbitration_code: 'under_review', resolution: null, resolved_at: null },
      ]},
      fromCache: false,
    })
    const { default: Page } = await import('./MemberComplaintsPage.vue')
    const w = mount(Page)
    await flushPromises()

    const rows = w.findAll('.item')
    expect(rows).toHaveLength(2)
    expect(rows[0].text()).toContain('First')
    expect(rows[1].text()).toContain('Second')
  })
})
