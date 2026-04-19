import { ref } from 'vue'
import { apiGet } from '../offline/api.js'

const authors = ref([])
const dynasties = ref([])
const tags = ref([])
const loaded = ref(false)
const loading = ref(false)

const DAY = 24 * 60 * 60 * 1000

async function loadList(url, cacheKey) {
  try {
    const { data } = await apiGet(url, { cacheKey, ttlMs: DAY, params: { limit: 500 } })
    return data?.items || []
  } catch {
    return []
  }
}

export async function loadFilters() {
  if (loaded.value || loading.value) return
  loading.value = true
  try {
    const [a, d, t] = await Promise.all([
      loadList('/api/v1/authors',  'filters:authors'),
      loadList('/api/v1/dynasties','filters:dynasties'),
      loadList('/api/v1/tags',     'filters:tags'),
    ])
    authors.value = a
    dynasties.value = d
    tags.value = t
    loaded.value = true
  } finally {
    loading.value = false
  }
}

export function useFilters() {
  return { authors, dynasties, tags, loaded, loading }
}
