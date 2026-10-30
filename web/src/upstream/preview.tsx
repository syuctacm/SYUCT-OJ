// Adapted from @hydrooj/ui-default components/preview/preview.page.ts (AGPL-3.0).
// The upstream click target and file-type rules are preserved; file bytes are
// fetched from the Go API and rendered through the shared Hydro Dialog.
import { createRoot } from 'react-dom/client'

import { Dialog } from '../components/Dialog.tsx'
import { notifyError, notifyInfo } from '../components/Notification.tsx'

const MAX_PREVIEW_SIZE = 8 * 1024 * 1024
const MAX_TEXT_SIZE = 1024 * 1024
const IMAGE_EXTENSIONS = new Set(['png', 'jpeg', 'jpg', 'gif', 'webp', 'bmp', 'svg'])
const VIDEO_EXTENSIONS = new Set(['mp4', 'webm', 'ogg'])
const DOWNLOAD_EXTENSIONS = new Set(['zip', 'rar', '7z', 'doc', 'docx', 'xls', 'xlsx', 'ppt', 'pptx'])

type PreviewProps = {
  filename: string
  href: string
  kind: 'text' | 'image' | 'video' | 'pdf'
  text?: string
  src?: string
  onClose: () => void
}

function PreviewDialog({ filename, href, kind, text, src, onClose }: PreviewProps) {
  return (
    <Dialog
      okLabel="关闭"
      cancelLabel=""
      width="min(1100px, calc(100vw - 80px))"
      onOk={onClose}
      onCancel={onClose}
    >
      <div className="typo" style={{ minHeight: '240px' }}>
        <div className="clearfix" style={{ marginBottom: '10px' }}>
          <strong>{filename}</strong>
          <a className="button rounded float-right" href={href}>下载</a>
        </div>
        {kind === 'text' ? (
          <pre style={{ maxHeight: '70vh', overflow: 'auto', whiteSpace: 'pre-wrap' }}>{text}</pre>
        ) : null}
        {kind === 'image' && src ? (
          <img src={src} alt={filename} style={{ display: 'block', maxWidth: '100%', maxHeight: '70vh', margin: '0 auto' }} />
        ) : null}
        {kind === 'video' && src ? (
          <video src={src} controls style={{ display: 'block', width: '100%', maxHeight: '70vh' }} />
        ) : null}
        {kind === 'pdf' && src ? (
          <iframe src={src} title={filename} style={{ width: '100%', height: '70vh', border: 0 }} />
        ) : null}
      </div>
    </Dialog>
  )
}

function download(href: string) {
  const anchor = document.createElement('a')
  anchor.href = href
  anchor.download = ''
  document.body.append(anchor)
  anchor.click()
  anchor.remove()
}

async function openPreview(filename: string, href: string, size: number) {
  const extension = filename.split('.').pop()?.toLowerCase() ?? ''
  if (DOWNLOAD_EXTENSIONS.has(extension) || size > MAX_PREVIEW_SIZE) {
    download(href)
    return
  }

  let kind: PreviewProps['kind'] = 'text'
  if (IMAGE_EXTENSIONS.has(extension)) kind = 'image'
  else if (VIDEO_EXTENSIONS.has(extension)) kind = 'video'
  else if (extension === 'pdf') kind = 'pdf'

  let src: string | undefined
  let text: string | undefined
  try {
    notifyInfo('正在加载文件...')
    const response = await fetch(href, { credentials: 'same-origin', headers: { Pragma: 'no-cache' } })
    if (!response.ok) throw new Error(`${response.status} ${response.statusText}`)
    const blob = await response.blob()
    if (kind === 'text' && blob.size > MAX_TEXT_SIZE) {
      download(href)
      return
    }
    if (kind === 'text') text = await blob.text()
    else src = URL.createObjectURL(blob)
  } catch (error) {
    notifyError(error instanceof Error ? error.message : '文件加载失败')
    return
  }

  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  await new Promise<void>((resolve) => {
    const close = () => {
      queueMicrotask(() => {
        root.unmount()
        container.remove()
        if (src) URL.revokeObjectURL(src)
        resolve()
      })
    }
    root.render(
      <PreviewDialog
        filename={filename}
        href={href}
        kind={kind}
        text={text}
        src={src}
        onClose={close}
      />,
    )
  })
}

let installed = false

export function installHydroFilePreview() {
  if (installed) return
  installed = true
  document.addEventListener('click', (event) => {
    if (event.defaultPrevented || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return
    const target = (event.target as Element | null)?.closest<HTMLElement>('[data-preview]')
    if (!target) return
    const anchor = target.querySelector<HTMLAnchorElement>('a')
    if (!anchor?.href) return
    event.preventDefault()
    const row = target.closest<HTMLElement>('[data-filename]')
    const filename = row?.dataset.filename || anchor.textContent?.trim() || 'file'
    const size = Number(row?.dataset.size ?? 0)
    void openPreview(filename, anchor.href, Number.isFinite(size) ? size : 0)
  })
}
