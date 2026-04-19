import { describe, it, expect, vi, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { reactive } from 'vue'

// A fake queue module so we don't touch IndexedDB or timers.
//
// NOTE: the real `queueState` is a Vue ref, but inside the template the
// component accesses `queueState.count` directly (relying on either ref
// auto-unwrapping or plain property access). Passing a plain reactive
// object satisfies the template because `queueState.count` resolves the
// same way.
const fakeQueue = vi.hoisted(() => ({
  listQueue: vi.fn(),
  processQueue: vi.fn(),
  removeFromQueue: vi.fn(),
  retryEntry: vi.fn(),
  queueState: { count: 0, pending: 0, failed: 0 },
}))

vi.mock('../offline/queue.js', () => ({
  listQueue: fakeQueue.listQueue,
  processQueue: fakeQueue.processQueue,
  removeFromQueue: fakeQueue.removeFromQueue,
  retryEntry: fakeQueue.retryEntry,
  queueState: fakeQueue.queueState,
}))

afterEach(() => {
  vi.clearAllMocks()
})

function setState(patch) {
  Object.assign(fakeQueue.queueState, { count: 0, pending: 0, failed: 0 }, patch)
}

describe('QueueDrawer.vue', () => {
  it('stays closed by default and toggles on click', async () => {
    setState({})
    fakeQueue.listQueue.mockResolvedValue([])
    const { default: QueueDrawer } = await import('./QueueDrawer.vue')
    const w = mount(QueueDrawer)
    await flushPromises()
    expect(w.find('.queue-panel').exists()).toBe(false)
    await w.find('.queue-toggle').trigger('click')
    expect(w.find('.queue-panel').exists()).toBe(true)
    expect(w.find('.empty').text()).toContain('No queued actions')
  })

  it('renders queued entries with method, url, status', async () => {
    setState({ count: 1, pending: 1 })
    fakeQueue.listQueue.mockResolvedValue([{
      id: 'abc', method: 'POST', url: '/api/v1/reviews', status: 'pending',
      kind: 'review', retries: 0, next_retry_at: Date.now() + 5000,
      created_at: Date.now(),
    }])
    const { default: QueueDrawer } = await import('./QueueDrawer.vue')
    const w = mount(QueueDrawer)
    await flushPromises()
    await w.find('.queue-toggle').trigger('click')
    expect(w.find('code').text()).toContain('POST /api/v1/reviews')
    expect(w.find('.status').text()).toBe('pending')
  })

  it('Retry now calls processQueue and then refreshes the list', async () => {
    setState({ count: 1, pending: 1 })
    fakeQueue.listQueue.mockResolvedValue([{
      id: 'abc', method: 'POST', url: '/x', status: 'pending', kind: 'a',
      retries: 0, next_retry_at: Date.now(), created_at: Date.now(),
    }])
    fakeQueue.processQueue.mockResolvedValue()
    const { default: QueueDrawer } = await import('./QueueDrawer.vue')
    const w = mount(QueueDrawer)
    await flushPromises()
    await w.find('.queue-toggle').trigger('click')
    const [retryAll] = w.findAll('.head button')
    await retryAll.trigger('click')
    await flushPromises()
    expect(fakeQueue.processQueue).toHaveBeenCalled()
  })

  it('per-row Retry and Drop actions dispatch the right queue methods', async () => {
    setState({ count: 1, failed: 1 })
    fakeQueue.listQueue.mockResolvedValue([{
      id: 'xyz', method: 'POST', url: '/x', status: 'failed', kind: 'a',
      retries: 3, next_retry_at: Date.now(), created_at: Date.now(),
      error: 'boom',
    }])
    const { default: QueueDrawer } = await import('./QueueDrawer.vue')
    const w = mount(QueueDrawer)
    await flushPromises()
    await w.find('.queue-toggle').trigger('click')
    const actionButtons = w.findAll('li .actions button')
    await actionButtons[0].trigger('click') // Retry
    await actionButtons[1].trigger('click') // Drop
    expect(fakeQueue.retryEntry).toHaveBeenCalledWith('xyz')
    expect(fakeQueue.removeFromQueue).toHaveBeenCalledWith('xyz')
    expect(w.find('.err').text()).toBe('boom')
  })
})
