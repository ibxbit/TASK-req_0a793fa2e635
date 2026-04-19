import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'

const state = vi.hoisted(() => ({ value: null }))
vi.mock('../composables/useSearch.js', () => {
  const query = ref('orig')
  state.value = { query }
  return { useSearch: () => state.value }
})

describe('DidYouMean.vue', () => {
  it('renders nothing when there are no suggestions', async () => {
    const { default: DidYouMean } = await import('./DidYouMean.vue')
    const w = mount(DidYouMean, { props: { suggestions: [] } })
    expect(w.find('.dym').exists()).toBe(false)
  })

  it('renders a chip per suggestion', async () => {
    const { default: DidYouMean } = await import('./DidYouMean.vue')
    const w = mount(DidYouMean, {
      props: { suggestions: [
        { term: '明月', distance: 1, source: 'synonym' },
        { term: '朙月', distance: 2, source: 'variant' },
      ] },
    })
    const chips = w.findAll('.chip')
    expect(chips).toHaveLength(2)
    expect(chips[0].text()).toBe('明月')
  })

  it('clicking a chip overwrites the shared query ref', async () => {
    const { default: DidYouMean } = await import('./DidYouMean.vue')
    const w = mount(DidYouMean, {
      props: { suggestions: [{ term: '春江', distance: 1, source: 'synonym' }] },
    })
    await w.find('.chip').trigger('click')
    expect(state.value.query.value).toBe('春江')
  })
})
