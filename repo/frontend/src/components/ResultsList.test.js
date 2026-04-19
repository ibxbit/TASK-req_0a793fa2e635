import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref, reactive } from 'vue'

const state = vi.hoisted(() => ({ value: null }))

vi.mock('../composables/useSearch.js', () => {
  const results = ref({ hits: [], count: 0, did_you_mean: [], query: '', options: null })
  const loading = ref(false)
  const error = ref('')
  const fromCache = ref(false)
  const pagination = reactive({ limit: 20, offset: 0 })
  const query = ref('')
  state.value = { results, loading, error, fromCache, pagination, query }
  return { useSearch: () => state.value }
})

describe('ResultsList.vue', () => {
  it('shows the "Start typing" hint when there is no query', async () => {
    const { default: ResultsList } = await import('./ResultsList.vue')
    const w = mount(ResultsList)
    expect(w.text()).toContain('Start typing to search')
  })

  it('shows the "cached" tag when fromCache is true', async () => {
    state.value.fromCache.value = true
    state.value.query.value = 'q'
    state.value.results.value = { hits: [{ poem_id: 1, title: 't', score: 0 }], count: 1, did_you_mean: [] }
    const { default: ResultsList } = await import('./ResultsList.vue')
    const w = mount(ResultsList)
    expect(w.find('.tag').exists()).toBe(true)
    expect(w.find('.tag').text()).toContain('cached')
    state.value.fromCache.value = false
  })

  it('surfaces error text in the dedicated error region', async () => {
    state.value.error.value = 'something failed'
    state.value.query.value = 'x'
    const { default: ResultsList } = await import('./ResultsList.vue')
    const w = mount(ResultsList)
    expect(w.find('.error').text()).toBe('something failed')
    state.value.error.value = ''
  })

  it('renders a ResultCard per hit and pages through results', async () => {
    state.value.results.value = {
      hits: [
        { poem_id: 1, title: 'one', score: 1.0 },
        { poem_id: 2, title: 'two', score: 0.5 },
      ],
      count: 42,
      did_you_mean: [],
    }
    state.value.query.value = 'q'
    state.value.pagination.offset = 0
    const { default: ResultsList } = await import('./ResultsList.vue')
    const w = mount(ResultsList)
    // 1–2 of 42 rendered in the summary bar.
    expect(w.find('.bar').text()).toContain('Showing 1–2 of 42')
    // Prev is disabled at offset 0, Next is disabled because we have fewer hits than limit.
    const [prev, next] = w.findAll('.pager button')
    expect(prev.attributes('disabled')).toBeDefined()
    expect(next.attributes('disabled')).toBeDefined()
  })
})
