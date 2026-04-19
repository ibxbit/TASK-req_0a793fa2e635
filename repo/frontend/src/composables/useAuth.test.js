import { describe, it, expect, vi, beforeEach } from 'vitest'

// Hoist the mock functions so the factory can reach them safely regardless of
// where they're declared in this module.
const { axiosInstance } = vi.hoisted(() => ({
  axiosInstance: { get: vi.fn(), post: vi.fn() },
}))

vi.mock('axios', () => ({
  default: { create: () => axiosInstance },
}))

beforeEach(() => {
  axiosInstance.get.mockReset()
  axiosInstance.post.mockReset()
})

describe('useAuth', () => {
  it('starts unauthenticated until checkSession runs', async () => {
    const { useAuth, checkSession } = await import('./useAuth.js')
    const { isAuthenticated, ready, user } = useAuth()
    expect(isAuthenticated.value).toBe(false)
    expect(user.value).toBeNull()
    expect(ready.value).toBe(false)

    axiosInstance.get.mockResolvedValueOnce({ data: { user: { id: 1, username: 'admin', role: 'administrator' } } })
    await checkSession()
    expect(ready.value).toBe(true)
    expect(isAuthenticated.value).toBe(true)
    expect(user.value.role).toBe('administrator')
  })

  it('login success updates user and returns true', async () => {
    const { useAuth, login } = await import('./useAuth.js')
    axiosInstance.post.mockResolvedValueOnce({ data: { user: { id: 2, username: 'ed', role: 'content_editor' } } })
    const ok = await login('ed', 'pw')
    expect(ok).toBe(true)
    expect(useAuth().user.value.role).toBe('content_editor')
  })

  it('login failure sets loginError and returns false', async () => {
    const { useAuth, login } = await import('./useAuth.js')
    axiosInstance.post.mockRejectedValueOnce({ response: { data: { error: 'invalid credentials' } } })
    const ok = await login('nobody', 'wrong')
    expect(ok).toBe(false)
    expect(useAuth().loginError.value).toBe('invalid credentials')
  })

  it('logout clears user state even if request rejects', async () => {
    const { useAuth, login, logout } = await import('./useAuth.js')
    axiosInstance.post.mockResolvedValueOnce({ data: { user: { id: 3, username: 'r', role: 'reviewer' } } })
    await login('r', 'pw')
    expect(useAuth().isAuthenticated.value).toBe(true)
    axiosInstance.post.mockRejectedValueOnce(new Error('net'))
    await logout()
    expect(useAuth().isAuthenticated.value).toBe(false)
    expect(useAuth().user.value).toBeNull()
  })
})
