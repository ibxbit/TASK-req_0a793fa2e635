import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import ResultCard from './ResultCard.vue'

describe('ResultCard.vue', () => {
  it('renders title, score, and matched fields', () => {
    const w = mount(ResultCard, {
      props: {
        hit: {
          poem_id: 1,
          title: 'Quiet Night',
          score: 1.2345,
          matched_fields: ['title', 'content'],
          first_line: '床前明月光',
        },
      },
    })
    expect(w.text()).toContain('Quiet Night')
    expect(w.text()).toContain('1.23') // toFixed(2)
    expect(w.text()).toContain('matched: title, content')
    expect(w.text()).toContain('床前明月光')
  })

  it('prefers title_highlighted over title and renders <mark>', () => {
    const w = mount(ResultCard, {
      props: {
        hit: {
          poem_id: 2,
          title: 'Plain Title',
          title_highlighted: '<mark>Bright</mark> Moon',
          score: 0.5,
        },
      },
    })
    expect(w.find('mark').exists()).toBe(true)
    expect(w.find('mark').text()).toBe('Bright')
    expect(w.text()).not.toContain('Plain Title')
  })

  it('renders snippet when provided', () => {
    const w = mount(ResultCard, {
      props: { hit: { poem_id: 3, title: 't', score: 0, snippet: 'sample <mark>snippet</mark>' } },
    })
    expect(w.find('.snippet').exists()).toBe(true)
    expect(w.find('.snippet mark').text()).toBe('snippet')
  })

  it('omits first-line block when neither field is set', () => {
    const w = mount(ResultCard, {
      props: { hit: { poem_id: 4, title: 't', score: 0 } },
    })
    expect(w.find('.first-line').exists()).toBe(false)
  })
})
