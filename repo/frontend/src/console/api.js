import axios from 'axios'
import { apiWrite } from '../offline/api.js'

const BASE = '/api/v1'

export const http = axios.create({
  baseURL: BASE,
  withCredentials: true,
})

/**
 * Mutation helpers route through the offline queue so edits survive network
 * drops. All three return the server payload when online, or null when the
 * request was queued (offline / transient network error). Callers that only
 * need the side-effect (create → reload list) are unaffected by the null.
 */
export async function post(url, body) {
  const res = await apiWrite({ method: 'POST', url: BASE + url, body, kind: 'edit' })
  return res.data ?? null
}

export async function put(url, body) {
  const res = await apiWrite({ method: 'PUT', url: BASE + url, body, kind: 'edit' })
  return res.data ?? null
}

export async function del(url) {
  const res = await apiWrite({ method: 'DELETE', url: BASE + url, body: null, kind: 'edit' })
  return res.data ?? null
}

export async function get(url, params) {
  const r = await http.get(url, { params })
  return r.data
}
