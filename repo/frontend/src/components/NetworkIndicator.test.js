import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'

vi.mock('../offline/network.js', async () => {
  const { ref } = await import('vue')
  return {
    isOnline: ref(true),
    onNetworkChange: vi.fn(),
  }
})

beforeEach(() => { vi.resetModules() })

describe('NetworkIndicator.vue', () => {
  it('renders Online by default', async () => {
    const net = await import('../offline/network.js')
    net.isOnline.value = true
    const { default: NetworkIndicator } = await import('./NetworkIndicator.vue')
    const wrapper = mount(NetworkIndicator)
    expect(wrapper.text()).toContain('Online')
    expect(wrapper.classes().join(' ')).not.toContain('offline')
  })

  it('renders Offline when isOnline flips false', async () => {
    const net = await import('../offline/network.js')
    net.isOnline.value = false
    const { default: NetworkIndicator } = await import('./NetworkIndicator.vue')
    const wrapper = mount(NetworkIndicator)
    expect(wrapper.text()).toContain('Offline')
  })
})
