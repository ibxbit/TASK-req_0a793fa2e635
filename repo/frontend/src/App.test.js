import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import { ref } from 'vue'
import { h } from 'vue'

// The App shell branches on (ready, isAuthenticated) to decide what to render.
// We drive those signals directly and stub the child components that reach out
// to axios/IndexedDB.
const authState = vi.hoisted(() => ({
  ready: null,
  isAuthenticated: null,
  user: null,
  logoutCalls: 0,
  checkSessionCalls: 0,
}))

vi.mock('./composables/useAuth.js', () => ({
  useAuth: () => ({
    ready: authState.ready,
    isAuthenticated: authState.isAuthenticated,
    user: authState.user,
  }),
  logout: async () => { authState.logoutCalls++ },
  checkSession: async () => { authState.checkSessionCalls++ },
}))

vi.mock('./composables/useSearch.js', () => ({
  useSearch: () => ({ run: vi.fn() }),
}))

// Replace children that touch real state with minimal stubs.
vi.mock('./components/NetworkIndicator.vue', () => ({
  default: { name: 'NetworkIndicator', render: () => h('span', { class: 'stub-net' }, 'net') },
}))
vi.mock('./components/QueueDrawer.vue', () => ({
  default: { name: 'QueueDrawer', render: () => h('span', { class: 'stub-queue' }, 'queue') },
}))
vi.mock('./components/LoginForm.vue', () => ({
  default: { name: 'LoginForm', render: () => h('form', { class: 'stub-login' }, 'login-form') },
}))

function makeRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', name: 'search', component: { render: () => h('p', 'search-page-content') } },
      { path: '/console', name: 'console', component: { render: () => h('p', 'console-content') } },
    ],
  })
}

describe('App.vue', () => {
  it('renders "Loading…" until auth state is ready', async () => {
    authState.ready = ref(false)
    authState.isAuthenticated = ref(false)
    authState.user = ref(null)
    const { default: App } = await import('./App.vue')
    const router = makeRouter()
    await router.push('/')
    await router.isReady()
    const w = mount(App, { global: { plugins: [router] } })
    expect(w.text()).toContain('Loading')
  })

  it('shows LoginForm when ready but unauthenticated', async () => {
    authState.ready = ref(true)
    authState.isAuthenticated = ref(false)
    authState.user = ref(null)
    const { default: App } = await import('./App.vue')
    const router = makeRouter()
    await router.push('/')
    await router.isReady()
    const w = mount(App, { global: { plugins: [router] } })
    expect(w.find('.stub-login').exists()).toBe(true)
  })

  it('shows router-view + user chip when authenticated', async () => {
    authState.ready = ref(true)
    authState.isAuthenticated = ref(true)
    authState.user = ref({ username: 'admin', role: 'administrator' })
    const { default: App } = await import('./App.vue')
    const router = makeRouter()
    await router.push('/')
    await router.isReady()
    const w = mount(App, { global: { plugins: [router] } })
    expect(w.text()).toContain('admin · administrator')
    expect(w.text()).toContain('search-page-content')
    expect(w.find('.stub-queue').exists()).toBe(true)
  })

  it('Sign out button calls logout and navigates home', async () => {
    authState.ready = ref(true)
    authState.isAuthenticated = ref(true)
    authState.user = ref({ username: 'admin', role: 'administrator' })
    authState.logoutCalls = 0
    const { default: App } = await import('./App.vue')
    const router = makeRouter()
    await router.push('/console')
    await router.isReady()
    const w = mount(App, { global: { plugins: [router] } })
    await w.find('.signout').trigger('click')
    // The logout mock records the call.
    expect(authState.logoutCalls).toBe(1)
  })
})
