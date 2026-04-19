import { describe, it, expect, vi, beforeEach } from 'vitest'

// Hoisted so the factory below can reach them safely. Vitest hoists vi.mock()
// to the top of the module so declared-after variables aren't always visible
// to the factory function.
const { axiosRequest, axiosGet } = vi.hoisted(() => ({
  axiosRequest: vi.fn(),
  axiosGet: vi.fn(),
}))

vi.mock('./network.js', () => ({
  isOnline: { value: true },
  onNetworkChange: vi.fn(),
}))

vi.mock('axios', () => ({
  default: {
    create: () => ({
      request: (cfg) => axiosRequest(cfg),
      get: (url, opts) => axiosGet(url, opts),
    }),
  },
}))

beforeEach(() => {
  axiosRequest.mockReset()
  axiosGet.mockReset()
})

describe('offline/api apiWrite', () => {
  it('sends request online and returns data', async () => {
    axiosRequest.mockResolvedValueOnce({ data: { id: 7 } })
    const { apiWrite } = await import('./api.js')
    const res = await apiWrite({ method: 'POST', url: '/api/v1/reviews', body: { r: 1 } })
    expect(res.queued).toBe(false)
    expect(res.data).toEqual({ id: 7 })
    const cfg = axiosRequest.mock.calls[0][0]
    expect(cfg.method).toBe('POST')
    expect(cfg.url).toBe('/api/v1/reviews')
    expect(cfg.headers['Idempotency-Key']).toBeTruthy()
  })

  it('queues a write when offline', async () => {
    const net = await import('./network.js')
    net.isOnline.value = false
    const { apiWrite } = await import('./api.js')
    const res = await apiWrite({ method: 'POST', url: '/api/v1/reviews', body: { r: 1 }, kind: 'review' })
    expect(res.queued).toBe(true)
    expect(res.entry.method).toBe('POST')
    expect(res.entry.url).toBe('/api/v1/reviews')
    expect(res.entry.kind).toBe('review')
    expect(axiosRequest).not.toHaveBeenCalled()
    net.isOnline.value = true
  })

  it('queues a write on network error (no response)', async () => {
    axiosRequest.mockRejectedValueOnce(Object.assign(new Error('net'), { response: undefined }))
    const { apiWrite } = await import('./api.js')
    const res = await apiWrite({ method: 'POST', url: '/api/v1/complaints', body: {} })
    expect(res.queued).toBe(true)
  })

  it('rethrows 4xx server rejects rather than queueing', async () => {
    axiosRequest.mockRejectedValueOnce({ response: { status: 400, data: { error: 'bad' } } })
    const { apiWrite } = await import('./api.js')
    await expect(
      apiWrite({ method: 'POST', url: '/x', body: {} })
    ).rejects.toMatchObject({ response: { status: 400 } })
  })
})

describe('offline/api apiGet', () => {
  it('returns live data online and caches it', async () => {
    axiosGet.mockResolvedValueOnce({ data: { items: [{ id: 1 }] } })
    const { apiGet } = await import('./api.js')
    const res = await apiGet('/api/v1/authors', { cacheKey: 'authors' })
    expect(res.fromCache).toBe(false)
    expect(res.data.items).toHaveLength(1)
  })

  it('falls back to cache when offline', async () => {
    const net = await import('./network.js')
    net.isOnline.value = true
    axiosGet.mockResolvedValueOnce({ data: { v: 1 } })
    const api = await import('./api.js')
    await api.apiGet('/api/v1/tags', { cacheKey: 'tags' })

    net.isOnline.value = false
    const res = await api.apiGet('/api/v1/tags', { cacheKey: 'tags' })
    expect(res.fromCache).toBe(true)
    expect(res.data).toEqual({ v: 1 })
    net.isOnline.value = true
  })

  it('throws when offline and nothing cached', async () => {
    const net = await import('./network.js')
    net.isOnline.value = false
    const api = await import('./api.js')
    await expect(api.apiGet('/nope', { cacheKey: 'nope' })).rejects.toThrow(/offline/)
    net.isOnline.value = true
  })
})
