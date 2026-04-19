import { put, get, all, del } from './db.js'

const STORE = 'cache'
const DEFAULT_TTL_MS = 24 * 60 * 60 * 1000

export async function cacheSet(key, value, ttlMs = DEFAULT_TTL_MS) {
  const entry = {
    key,
    value,
    updated_at: Date.now(),
    expires_at: ttlMs === 0 ? 0 : Date.now() + ttlMs,
  }
  await put(STORE, entry)
  return entry
}

export async function cacheGet(key) {
  const entry = await get(STORE, key)
  if (!entry) return null
  if (entry.expires_at && entry.expires_at < Date.now()) {
    await del(STORE, key)
    return null
  }
  return entry.value
}

export async function cacheList() {
  return all(STORE)
}

export async function cacheDelete(key) {
  return del(STORE, key)
}
