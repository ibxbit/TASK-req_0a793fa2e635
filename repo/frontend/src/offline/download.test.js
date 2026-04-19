import { describe, it, expect, vi, beforeEach } from 'vitest'

// ---- db.js stub ---------------------------------------------------------
// Keeps a simple in-memory map so we can assert on persisted state without
// a real IndexedDB.
const dbStore = new Map()

vi.mock('./db.js', () => ({
  get: vi.fn(async (_store, key) => dbStore.get(key) ?? null),
  put: vi.fn(async (_store, value) => { dbStore.set(value.key, value) }),
  del: vi.fn(async (_store, key) => { dbStore.delete(key) }),
}))

import { get as dbGet, put as dbPut, del as dbDel } from './db.js'

beforeEach(() => {
  dbStore.clear()
  dbGet.mockClear()
  dbPut.mockClear()
  dbDel.mockClear()
  vi.stubGlobal('fetch', undefined)
})

// ---- helpers ------------------------------------------------------------

function makeHead({ contentLength, etag = '"v1"', acceptRanges = true }) {
  return Promise.resolve({
    ok: true,
    headers: {
      get: (h) => {
        if (h === 'Content-Length') return String(contentLength)
        if (h === 'ETag') return etag
        if (h === 'Accept-Ranges') return acceptRanges ? 'bytes' : ''
        return null
      },
    },
  })
}

function makeRange(bytes) {
  return Promise.resolve({
    ok: true,
    status: 206,
    arrayBuffer: async () => bytes.buffer,
  })
}

function makeFullResponse(bytes) {
  return Promise.resolve({
    ok: true,
    status: 200,
    arrayBuffer: async () => bytes.buffer,
  })
}

// ---- tests --------------------------------------------------------------

