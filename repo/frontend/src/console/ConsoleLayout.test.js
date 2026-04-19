import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import { ref, h } from 'vue'

vi.mock('../composables/useAuth.js', () => ({
  useAuth: () => ({ user: ref({ username: 'admin', role: 'administrator' }) }),
}))

// SideNav internally uses useRbac which reads the user — stub it so this
// suite stays focused on ConsoleLayout's header + router-view.
vi.mock('./SideNav.vue', () => ({
  default: { name: 'SideNav', render: () => h('nav', { class: 'stub-sidenav' }, 'side') },
}))

function makeRouter(path = '/console/dashboard', name = 'console.dashboard') {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/console/dashboard', name, component: { render: () => h('p', 'inner-dashboard') } },
    ],
  })
}

describe('ConsoleLayout.vue', () => {
  it('renders the side nav, header, and the matched child route', async () => {
    const { default: ConsoleLayout } = await import('./ConsoleLayout.vue')
    const router = makeRouter()
    await router.push('/console/dashboard')
    await router.isReady()
    const w = mount(ConsoleLayout, { global: { plugins: [router] } })
    expect(w.find('.stub-sidenav').exists()).toBe(true)
    expect(w.text()).toContain('inner-dashboard')
  })

  it('shows the signed-in username and role in the header', async () => {
    const { default: ConsoleLayout } = await import('./ConsoleLayout.vue')
    const router = makeRouter()
    await router.push('/console/dashboard')
    await router.isReady()
    const w = mount(ConsoleLayout, { global: { plugins: [router] } })
    expect(w.find('.role').text()).toBe('admin · administrator')
  })
})
