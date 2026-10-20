import { useLayoutEffect, useMemo, useRef, type MouseEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { renderHydroTemplate, type HydroRenderOptions } from '../upstream/render.ts'

type Props = HydroRenderOptions & {
  template: string
  className?: string
  onReady?: (root: HTMLDivElement) => void | (() => void)
}

export function HydroTemplate({ template, className, context, url, urlForFile, onReady }: Props) {
  const navigate = useNavigate()
  const root = useRef<HTMLDivElement>(null)
  const html = useMemo(
    () => renderHydroTemplate(template, { context, url, urlForFile }),
    [template, context, url, urlForFile],
  )

  useLayoutEffect(() => {
    if (!root.current || !onReady) return
    return onReady(root.current)
  }, [html, onReady])

  function handleClick(event: MouseEvent<HTMLDivElement>) {
    if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return
    const anchor = (event.target as Element).closest('a')
    if (!anchor) return
    const href = anchor.getAttribute('href')
    if (!href || !href.startsWith('/') || href.startsWith('/api/')) return
    if (anchor.target && anchor.target !== '_self') return
    event.preventDefault()
    navigate(href)
  }

  return (
    <div
      ref={root}
      className={className}
      onClick={handleClick}
      dangerouslySetInnerHTML={{ __html: html }}
    />
  )
}
