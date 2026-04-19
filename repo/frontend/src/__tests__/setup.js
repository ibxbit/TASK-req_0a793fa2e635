import 'fake-indexeddb/auto'
import { IDBFactory } from 'fake-indexeddb'
import { beforeEach, vi } from 'vitest'

// Before every test: reset module registry and replace the global indexedDB
// with a fresh IDBFactory instance. This is more reliable than deleteDatabase()
// which can hang in "blocked" state when another worker has open connections.
beforeEach(() => {
  vi.resetModules()
  globalThis.indexedDB = new IDBFactory()
})
