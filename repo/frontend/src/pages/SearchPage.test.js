import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref, reactive } from 'vue'

// SearchPage composes SearchBar + FilterPanel + ResultsList + DidYouMean.
// We mock the composables shared by those children so the page can mount
// under jsdom without touching axios or IndexedDB.
const state = vi.hoisted(() => ({ value: null }))

vi.mock('../composables/useSearch.js', () => {
  const query = ref('')
  const filters = reactive({ author_id: null, dynasty_id: null, tag_id: null, meter_id: null, snippet: '' })
  const options = reactive({ highlight: true, syn: false, cjk: false })
  const results = ref({ hits: [], count: 0, did_you_mean: [] })
  const loading = ref(false)
  const error = ref('')
  const fromCache = ref(false)
  const pagination = reactive({ limit: 20, offset: 0 })
  state.value = { query, filters, options, results, loading, error, fromCache, pagination }
  return { useSearch: () => state.value }
})

vi.mock('../composables/useFilters.js', () => ({
  useFilters: () => ({
    authors: ref([]),
    dynasties: ref([]),
    tags: ref([]),
    loading: ref(false),
  }),
  loadFilters: vi.fn(),
}))

describe('SearchPage.vue', () => {
  it('mounts the SearchBar + FilterPanel + ResultsList hierarchy', async () => {
    const { default: SearchPage } = await import('./SearchPage.vue')
    const w = mount(SearchPage)
    // SearchBar input (search), FilterPanel (Clear button), ResultsList (Start typing hint)
    expect(w.find('input[type="search"]').exists()).toBe(true)
    expect(w.find('.filters').exists()).toBe(true)
    expect(w.text()).toContain('Start typing to search')
  })

  it('propagates typing into the shared query ref', async () => {
    const { default: SearchPage } = await import('./SearchPage.vue')
    const w = mount(SearchPage)
    await w.find('input[type="search"]').setValue('静夜思')
    expect(state.value.query.value).toBe('静夜思')
  })
})
