import { ref, reactive, watch } from 'vue'
import axios from 'axios'
import { cacheGet, cacheSet } from '../offline/cache.js'
import { isOnline } from '../offline/network.js'

const DEBOUNCE_MS = 300
const CACHE_TTL_MS = 5 * 60 * 1000

const client = axios.create({ withCredentials: true })

const query = ref('')
const filters = reactive({
  author_id: null,
  dynasty_id: null,
  tag_id: null,
  meter_id: null,
  snippet: '',
})
const options = reactive({
  highlight: true,
  syn: false,
  cjk: false,
})
const pagination = reactive({
  limit: 20,
  offset: 0,
})

const results = ref({ hits: [], count: 0, did_you_mean: [], query: '', options: null })
const loading = ref(false)
const error = ref('')
const fromCache = ref(false)

let debounceTimer = null
let activeController = null

function buildParams() {
  const p = { q: query.value, limit: pagination.limit, offset: pagination.offset }
  for (const [k, v] of Object.entries(filters)) {
    if (v !== null && v !== '' && v !== undefined) p[k] = v
  }
  for (const [k, v] of Object.entries(options)) {
    if (v) p[k] = 1
  }
  return p
}

function cacheKeyFor(params) {
  const qs = new URLSearchParams(params).toString()
  return `search:${qs}`
}

async function runSearch() {
  const params = buildParams()
  const key = cacheKeyFor(params)

  // abort in-flight
  if (activeController) { try { activeController.abort() } catch {} }
  activeController = new AbortController()
  const signal = activeController.signal

  loading.value = true
  error.value = ''
  fromCache.value = false

  // Offline: use cache or fail gracefully
  if (!isOnline.value) {
    const cached = await cacheGet(key)
    loading.value = false
    if (cached) {
      results.value = cached
      fromCache.value = true
    } else {
      error.value = 'offline and no cached result for this query'
      results.value = { hits: [], count: 0, did_you_mean: [], query: query.value, options: null }
    }
    return
  }

  try {
    const res = await client.get('/api/v1/search', { params, signal })
    if (signal.aborted) return
    results.value = res.data
    await cacheSet(key, res.data, CACHE_TTL_MS)
  } catch (e) {
    if (e?.name === 'CanceledError' || signal.aborted) return
    // fall back to cache on network error
    const cached = await cacheGet(key)
    if (cached) {
      results.value = cached
      fromCache.value = true
    } else {
      error.value = e?.response?.data?.error || e?.message || 'search failed'
      results.value = { hits: [], count: 0, did_you_mean: [], query: query.value, options: null }
    }
  } finally {
    if (!signal.aborted) loading.value = false
  }
}

function schedule() {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(runSearch, DEBOUNCE_MS)
}

// Debounce on query changes; immediate on filter/option changes.
watch(query, () => { pagination.offset = 0; schedule() })
watch([() => ({ ...filters }), () => ({ ...options })], () => {
  pagination.offset = 0
  runSearch()
}, { deep: true })
watch(() => pagination.offset, () => { runSearch() })

export function useSearch() {
  return {
    query, filters, options, pagination,
    results, loading, error, fromCache,
    run: runSearch,
  }
}
