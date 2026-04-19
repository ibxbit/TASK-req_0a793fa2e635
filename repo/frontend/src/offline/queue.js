import { ref } from 'vue'
import { put, get, del, all } from './db.js'
import { isOnline, onNetworkChange } from './network.js'

const STORE = 'queue'
const MAX_RETRIES = 5
const PROCESS_INTERVAL_MS = 10_000

export const queueState = ref({ count: 0, pending: 0, failed: 0 })

function uuid() {
  if (typeof crypto !== 'undefined' && crypto.randomUUID) return crypto.randomUUID()
  return 'q-' + Date.now().toString(36) + '-' + Math.random().toString(36).slice(2, 10)
}

async function refreshState() {
  const items = await all(STORE)
  const pending = items.filter(i => i.status === 'pending' || i.status === 'in_flight').length
  const failed = items.filter(i => i.status === 'failed').length
  queueState.value = { count: items.length, pending, failed }
  return items
}

export async function enqueue({ method, url, body, kind, idempotencyKey }) {
  const entry = {
    id: uuid(),
    idempotency_key: idempotencyKey || uuid(),
    method,
    url,
    body: body ?? null,
    kind: kind || 'generic',
    status: 'pending',
    retries: 0,
    next_retry_at: Date.now(),
    created_at: Date.now(),
    updated_at: Date.now(),
    error: null,
  }
  await put(STORE, entry)
  await refreshState()
  processQueue().catch(() => {})
  return entry
}

export async function listQueue() {
  return all(STORE)
}

export async function removeFromQueue(id) {
  await del(STORE, id)
  await refreshState()
}

export async function retryEntry(id) {
  const entry = await get(STORE, id)
  if (!entry) return
  entry.status = 'pending'
  entry.next_retry_at = Date.now()
  entry.retries = 0
  entry.error = null
  entry.updated_at = Date.now()
  await put(STORE, entry)
  await refreshState()
  processQueue().catch(() => {})
}

function backoff(retries) {
  return Math.min(60_000, 1_000 * Math.pow(2, retries))
}

async function attempt(entry) {
  const res = await fetch(entry.url, {
    method: entry.method,
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      'Idempotency-Key': entry.idempotency_key,
    },
    body: entry.body !== null ? JSON.stringify(entry.body) : undefined,
  })
  let payload = null
  try { payload = await res.json() } catch (_) {}
  return { ok: res.ok, status: res.status, payload }
}

let processing = false

export async function processQueue() {
  if (processing) return
  if (!isOnline.value) return
  processing = true
  try {
    const items = (await all(STORE))
      .filter(e => e.status === 'pending' && e.next_retry_at <= Date.now())
      .sort((a, b) => a.created_at - b.created_at)

    for (const e of items) {
      e.status = 'in_flight'
      e.updated_at = Date.now()
      await put(STORE, e)
      try {
        const r = await attempt(e)
        if (r.ok) {
          await del(STORE, e.id)
          continue
        }
        // 4xx (except 408/425/429) are permanent — don't keep retrying.
        const transient = r.status === 408 || r.status === 425 || r.status === 429 || r.status >= 500
        e.retries += 1
        e.error = `${r.status}: ${r.payload ? JSON.stringify(r.payload) : ''}`
        if (!transient || e.retries >= MAX_RETRIES) {
          e.status = 'failed'
        } else {
          e.status = 'pending'
          e.next_retry_at = Date.now() + backoff(e.retries)
        }
        e.updated_at = Date.now()
        await put(STORE, e)
      } catch (err) {
        e.retries += 1
        e.error = err && err.message ? err.message : String(err)
        if (e.retries >= MAX_RETRIES) {
          e.status = 'failed'
        } else {
          e.status = 'pending'
          e.next_retry_at = Date.now() + backoff(e.retries)
        }
        e.updated_at = Date.now()
        await put(STORE, e)
      }
    }
  } finally {
    processing = false
    await refreshState()
  }
}

let started = false

export function startQueueProcessor() {
  if (started) return
  started = true
  refreshState().catch(() => {})
  setInterval(() => { processQueue().catch(() => {}) }, PROCESS_INTERVAL_MS)
  onNetworkChange(ok => { if (ok) processQueue().catch(() => {}) })
  setTimeout(() => { processQueue().catch(() => {}) }, 2_000)
}
