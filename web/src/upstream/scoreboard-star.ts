// Ported from ui-default utils/db.ts + pages/contest_scoreboard.page.ts: the
// scoreboard "starred users" filter is stored in IndexedDB per contest.
const DB_NAME = 'hydro'
const DB_VERSION = 3
const STORE = 'scoreboard-star'

function openDatabase(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open(DB_NAME, DB_VERSION)
    request.onupgradeneeded = () => {
      const db = request.result
      for (const name of ['solutions', STORE, 'domain-info', 'scratchpad-drafts']) {
        if (!db.objectStoreNames.contains(name)) db.createObjectStore(name, { keyPath: 'id' })
      }
    }
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => reject(request.error)
  })
}

function transaction<T>(mode: IDBTransactionMode, run: (store: IDBObjectStore) => IDBRequest<T>): Promise<T> {
  return openDatabase().then((db) => new Promise<T>((resolve, reject) => {
    const tx = db.transaction(STORE, mode)
    const request = run(tx.objectStore(STORE))
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => reject(request.error)
    tx.oncomplete = () => db.close()
  }))
}

export async function readScoreboardStars(id: string): Promise<number[]> {
  try {
    const record = await transaction<{ id: string; data?: number[] }>('readonly', (store) => store.get(id))
    return record?.data ?? []
  } catch {
    return []
  }
}

export async function writeScoreboardStars(id: string, data: number[]): Promise<void> {
  try {
    await transaction('readwrite', (store) => store.put({ id, data }))
  } catch {
    // starring is a local convenience; ignore storage failures
  }
}
