import { describe, it, expect } from 'vitest'
import { cacheSet, cacheGet, cacheDelete, cacheList } from './cache.js'

describe('offline/cache', () => {
  it('stores and retrieves a value under a key', async () => {
    await cacheSet('k1', { hello: 'world' }, 60_000)
    expect(await cacheGet('k1')).toEqual({ hello: 'world' })
  })

  it('returns null for unknown keys', async () => {
    expect(await cacheGet('no-such-key')).toBeNull()
  })

  it('expires entries whose TTL has elapsed and cleans them up', async () => {
    await cacheSet('stale', 'x', -1)
    expect(await cacheGet('stale')).toBeNull()
    const rows = await cacheList()
    expect(rows.find(r => r.key === 'stale')).toBeUndefined()
  })

  it('deletes entries by key', async () => {
    await cacheSet('doomed', 42)
    await cacheDelete('doomed')
    expect(await cacheGet('doomed')).toBeNull()
  })

  it('TTL=0 sentinel means never expires', async () => {
    await cacheSet('sticky', 'forever', 0)
    expect(await cacheGet('sticky')).toBe('forever')
  })
})