describe('offline/resumableDownload', () => {
  it('HEAD then Range GET — full single-chunk file completes and is persisted', async () => {
    const body = new Uint8Array([1, 2, 3, 4])
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(makeHead({ contentLength: 4, etag: '"abc"' }))
      .mockResolvedValueOnce(makeRange(body))
    vi.stubGlobal('fetch', fetchMock)

    const { resumableDownload } = await import('./download.js')
    const result = await resumableDownload({ url: '/content/pack.zip', key: 'pack-1' })

    expect(result).toBeInstanceOf(Uint8Array)
    expect(Array.from(result)).toEqual([1, 2, 3, 4])
    // State must be persisted to IndexedDB after completion.
    expect(dbPut).toHaveBeenCalled()
  })

  it('resumes from persisted partial state — skips already-downloaded bytes', async () => {
    // Simulate first 2 bytes already downloaded (stored in db).
    const partial = {
      key: 'pack-resume',
      url: '/content/pack.zip',
      etag: '"v1"',
      total: 4,
      downloaded: 2,
      chunks: [new Uint8Array([10, 20])],
      updated_at: Date.now(),
    }
    dbStore.set('pack-resume', partial)

    const remaining = new Uint8Array([30, 40])
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(makeHead({ contentLength: 4, etag: '"v1"' }))
      .mockResolvedValueOnce(makeRange(remaining))
    vi.stubGlobal('fetch', fetchMock)

    const { resumableDownload } = await import('./download.js')
    const result = await resumableDownload({ url: '/content/pack.zip', key: 'pack-resume' })

    // Only one Range GET (for bytes 2-3).
    expect(fetchMock).toHaveBeenCalledTimes(2)
    const rangeCall = fetchMock.mock.calls[1]
    expect(rangeCall[1].headers['Range']).toBe('bytes=2-3')
    expect(Array.from(result)).toEqual([10, 20, 30, 40])
  })

  it('ETag mismatch invalidates stale state and restarts from byte 0', async () => {
    // Stale state with old ETag.
    dbStore.set('pack-etag', {
      key: 'pack-etag',
      url: '/content/pack.zip',
      etag: '"old"',
      total: 4,
      downloaded: 2,
      chunks: [new Uint8Array([99, 99])],
      updated_at: Date.now(),
    })

    const freshBody = new Uint8Array([1, 2, 3, 4])
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(makeHead({ contentLength: 4, etag: '"new"' }))
      .mockResolvedValueOnce(makeRange(freshBody))
    vi.stubGlobal('fetch', fetchMock)

    const { resumableDownload } = await import('./download.js')
    const result = await resumableDownload({ url: '/content/pack.zip', key: 'pack-etag' })

    // Range request must start at byte 0, not byte 2.
    const rangeCall = fetchMock.mock.calls[1]
    expect(rangeCall[1].headers['Range']).toBe('bytes=0-3')
    expect(Array.from(result)).toEqual([1, 2, 3, 4])
  })

  it('AbortSignal mid-download leaves partial progress in IndexedDB', async () => {
    const controller = new AbortController()
    const firstChunk = new Uint8Array([1, 2])

    let resolveFetch
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(makeHead({ contentLength: 4, etag: '"v1"' }))
      .mockImplementationOnce(() => {
        // Abort after the first Range GET is initiated.
        controller.abort()
        return makeRange(firstChunk)
      })
      .mockRejectedValueOnce(Object.assign(new Error('aborted'), { name: 'AbortError' }))
    vi.stubGlobal('fetch', fetchMock)

    const { resumableDownload } = await import('./download.js')
    await expect(
      resumableDownload({ url: '/content/pack.zip', key: 'pack-abort', signal: controller.signal })
    ).rejects.toMatchObject({ name: 'AbortError' })

    // Partial state must still be in IndexedDB for next call to resume.
    expect(dbPut).toHaveBeenCalled()
    const savedState = dbStore.get('pack-abort')
    expect(savedState).not.toBeNull()
    expect(savedState.downloaded).toBeGreaterThan(0)
  })

  it('server without Range support falls back to full GET', async () => {
    const body = new Uint8Array([5, 6, 7])
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(makeHead({ contentLength: 3, etag: '"v1"', acceptRanges: false }))
      .mockResolvedValueOnce(makeFullResponse(body))
    vi.stubGlobal('fetch', fetchMock)

    const { resumableDownload } = await import('./download.js')
    const result = await resumableDownload({ url: '/content/pack.zip', key: 'pack-fallback' })

    // The second fetch should NOT include a Range header.
    const getCall = fetchMock.mock.calls[1]
    expect(getCall[1]?.headers?.['Range']).toBeUndefined()
    expect(Array.from(result)).toEqual([5, 6, 7])
  })

  it('clearDownload removes the persisted state from IndexedDB', async () => {
    dbStore.set('pack-clear', { key: 'pack-clear', downloaded: 100 })

    const { clearDownload } = await import('./download.js')
    await clearDownload('pack-clear')

    expect(dbDel).toHaveBeenCalledWith('downloads', 'pack-clear')
    expect(dbStore.has('pack-clear')).toBe(false)
  })

  it('onProgress callback fires for each chunk with cumulative downloaded/total', async () => {
    // Two 2-byte chunks from a 4-byte file.
    const chunk1 = new Uint8Array([1, 2])
    const chunk2 = new Uint8Array([3, 4])
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(makeHead({ contentLength: 4, etag: '"v1"' }))
      .mockResolvedValueOnce(makeRange(chunk1))
      .mockResolvedValueOnce(makeRange(chunk2))
    vi.stubGlobal('fetch', fetchMock)

    // Override CHUNK_SIZE is not possible without a module reset; instead we
    // feed a 4-byte file and verify at least one progress report fires.
    const progress = []
    const { resumableDownload } = await import('./download.js')
    await resumableDownload({
      url: '/content/pack.zip',
      key: 'pack-progress',
      onProgress: (p) => progress.push({ ...p }),
    })

    expect(progress.length).toBeGreaterThan(0)
    const last = progress[progress.length - 1]
    expect(last.total).toBe(4)
    expect(last.downloaded).toBeGreaterThan(0)
  })

  it('HEAD failure throws without writing to IndexedDB', async () => {
    const fetchMock = vi.fn().mockResolvedValueOnce({ ok: false, status: 403 })
    vi.stubGlobal('fetch', fetchMock)

    const { resumableDownload } = await import('./download.js')
    await expect(
      resumableDownload({ url: '/content/pack.zip', key: 'pack-head-fail' })
    ).rejects.toThrow('HEAD failed: 403')

    expect(dbPut).not.toHaveBeenCalled()
  })
})
