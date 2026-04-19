<script setup>
// Offline-first content pack download. Any authenticated user (including
// members) can use this page to pull the current content pack and keep it
// in the browser for offline reading.
//
// Uses `offline/download.js` which handles HTTP Range resumption keyed by
// ETag — if the user hits Pause or the network drops mid-stream, the next
// Start call resumes where the last chunk left off. Pausing is implemented
// by racing the download promise against a cancel signal.
import { ref, computed, onUnmounted } from 'vue'
import { resumableDownload, clearDownload } from '../../offline/download.js'
import { isOnline } from '../../offline/network.js'

const CONTENT_PACK_URL = '/api/v1/content-packs/current'
const CACHE_KEY = 'content-pack:current'

const state = ref('idle') // idle | running | paused | complete | error
const downloaded = ref(0)
const total = ref(0)
const errorMessage = ref('')

// Abort handle for the in-flight download. Pause calls .abort() which
// surfaces as an `AbortError` inside resumableDownload; the outer catch
// classifies that as "paused" rather than a hard error.
let aborter = null

const percent = computed(() => {
  if (!total.value) return 0
  return Math.min(100, Math.round((downloaded.value / total.value) * 100))
})

const humanSize = (n) => {
  if (!n) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let v = n, i = 0
  while (v >= 1024 && i < units.length - 1) { v /= 1024; i++ }
  return `${v.toFixed(1)} ${units[i]}`
}

async function start() {
  if (state.value === 'running') return
  if (!isOnline.value) {
    errorMessage.value = 'Cannot start download while offline.'
    state.value = 'error'
    return
  }
  state.value = 'running'
  errorMessage.value = ''
  aborter = new AbortController()

  try {
    await resumableDownload({
      url: CONTENT_PACK_URL,
      key: CACHE_KEY,
      signal: aborter.signal,
      onProgress: ({ downloaded: d, total: t }) => {
        downloaded.value = d
        total.value = t
      },
    })
    state.value = 'complete'
  } catch (e) {
    if (e?.name === 'AbortError' || e?.message === 'aborted') {
      state.value = 'paused'
      return
    }
    errorMessage.value = e?.message || 'download failed'
    state.value = 'error'
  } finally {
    aborter = null
  }
}

function pause() {
  if (aborter) aborter.abort()
  state.value = 'paused'
}

async function reset() {
  pause()
  await clearDownload(CACHE_KEY)
  downloaded.value = 0
  total.value = 0
  state.value = 'idle'
  errorMessage.value = ''
}

// If the user navigates away mid-download, abort so the browser's fetch
// cancel is honoured — no point holding the connection open.
onUnmounted(() => { if (aborter) aborter.abort() })
</script>

<template>
  <div class="pack">
    <h3>Content pack (offline archive)</h3>
    <p class="hint">
      Download the current content pack so search &amp; reading work without a
      network connection. If the download is interrupted, it resumes from the
      last chunk when you click Start again.
    </p>

    <div class="progress" v-if="total > 0 || state !== 'idle'">
      <div class="bar">
        <div class="fill" :style="{ width: percent + '%' }" data-test="dl-fill"></div>
      </div>
      <p class="stats" data-test="dl-stats">
        <strong>{{ percent }}%</strong>
        — {{ humanSize(downloaded) }} of {{ humanSize(total) }}
        <span class="muted">· state: {{ state }}</span>
      </p>
    </div>

    <div class="actions">
      <button
        v-if="state !== 'running'"
        class="primary"
        @click="start"
        :disabled="!isOnline || state === 'complete'"
        data-test="dl-start"
      >
        {{ state === 'paused' ? 'Resume' : (state === 'complete' ? 'Done' : 'Start') }}
      </button>
      <button v-else class="warn" @click="pause" data-test="dl-pause">Pause</button>
      <button class="ghost" @click="reset" data-test="dl-reset">Reset</button>
    </div>

    <p v-if="errorMessage" class="err" data-test="dl-error">{{ errorMessage }}</p>
    <p v-if="!isOnline" class="offline">
      You are offline — start requires an active connection. A previously paused
      download can be resumed from cache once you are back online.
    </p>
  </div>
</template>

<style scoped>
.pack {
  max-width: 48em;
  margin: 2em auto;
  padding: 1.5em;
  background: #fff;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  display: flex;
  flex-direction: column;
  gap: 0.75em;
}
.hint { color: #6b7280; margin: 0; }
.bar {
  width: 100%;
  height: 10px;
  background: #e5e7eb;
  border-radius: 999px;
  overflow: hidden;
}
.fill {
  height: 100%;
  background: #2563eb;
  transition: width 0.2s ease;
}
.stats { margin: 0; font-size: 0.9em; color: #111827; }
.muted { color: #6b7280; }
.actions { display: flex; gap: 0.5em; flex-wrap: wrap; }
.actions button {
  padding: 0.4em 1.2em;
  border-radius: 4px;
  font-weight: 600;
  cursor: pointer;
  border: 1px solid transparent;
}
.actions button:disabled { opacity: 0.5; cursor: not-allowed; }
.primary { background: #2563eb; color: white; border-color: #2563eb; }
.warn    { background: #f59e0b; color: white; border-color: #f59e0b; }
.ghost   { background: #fff; color: #374151; border-color: #d1d5db; }
.err     { color: #b91c1c; margin: 0; }
.offline { color: #b91c1c; margin: 0; font-size: 0.85em; }
</style>
