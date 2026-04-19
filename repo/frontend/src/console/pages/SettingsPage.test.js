import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

const api = vi.hoisted(() => ({
  get: vi.fn(),
  put: vi.fn(),
}))

// SettingsPage now imports { get, put } — no longer imports http directly.
vi.mock('../api.js', () => ({
  get: api.get,
  put: api.put,
}))

beforeEach(() => {
  api.get.mockReset()
  api.put.mockReset()
})

describe('console/SettingsPage.vue', () => {
  it('loads the current approval setting on mount and reflects it on the switch', async () => {
    api.get.mockResolvedValueOnce({ approval_required: true })
    const { default: SettingsPage } = await import('./SettingsPage.vue')
    const w = mount(SettingsPage)
    await flushPromises()
    expect(api.get).toHaveBeenCalledWith('/settings/approval')
    expect(w.find('.switch.on').exists()).toBe(true)
    expect(w.find('.switch').text()).toBe('ON')
  })

  it('toggle calls put() — routed through the offline queue — and flips the UI on success', async () => {
    api.get.mockResolvedValueOnce({ approval_required: false })
    // Simulate the online path: put() resolves with server data.
    api.put.mockResolvedValueOnce({ queued: false, data: {} })
    const { default: SettingsPage } = await import('./SettingsPage.vue')
    const w = mount(SettingsPage)
    await flushPromises()
    expect(w.find('.switch.off').exists()).toBe(true)

    await w.find('.switch').trigger('click')
    await flushPromises()

    // put() is called with (url, body) — the Idempotency-Key and queue
    // mechanics are handled inside console/api.js, not the component.
    expect(api.put).toHaveBeenCalledOnce()
    const [url, body] = api.put.mock.calls[0]
    expect(url).toBe('/settings/approval')
    expect(body).toEqual({ enabled: true })
    expect(w.find('.switch.on').exists()).toBe(true)
  })

  it('toggle when offline (put queues) still flips the UI — optimistic update', async () => {
    api.get.mockResolvedValueOnce({ approval_required: false })
    // put() in console/api.js returns `res.data ?? null` — when apiWrite enqueues
    // the entry (offline), res.data is undefined so the caller receives null.
    api.put.mockResolvedValueOnce(null)
    const { default: SettingsPage } = await import('./SettingsPage.vue')
    const w = mount(SettingsPage)
    await flushPromises()
    expect(w.find('.switch.off').exists()).toBe(true)

    await w.find('.switch').trigger('click')
    await flushPromises()

    // null resolves without throwing → optimistic flip succeeds, no error shown.
    expect(w.find('.err').exists()).toBe(false)
    expect(w.find('.switch.on').exists()).toBe(true)
  })

  it('switch stays flipped after background queue drain completes — no regression', async () => {
    // Step 1: offline toggle — put() resolves null (entry queued), component flips.
    api.get.mockResolvedValueOnce({ approval_required: false })
    api.put.mockResolvedValueOnce(null)
    const { default: SettingsPage } = await import('./SettingsPage.vue')
    const w = mount(SettingsPage)
    await flushPromises()
    expect(w.find('.switch.off').exists()).toBe(true)

    await w.find('.switch').trigger('click')
    await flushPromises()

    expect(w.find('.switch.on').exists()).toBe(true)
    expect(w.find('.err').exists()).toBe(false)

    // Step 2: network restores — queue.js calls processQueue() in the background.
    // The component is not involved: it already flipped optimistically.
    // Confirm the component state is stable (no rollback, no stale error).
    // A second put() call (from a re-toggle) also works cleanly after drain.
    api.put.mockResolvedValueOnce({ approval_required: false })
    await w.find('.switch').trigger('click')
    await flushPromises()

    // Toggled back to OFF — drain path returns a value (online), no error.
    expect(w.find('.switch.off').exists()).toBe(true)
    expect(w.find('.err').exists()).toBe(false)
  })

  it('surfaces API errors in the .err region without flipping the switch', async () => {
    api.get.mockResolvedValueOnce({ approval_required: false })
    api.put.mockRejectedValueOnce({ response: { data: { error: 'forbidden' } } })
    const { default: SettingsPage } = await import('./SettingsPage.vue')
    const w = mount(SettingsPage)
    await flushPromises()
    await w.find('.switch').trigger('click')
    await flushPromises()
    expect(w.find('.err').text()).toBe('forbidden')
    // Still OFF — the flip only happens when put() resolves (online or queued).
    expect(w.find('.switch.off').exists()).toBe(true)
  })
})
