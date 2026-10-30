// IndexedDB draft store compatible with the upstream scratchpad database.
export type ScratchpadDraft = {
  id: string
  code: string
  lang: string
}

const DB_NAME = 'hydro'
const DB_VERSION = 1
const STORE = 'scratchpad-drafts'
let opening: Promise<IDBDatabase> | null = null

function openDatabase() {
  if (opening) return opening
  opening = new Promise<IDBDatabase>((resolve, reject) => {
    const request = indexedDB.open(DB_NAME, DB_VERSION)
    request.onupgradeneeded = () => {
      const db = request.result
      if (!db.objectStoreNames.contains(STORE)) db.createObjectStore(STORE, { keyPath: 'id' })
    }
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => reject(request.error ?? new Error('unable to open scratchpad database'))
  })
  return opening
}

export async function getScratchpadDraft(id: string): Promise<ScratchpadDraft | null> {
  try {
    const db = await openDatabase()
    return await new Promise((resolve, reject) => {
      const request = db.transaction(STORE, 'readonly').objectStore(STORE).get(id)
      request.onsuccess = () => resolve((request.result as ScratchpadDraft | undefined) ?? null)
      request.onerror = () => reject(request.error)
    })
  } catch {
    return null
  }
}

export async function putScratchpadDraft(draft: ScratchpadDraft): Promise<void> {
  try {
    const db = await openDatabase()
    await new Promise<void>((resolve, reject) => {
      const transaction = db.transaction(STORE, 'readwrite')
      transaction.objectStore(STORE).put(draft)
      transaction.oncomplete = () => resolve()
      transaction.onerror = () => reject(transaction.error)
      transaction.onabort = () => reject(transaction.error)
    })
  } catch {
    // Draft persistence is best-effort and must never block submission.
  }
}
