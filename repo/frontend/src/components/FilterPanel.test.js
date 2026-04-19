import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref, reactive } from 'vue'

const searchState = vi.hoisted(() => ({ value: null }))
const filtersState = vi.hoisted(() => ({ value: null, loadCount: 0 }))

vi.mock('../composables/useSearch.js', () => {
  const filters = reactive({
    author_id: 7, dynasty_id: 3, tag_id: 1, meter_id: 2, snippet: 'moon',
  })
  searchState.value = { filters }
  return { useSearch: () => searchState.value }
})

vi.mock('../composables/useFilters.js', () => {
  const authors = ref([{ id: 1, name: 'LiBai' }])
  const dynasties = ref([{ id: 10, name: 'Tang' }])
  const tags = ref([{ id: 100, name: 'lyric' }])
  const loading = ref(false)
  filtersState.value = { authors, dynasties, tags, loading, loadCount: 0 }
  const loadFilters = () => { filtersState.value.loadCount++ }
  return { useFilters: () => filtersState.value, loadFilters }
})

describe('FilterPanel.vue', () => {
  it('calls loadFilters on mount to populate dropdown options', async () => {
    const { default: FilterPanel } = await import('./FilterPanel.vue')
    mount(FilterPanel)
    expect(filtersState.value.loadCount).toBe(1)
  })

  it('renders <option> entries from authors/dynasties/tags', async () => {
    const { default: FilterPanel } = await import('./FilterPanel.vue')
    const w = mount(FilterPanel)
    const html = w.html()
    expect(html).toContain('LiBai')
    expect(html).toContain('Tang')
    expect(html).toContain('lyric')
  })

  it('Clear button resets every filter value to its empty state', async () => {
    const { default: FilterPanel } = await import('./FilterPanel.vue')
    const w = mount(FilterPanel)
    expect(searchState.value.filters.author_id).toBe(7)
    await w.find('button.clear').trigger('click')
    expect(searchState.value.filters.author_id).toBeNull()
    expect(searchState.value.filters.dynasty_id).toBeNull()
    expect(searchState.value.filters.tag_id).toBeNull()
    expect(searchState.value.filters.meter_id).toBeNull()
    expect(searchState.value.filters.snippet).toBe('')
  })
})
