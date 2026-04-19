import axios from 'axios'
import { cacheGet, cacheSet } from './cache.js'
import { enqueue } from './queue.js'
import { isOnline } from './network.js'

const client = axios.create({ baseURL: '', withCredentials: true })

function genIdempotencyKey() {
  if (typeof crypto !== 'undefined' && crypto.randomUUID) return crypto.randomUUID()
  return 'k-' + Date.now().toString(36) + '-' + Math.random().toString(36).slice(2, 10)
}

/**
 * GET with cache fallback. Returns cached data if offline or request fails.
 */
export async function apiGet(url, { cacheKey, ttlMs, params } = {}) {
  const key = cacheKey || url + (params ? '?' + new URLSearchParams(params).toString() : '')
  if (!isOnline.value) {
    const cached = await cacheGet(key)
    if (cached !== null) return { data: cached, fromCache: true }
    throw new Error('offline and no cached data for ' + key)
  }
  try {
    const res = await client.get(url, { params })
    await cacheSet(key, res.data, ttlMs)
    return { data: res.data, fromCache: false }
  } catch (err) {
    const cached = await cacheGet(key)
    if (cached !== null) return { data: cached, fromCache: true }
    throw err
  }
}

/**
 * Mutation with queue fallback. Supported kinds: 'review', 'complaint', 'edit', 'generic'.
 * When offline OR the request fails with a network error, the action is persisted
 * to the offline queue for later retry. The server dedupes by Idempotency-Key.
 */
export async function apiWrite({ method, url, body, kind = 'generic', queueIfOffline = true }) {
  const idempotencyKey = genIdempotencyKey()
  if (!isOnline.value && queueIfOffline) {
    const entry = await enqueue({ method, url, body, kind, idempotencyKey })
    return { queued: true, entry }
  }
  try {
    const res = await client.request({
      method, url, data: body,
      headers: { 'Idempotency-Key': idempotencyKey },
    })
    return { queued: false, data: res.data }
  } catch (err) {
    const isNetwork = !err.response
    if (isNetwork && queueIfOffline) {
      const entry = await enqueue({ method, url, body, kind, idempotencyKey })
      return { queued: true, entry }
    }
    throw err
  }
}

export const offlineClient = client
