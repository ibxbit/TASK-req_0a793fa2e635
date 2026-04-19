import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref, reactive } from 'vue'

// Ship a minimal fake composable — SearchBar owns no logic, only bindings.
const state = vi.hoisted(() => {
  // `vi.hoisted` runs before imports, so we cannot yet call `ref` here — we
  // lazily fill the refs on first access from the test.
  return { value: null }
})

vi.mock('../composables/useSearch.js', () => {
  const query = ref('')
  const options = reactive({ highlight: true, syn: false, cjk: false })
  const loading = ref(false)
  state.value = { query, options, loading }
  return { useSearch: () => state.value }
})

describe('SearchBar.vue', () => {
  it('binds the input to the shared `query` ref', async () => {
    const { default: SearchBar } = await import('./SearchBar.vue')
    const w = mount(SearchBar)
    await w.find('input[type="search"]').setValue('李白')
    expect(state.value.query.value).toBe('李白')
  })

  it('flips options via the toggle checkboxes', async () => {
    const { default: SearchBar } = await import('./SearchBar.vue')
    const w = mount(SearchBar)
    // highlight is pre-checked (default true) — unticking should flip to false.
    const boxes = w.findAll('input[type="checkbox"]')
    await boxes[0].setValue(false)
    expect(state.value.options.highlight).toBe(false)
    await boxes[1].setValue(true)
    expect(state.value.options.syn).toBe(true)
    await boxes[2].setValue(true)
    expect(state.value.options.cjk).toBe(true)
  })

  it('shows "Searching…" only while loading is true', async () => {
    const { default: SearchBar } = await import('./SearchBar.vue')
    const w = mount(SearchBar)
    expect(w.text()).not.toContain('Searching')
    state.value.loading.value = true
    await w.vm.$nextTick()
    expect(w.text()).toContain('Searching')
  })
})
