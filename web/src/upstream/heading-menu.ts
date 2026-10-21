// Ported from Hydro ui-default components/menu/menu-heading.page.js: headings
// marked with `data-heading` inside a `data-heading-extract-to` container are
// copied into the sidebar menu item it points at.
export function mountHeadingMenu(root: HTMLElement) {
  const cleanups: Array<() => void> = []
  root.querySelectorAll<HTMLElement>('[data-heading-extract-to]').forEach((container) => {
    const selector = container.getAttribute('data-heading-extract-to')
    if (!selector) return
    const target = document.querySelector(selector)
    if (!target) return
    let menu = target.querySelector<HTMLElement>(':scope > .menu')
    if (!menu) {
      menu = document.createElement('ul')
      menu.className = 'menu'
      target.appendChild(menu)
      target.querySelectorAll(':scope > .menu__link').forEach((link) => link.classList.add('expandable'))
    }
    container.querySelectorAll<HTMLElement>('[data-heading]').forEach((heading) => {
      if (menu!.querySelector(`a[href="#${heading.id}"]`)) return
      const item = document.createElement('li')
      item.className = 'menu__item'
      const link = document.createElement('a')
      link.className = 'menu__link'
      link.href = `#${heading.id}`
      link.textContent = heading.textContent
      item.appendChild(link)
      menu!.appendChild(item)
    })
  })
  return () => cleanups.forEach((cleanup) => cleanup())
}
