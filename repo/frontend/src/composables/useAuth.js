import { ref, computed } from 'vue'
import axios from 'axios'

const user = ref(null)
const ready = ref(false)
const loginError = ref('')

const client = axios.create({ withCredentials: true })

export async function checkSession() {
  try {
    const res = await client.get('/api/v1/auth/me')
    user.value = res.data.user
  } catch {
    user.value = null
  } finally {
    ready.value = true
  }
}

export async function login(username, password) {
  loginError.value = ''
  try {
    const res = await client.post('/api/v1/auth/login', { username, password })
    user.value = res.data.user
    return true
  } catch (e) {
    loginError.value = e?.response?.data?.error || 'login failed'
    return false
  }
}

export async function logout() {
  try { await client.post('/api/v1/auth/logout') } catch {}
  user.value = null
}

export function useAuth() {
  return {
    user,
    ready,
    loginError,
    isAuthenticated: computed(() => user.value !== null),
  }
}
