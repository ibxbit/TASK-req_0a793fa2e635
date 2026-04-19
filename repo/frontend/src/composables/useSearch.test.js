import { describe, it, expect, vi, beforeEach } from 'vitest'

// Mock axios create so useSearch hits our controllable stub, not a real URL.
const { axiosGet } = vi.hoisted(() => ({ axiosGet: vi.fn() }))
vi.mock('axios', () => ({
  default: { create: () => ({ get: (url, opts) => axiosGet(url, opts) }) },
}))

// Keep the offline cache isolated from queue.test.js — we use real fake-indexeddb.
vi.mock('../offline/network.js', () => ({
  isOnline: { value: true },
  onNetworkChange: vi.fn(),
}))

beforeEach(() => { axiosGet.mockReset() })

describe('useSearch', () => {
  it('runSearch online hits /api/v1/search with query + active options', async () => {
    axiosGet.mockResolvedValueOnce({ data: { hits: [{ poem_id: 1 }], count: 1, did_you_mean: [] } })
    const { useSearch } = await import('./useSearch.js')
    const { query, options, run, results } = useSearch()
    query.value = '春'
    options.highlight = true
    await run()
    expect(axiosGet).toHaveBeenCalledTimes(1)
    const [url, opts] = axiosGet.mock.calls[0]
    expect(url).toBe('/api/v1/search')
    expect(opts.params.q).toBe('春')
    expect(opts.params.limit).toBe(20)
    expect(opts.params.highlight).toBe(1)
    expect(results.value.hits).toEqual([{ poem_id: 1 }])
  })

  it('only forwards options that are truthy (no zeros in params)', async () => {
    axiosGet.mockResolvedValueOnce({ data: { hits: [], count: 0, did_you_mean: [] } })
    const { useSearch } = await import('./useSearch.js')
    const { options, query, run } = useSearch()
    query.value = 'x'
    options.highlight = false
    options.syn = true
    options.cjk = false
    await run()
    const [, opts] = axiosGet.mock.calls[0]
    expect(opts.params.highlight).toBeUndefined()
    expect(opts.params.syn).toBe(1)
    expect(opts.params.cjk).toBeUndefined()
  })

  it('falls back to cached result on offline with empty error when cache is empty', async () => {
    const net = await import('../offline/network.js')
    net.isOnline.value = false
    const { useSearch } = await import('./useSearch.js')
    const { query, run, results, error, fromCache } = useSearch()
    query.value = 'nothing-cached'
    await run()
    expect(error.value).toContain('offline')
    expect(fromCache.value).toBe(false)
    expect(results.value.hits).toEqual([])
    net.isOnline.value = true
  })

  it('falls back to cached result offline when one exists', async () => {
    // Prime cache by running once online.
    const payload = { hits: [{ poem_id: 42 }], count: 1, did_you_mean: [] }
    axiosGet.mockResolvedValueOnce({ data: payload })
    const { useSearch } = await import('./useSearch.js')
    const { query, run, results, fromCache } = useSearch()
    query.value = 'cached-term'
    await run()
    expect(results.value.hits).toEqual([{ poem_id: 42 }])

    const net = await import('../offline/network.js')
    net.isOnline.value = false
    await run()
    expect(fromCache.value).toBe(true)
    expect(results.value.hits).toEqual([{ poem_id: 42 }])
    net.isOnline.value = true
  })

  it('surfaces server errors via `error` when no cache entry exists', async () => {
    axiosGet.mockRejectedValue({ message: 'boom', response: { data: { error: 'server broke' } } })
    const { useSearch } = await import('./useSearch.js')
    const { query, run, error, results } = useSearch()
    query.value = 'zxcv-never-cached'
    await run()
    expect(error.value).toBe('server broke')
    expect(results.value.hits).toEqual([])
  })
})
