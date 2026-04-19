import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'

const { login } = vi.hoisted(() => ({ login: vi.fn() }))
vi.mock('../composables/useAuth.js', () => ({
  login,
  useAuth: () => ({ loginError: { value: '' } }),
}))

beforeEach(() => { login.mockReset() })

describe('LoginForm.vue', () => {
  it('calls login with entered credentials on submit', async () => {
    login.mockResolvedValueOnce(true)
    const { default: LoginForm } = await import('./LoginForm.vue')
    const wrapper = mount(LoginForm)
    await wrapper.find('input[autocomplete="username"]').setValue('editor')
    await wrapper.find('input[type="password"]').setValue('editor123')
    await wrapper.find('form').trigger('submit.prevent')
    expect(login).toHaveBeenCalledWith('editor', 'editor123')
  })

  it('disables the submit button while request is in flight', async () => {
    let resolver
    login.mockImplementationOnce(() => new Promise(r => { resolver = r }))
    const { default: LoginForm } = await import('./LoginForm.vue')
    const wrapper = mount(LoginForm)
    await wrapper.find('input[autocomplete="username"]').setValue('x')
    await wrapper.find('input[type="password"]').setValue('y')
    const p = wrapper.find('form').trigger('submit.prevent')
    await wrapper.vm.$nextTick()
    expect(wrapper.find('button').attributes('disabled')).toBeDefined()
    resolver(true)
    await p
  })
})
