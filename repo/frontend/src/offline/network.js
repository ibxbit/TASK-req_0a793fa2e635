import { ref, readonly } from 'vue'

const online = ref(typeof navigator !== 'undefined' ? navigator.onLine : true)

const listeners = new Set()

function emit() {
  for (const cb of listeners) {
    try { cb(online.value) } catch (_) {}
  }
}

if (typeof window !== 'undefined') {
  window.addEventListener('online', () => { online.value = true; emit() })
  window.addEventListener('offline', () => { online.value = false; emit() })
}

export const isOnline = readonly(online)

export function onNetworkChange(cb) {
  listeners.add(cb)
  return () => listeners.delete(cb)
}
