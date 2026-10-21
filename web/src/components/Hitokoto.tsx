// Ported from Hydro ui-default components/hitokoto/index.page.js: the homepage
// section is an empty container that gets one quote appended.
export function mountHitokoto(root: HTMLElement) {
  const container = root.querySelector<HTMLElement>('[name="hitokoto"]')
  if (!container) return () => {}
  let cancelled = false
  fetch('https://v1.hitokoto.cn?c=a&c=b&c=c&c=d&c=e&c=f')
    .then((response) => response.json())
    .then((data: { hitokoto?: string }) => {
      if (cancelled) return
      const paragraph = document.createElement('p')
      paragraph.textContent = data.hitokoto ?? ''
      container.replaceChildren(paragraph)
    })
    .catch(() => {
      if (cancelled) return
      const paragraph = document.createElement('p')
      paragraph.textContent = '无法获取一言。'
      container.replaceChildren(paragraph)
    })
  return () => {
    cancelled = true
  }
}
