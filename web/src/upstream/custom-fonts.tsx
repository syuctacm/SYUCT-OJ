// Ported from @hydrooj/ui-default components/customFont.page.tsx (AGPL-3.0).
// The controls are mounted into the upstream home_settings.html form.
import { useRef, type Ref } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import CustomSelectAutoComplete from '../components/autocomplete/CustomSelectAutoComplete.tsx'
import { Dialog } from '../components/Dialog.tsx'
import { alertDialog } from '../components/dialogs.tsx'

type FontItem = { id: string; name: string; fullName: string }
type LocalFontData = { family: string; fullName: string }
type FontSaveKey = 'customFont' | 'customCodeFont'
type Cleanup = () => void

declare global {
  interface Window {
    queryLocalFonts?: () => Promise<LocalFontData[]>
  }
}

let fontCache: FontItem[] | null = null

async function getAvailableFonts(): Promise<FontItem[]> {
  if (fontCache) return fontCache
  const fonts = await window.queryLocalFonts?.() ?? []
  const families = new Map<string, string>()
  for (const font of fonts) {
    if (!families.has(font.family)) families.set(font.family, font.fullName)
  }
  const items = [...families].map(([family, fullName]) => ({
    id: family,
    name: family === fullName ? family : `${fullName} (${family})`,
    fullName,
  }))
  if (items.length) fontCache = items
  return items
}

type FontDialogProps = {
  items: FontItem[]
  onResolve: (value: string | null) => void
}

function FontDialog({ items, onResolve }: FontDialogProps) {
  const handle = useRef<any>(null)
  const selected = () => handle.current?.getSelectedItems()?.[0]?.id as string | undefined
  return (
    <Dialog
      okLabel="确定"
      cancelLabel="取消"
      width="min(620px, calc(100vw - 40px))"
      validate={() => Boolean(selected())}
      onOk={() => onResolve(selected() ?? null)}
      onCancel={() => onResolve(null)}
    >
      <div className="typo">
        <p>选择本机字体</p>
        <CustomSelectAutoComplete
          ref={handle as Ref<any>}
          allowEmptyQuery={false}
          data={items}
          renderItem={(item: FontItem) => <div style={{ fontFamily: item.id }}>{item.name}</div>}
        />
      </div>
    </Dialog>
  )
}

async function selectFont(saveKey: FontSaveKey): Promise<void> {
  let items: FontItem[]
  try {
    items = await getAvailableFonts()
  } catch (error) {
    await alertDialog(error instanceof Error ? error.message : '无法读取本机字体')
    return
  }
  if (!items.length) {
    await alertDialog('没有找到可用的本机字体')
    return
  }

  return new Promise((resolve) => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    root.render(
      <FontDialog
        items={items}
        onResolve={(family) => {
          if (family) globalThis.localStorage.setItem(saveKey, family)
          else globalThis.localStorage.removeItem(saveKey)
          root.unmount()
          container.remove()
          if (family) window.location.reload()
          else resolve()
        }}
      />,
    )
  })
}

function localFont(saveKey: FontSaveKey) {
  return globalThis.localStorage.getItem(saveKey) ?? ''
}

function mountButton(row: Element, saveKey: FontSaveKey, label: string): { host: HTMLDivElement; root: Root; cleanup: Cleanup } {
  const host = document.createElement('div')
  host.className = 'medium-6 columns'
  row.append(host)
  const root = createRoot(host)
  const value = localFont(saveKey)
  root.render(
    <label>
      {label}
      <div>
        <button className="button inline" type="button" data-show-font-dialog={saveKey}>
          {value ? `Using custom font: ${value}` : 'Find custom font'}
        </button>
      </div>
    </label>,
  )
  return { host, root, cleanup: () => queueMicrotask(() => root.unmount()) }
}

export function installCustomFontControls(root: HTMLElement): Cleanup {
  if (!('queryLocalFonts' in window)) return () => {}
  const mounted: Array<{ host: HTMLDivElement; root: Root; cleanup: Cleanup }> = []
  const cleanups: Cleanup[] = []

  for (const [key, label] of [
    ['customFont', 'Select Custom Font'],
    ['customCodeFont', 'Select Custom Code Font'],
  ] as const) {
    const input = root.querySelector<HTMLSelectElement>(`[name="form_item_${key === 'customFont' ? 'fontFamily' : 'codeFontFamily'}"]`)
    const row = input?.parentElement?.parentElement?.parentElement
    if (!row) continue
    mounted.push(mountButton(row, key, label))
  }

  const onClick = (event: MouseEvent) => {
    const button = (event.target as Element).closest<HTMLElement>('[data-show-font-dialog]')
    if (!button) return
    const saveKey = button.dataset.showFontDialog
    if (saveKey !== 'customFont' && saveKey !== 'customCodeFont') return
    event.preventDefault()
    void selectFont(saveKey)
  }
  root.addEventListener('click', onClick)
  cleanups.push(() => root.removeEventListener('click', onClick))
  cleanups.push(...mounted.map((item) => () => {
    item.host.remove()
    item.cleanup()
  }))

  return () => cleanups.forEach((cleanup) => cleanup())
}
