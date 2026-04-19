import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import HighlightedText from './HighlightedText.vue'

describe('HighlightedText.vue', () => {
  it('renders plain text when no <mark> tags', () => {
    const w = mount(HighlightedText, { props: { text: 'hello world' } })
    expect(w.text()).toBe('hello world')
    expect(w.find('mark').exists()).toBe(false)
  })

  it('wraps the highlighted segment in a real <mark> element', () => {
    const w = mount(HighlightedText, { props: { text: 'hello <mark>brave</mark> world' } })
    expect(w.text()).toBe('hello brave world')
    const mark = w.find('mark')
    expect(mark.exists()).toBe(true)
    expect(mark.text()).toBe('brave')
  })

  it('supports multiple highlights in one string', () => {
    const w = mount(HighlightedText, { props: { text: '<mark>a</mark> and <mark>b</mark>' } })
    const marks = w.findAll('mark')
    expect(marks.length).toBe(2)
    expect(marks[0].text()).toBe('a')
    expect(marks[1].text()).toBe('b')
  })

  it('falls back to the fallback prop when text is empty', () => {
    const w = mount(HighlightedText, { props: { text: '', fallback: 'nothing' } })
    expect(w.text()).toBe('nothing')
  })
})
