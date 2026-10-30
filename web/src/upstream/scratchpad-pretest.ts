type Listener = (value: string) => void

const listeners = new Set<Listener>()
let pending = ''

export function fillScratchpadPretest(value: string) {
  pending = value
  for (const listener of listeners) listener(value)
}

export function subscribeScratchpadPretest(listener: Listener) {
  listeners.add(listener)
  if (pending) {
    const value = pending
    pending = ''
    listener(value)
  }
  return () => {
    listeners.delete(listener)
  }
}
