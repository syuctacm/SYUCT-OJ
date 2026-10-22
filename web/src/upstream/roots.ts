// React 19 StrictMode double-invokes effects in development: a mount/cleanup
// pair runs twice against the same DOM. Creating a new root on the second pass
// logs "createRoot() on a container that has already been passed to
// createRoot()", and unmounting inside the cleanup logs a synchronous-unmount
// warning. Reuse one root per container and clear it with `render(null)`.
import type { ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'

const roots = new WeakMap<Element, Root>()

export function renderInto(container: Element, node: ReactNode): Root {
  let root = roots.get(container)
  if (!root) {
    root = createRoot(container)
    roots.set(container, root)
  }
  root.render(node)
  return root
}

// Clears the tree but keeps the root registered, so a StrictMode re-mount can
// render into the same container.
export function clearRendered(container: Element) {
  roots.get(container)?.render(null)
}
