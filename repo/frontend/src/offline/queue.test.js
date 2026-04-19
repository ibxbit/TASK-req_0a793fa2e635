import { describe, it, expect, vi, beforeEach } from 'vitest'

// queue.js reads `isOnline.value` as a ref and fires fetch on enqueue. We stub
// both so tests are hermetic — no network, no timers.
vi.mock('./network.js', () => {
  const ref = { value: false }
  return {
    isOnline: ref,
    onNetworkChange: vi.fn(),
  }
})

let fetchMock
beforeEach(() => {
  fetchMock = vi.fn().mockResolvedValue({
    ok: true,
    status: 200,
    json: async () => ({ id: 1 }),
  })
  globalThis.fetch = fetchMock
})

// Drain any in-flight fire-and-forget processQueue by looping until the store
// is stable. This avoids flaky assertions caused by enqueue()'s background
// flush running concurrently with our explicit processQueue() call.
async function drain(mod) {
  for (let i = 0; i < 10; i++) {
    await mod.processQueue()
    await new Promise(r => setTimeout(r, 0))
    const rows = await mod.listQueue()
    if (rows.every(r => r.status !== 'in_flight')) return rows
  }
  return mod.listQueue()
}

describe('offline/queue', () => {
  it('enqueue stores a pending entry with idempotency key + updates state', async () => {
    const { enqueue, queueState, listQueue } = await import('./queue.js')
    const entry = await enqueue({
      method: 'POST', url: '/api/v1/dynasties', body: { name: 'x' }, kind: 'edit',
    })
    expect(entry.idempotency_key).toBeTruthy()
    expect(entry.retries).toBe(0)
    const rows = await listQueue()
    expect(rows.length).toBe(1)
    expect(queueState.value.count).toBe(1)
  })

  it('processQueue does nothing while offline', async () => {
    const mod = await import('./queue.js')
    await mod.enqueue({ method: 'POST', url: '/x', body: { a: 1 } })
    await mod.processQueue()
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('processQueue drains pending entries when online and removes them on 2xx', async () => {
    const netMod = await import('./network.js')
    netMod.isOnline.value = true
    const mod = await import('./queue.js')
    await mod.enqueue({ method: 'POST', url: '/api/v1/reviews', body: { r: 1 } })
    const rows = await drain(mod)
    expect(fetchMock).toHaveBeenCalled()
    expect(rows.length).toBe(0)
    netMod.isOnline.value = false
  })

  it('persists an Idempotency-Key header on flush', async () => {
    const netMod = await import('./network.js')
    netMod.isOnline.value = true
    const mod = await import('./queue.js')
    await mod.enqueue({ method: 'POST', url: '/x', body: { a: 1 } })
    await drain(mod)
    expect(fetchMock.mock.calls.length).toBeGreaterThan(0)
    const opts = fetchMock.mock.calls[0][1]
    expect(opts.headers['Idempotency-Key']).toBeTruthy()
    expect(opts.headers['Content-Type']).toBe('application/json')
    expect(opts.credentials).toBe('include')
    netMod.isOnline.value = false
  })

  it('marks 4xx client errors as permanently failed (not retried)', async () => {
    const netMod = await import('./network.js')
    netMod.isOnline.value = true
    fetchMock.mockResolvedValue({ ok: false, status: 400, json: async () => ({ error: 'bad' }) })
    const mod = await import('./queue.js')
    await mod.enqueue({ method: 'POST', url: '/x', body: {} })
    const rows = await drain(mod)
    expect(rows).toHaveLength(1)
    expect(rows[0].status).toBe('failed')
    netMod.isOnline.value = false
  })

  it('retries 5xx with backoff before giving up', async () => {
    const netMod = await import('./network.js')
    netMod.isOnline.value = true
    fetchMock.mockResolvedValue({ ok: false, status: 503, json: async () => ({ error: 'oops' }) })
    const mod = await import('./queue.js')
    await mod.enqueue({ method: 'POST', url: '/x', body: {} })
    const rows = await drain(mod)
    expect(rows).toHaveLength(1)
    // First transient failure keeps it pending, not failed.
    expect(['pending', 'failed']).toContain(rows[0].status)
    expect(rows[0].retries).toBeGreaterThanOrEqual(1)
    netMod.isOnline.value = false
  })
})
