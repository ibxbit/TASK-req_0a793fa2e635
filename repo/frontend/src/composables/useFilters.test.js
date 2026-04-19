import { describe, it, expect, vi, beforeEach } from 'vitest'

const { apiGet } = vi.hoisted(() => ({ apiGet: vi.fn() }))
vi.mock('../offline/api.js', () => ({ apiGet }))

beforeEach(() => { apiGet.mockReset() })

describe('useFilters', () => {
  it('populates authors/dynasties/tags from API results', async () => {
    apiGet
      .mockResolvedValueOnce({ data: { items: [{ id: 1, name: 'LiBai' }] } })
      .mockResolvedValueOnce({ data: { items: [{ id: 2, name: 'Tang' }] } })
      .mockResolvedValueOnce({ data: { items: [{ id: 3, name: 'lyric' }] } })
    const { loadFilters, useFilters } = await import('./useFilters.js')
    await loadFilters()
    const { authors, dynasties, tags, loaded } = useFilters()
    expect(authors.value).toEqual([{ id: 1, name: 'LiBai' }])
    expect(dynasties.value[0].name).toBe('Tang')
    expect(tags.value[0].name).toBe('lyric')
    expect(loaded.value).toBe(true)
  })

  it('is idempotent — a second call is a no-op', async () => {
    apiGet
      .mockResolvedValue({ data: { items: [] } })
    const { loadFilters } = await import('./useFilters.js')
    await loadFilters()
    const firstCalls = apiGet.mock.calls.length
    await loadFilters()
    expect(apiGet.mock.calls.length).toBe(firstCalls)
  })

  it('never rejects when the API errors out — returns empty lists', async () => {
    apiGet.mockRejectedValue(new Error('boom'))
    const { loadFilters, useFilters } = await import('./useFilters.js')
    await loadFilters()
    const { authors, dynasties, tags } = useFilters()
    expect(authors.value).toEqual([])
    expect(dynasties.value).toEqual([])
    expect(tags.value).toEqual([])
  })
})
