const DB_NAME = 'helios-offline'
const DB_VERSION = 1
const STORES = ['queue', 'cache', 'downloads']

let dbPromise = null

function open() {
  if (dbPromise) return dbPromise
  dbPromise = new Promise((resolve, reject) => {
    const req = indexedDB.open(DB_NAME, DB_VERSION)
    req.onupgradeneeded = () => {
      const db = req.result
      for (const name of STORES) {
        if (!db.objectStoreNames.contains(name)) {
          db.createObjectStore(name, { keyPath: name === 'queue' ? 'id' : 'key' })
        }
      }
    }
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error)
  })
  return dbPromise
}

function tx(storeName, mode = 'readonly') {
  return open().then(db => db.transaction(storeName, mode).objectStore(storeName))
}

function req(request) {
  return new Promise((resolve, reject) => {
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => reject(request.error)
  })
}

export async function put(storeName, value) {
  const store = await tx(storeName, 'readwrite')
  return req(store.put(value))
}

export async function get(storeName, key) {
  const store = await tx(storeName)
  return req(store.get(key))
}

export async function del(storeName, key) {
  const store = await tx(storeName, 'readwrite')
  return req(store.delete(key))
}

export async function all(storeName) {
  const store = await tx(storeName)
  return req(store.getAll())
}

export async function clear(storeName) {
  const store = await tx(storeName, 'readwrite')
  return req(store.clear())
}
