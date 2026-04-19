import { put, get, del } from './db.js'

const STORE = 'downloads'
const CHUNK_SIZE = 1024 * 1024 // 1 MiB

/**
 * Resumable HTTP Range download.
 *
 * Persists partial progress to IndexedDB keyed by `key` so subsequent calls
 * continue from where they left off. Server must honor Range + ETag.
 *
 * onProgress: ({ downloaded, total }) => void
 * signal:     AbortSignal — aborts rejects with AbortError, partial progress
 *             stays on disk so a subsequent call resumes from there.
 */
export async function resumableDownload({ url, key, onProgress, signal }) {
  let state = (await get(STORE, key)) || null

  const throwIfAborted = () => {
    if (signal && signal.aborted) {
      const e = new Error('aborted')
      e.name = 'AbortError'
      throw e
    }
  }
  throwIfAborted()

  const headRes = await fetch(url, { method: 'HEAD', credentials: 'include', signal })
  if (!headRes.ok) throw new Error('HEAD failed: ' + headRes.status)

  const total = parseInt(headRes.headers.get('Content-Length') || '0', 10)
  const etag = headRes.headers.get('ETag') || ''
  const acceptRanges = (headRes.headers.get('Accept-Ranges') || '').toLowerCase() === 'bytes'

  // Invalidate stale state when the server copy has changed.
  if (state && state.etag !== etag) {
    state = null
  }
  if (!state) {
    state = { key, url, etag, total, downloaded: 0, chunks: [], updated_at: Date.now() }
    await put(STORE, state)
  }

  if (!acceptRanges) {
    const res = await fetch(url, { credentials: 'include', signal })
    if (!res.ok) throw new Error('GET failed: ' + res.status)
    const buf = new Uint8Array(await res.arrayBuffer())
    state.chunks = [buf]
    state.downloaded = buf.length
    state.etag = etag
    await put(STORE, state)
    if (onProgress) onProgress({ downloaded: buf.length, total: buf.length })
    return concatChunks(state.chunks)
  }

  while (state.downloaded < total) {
    throwIfAborted()
    const start = state.downloaded
    const end = Math.min(start + CHUNK_SIZE - 1, total - 1)
    const res = await fetch(url, {
      method: 'GET',
      credentials: 'include',
      signal,
      headers: {
        'Range': `bytes=${start}-${end}`,
        'If-Match': etag,
      },
    })
    if (res.status !== 206 && res.status !== 200) {
      throw new Error('range GET failed: ' + res.status)
    }
    const buf = new Uint8Array(await res.arrayBuffer())
    state.chunks.push(buf)
    state.downloaded += buf.length
    state.updated_at = Date.now()
    await put(STORE, state)
    if (onProgress) onProgress({ downloaded: state.downloaded, total })
    // 200 means server returned full body; we're done.
    if (res.status === 200) break
  }

  return concatChunks(state.chunks)
}

export async function clearDownload(key) {
  return del(STORE, key)
}

function concatChunks(chunks) {
  const total = chunks.reduce((n, c) => n + c.length, 0)
  const out = new Uint8Array(total)
  let off = 0
  for (const c of chunks) {
    out.set(c, off)
    off += c.length
  }
  return out
}
