import { createRoot } from 'react-dom/client'
import { uploadActivityFiles, uploadUserFiles, type ActivityFile, type ActivityFileResource } from '../api.ts'
import { LegacyMarkdownEditor } from '../components/LegacyMarkdownEditor.tsx'
import { AssignSelectAutoComplete } from '../components/autocomplete/AssignSelectAutoComplete.tsx'
import { LanguageSelectAutoComplete } from '../components/autocomplete/LanguageSelectAutoComplete.tsx'
import { ProblemSelectAutoComplete } from '../components/autocomplete/ProblemSelectAutoComplete.tsx'
import { UserSelectAutoComplete } from '../components/autocomplete/UserSelectAutoComplete.tsx'
import { attachDatePicker, attachTimePicker } from '../legacy-pickers.ts'

type Cleanup = () => void
type AutocompleteKind = 'problems' | 'users' | 'assign' | 'langs'

function addListener<K extends keyof HTMLElementEventMap>(
  element: HTMLElement,
  type: K,
  listener: (event: HTMLElementEventMap[K]) => void,
) {
  element.addEventListener(type, listener)
  return () => element.removeEventListener(type, listener)
}

function mountAutocomplete(input: HTMLInputElement, kind: AutocompleteKind): Cleanup {
  const selectedKeys = input.value.split(',').map((item) => item.trim()).filter(Boolean)
  const host = document.createElement('div')
  const Component = {
    problems: ProblemSelectAutoComplete,
    users: UserSelectAutoComplete,
    assign: AssignSelectAutoComplete,
    langs: LanguageSelectAutoComplete,
  }[kind]
  input.hidden = true
  input.classList.add('autocomplete-dummy')
  input.parentElement?.appendChild(host)
  const root = createRoot(host)
  root.render(
    <Component
      selectedKeys={selectedKeys}
      multi
      placeholder={input.placeholder}
      onChange={(value) => {
        input.value = value
        input.dispatchEvent(new Event('change', { bubbles: true }))
      }}
    />,
  )
  return () => {
    host.remove()
    input.hidden = false
    input.classList.remove('autocomplete-dummy')
    queueMicrotask(() => root.unmount())
  }
}

type MarkdownHydrationOptions = {
  resource?: ActivityFileResource
  tid?: string
  onChange?: () => void
  onUploaded?: (files: ActivityFile[]) => void
  uploadProblem?: (files: File[]) => Promise<Array<{ name?: string; downloadUrl: string }>>
  onProblemUploaded?: (files: Array<{ name?: string; downloadUrl: string }>) => void
  uploadToUserFiles?: boolean
}

type MarkdownUploadHandler = (
  files: File[],
  callback: (urls: string[]) => void,
) => Promise<void>

function uploadedForInsertion<T extends { name?: string; downloadUrl: string }>(files: File[], uploaded: T[]): T[] {
  if (uploaded.some((file) => !file.name)) return uploaded
  return newlyUploaded(files, uploaded as Array<T & { name: string }>)
}

function uploadWithFeedback<T extends { name?: string; downloadUrl: string }>(
  upload: (files: File[]) => Promise<T[]>,
  onUploaded?: (files: T[]) => void,
): MarkdownUploadHandler {
  return async (files, callback) => {
    try {
      const uploaded = await upload(files)
      callback(uploadedForInsertion(files, uploaded).map((file) => file.downloadUrl))
      onUploaded?.(uploaded)
    } catch (error) {
      callback([])
      window.alert(error instanceof Error ? error.message : '图片上传失败')
    }
  }
}

function newlyUploaded<T extends { name: string; downloadUrl: string }>(files: File[], items: T[]): T[] {
  const names = new Set(files
    .map((file) => file.name.replaceAll('\\', '/').split('/').pop())
    .filter(Boolean))
  return items.filter((item) => names.has(item.name))
}

function randomizeImageNames(files: File[]): File[] {
  return files.map((file) => {
    const dot = file.name.lastIndexOf('.')
    const extension = dot >= 0 ? file.name.slice(dot) : ''
    const id = typeof crypto.randomUUID === 'function'
      ? crypto.randomUUID()
      : `${Date.now()}-${Math.random().toString(36).slice(2)}`
    return new File([file], `${id}${extension}`, { type: file.type, lastModified: file.lastModified })
  })
}

function mountMarkdownEditor(textarea: HTMLTextAreaElement, options: MarkdownHydrationOptions): Cleanup {
  const host = document.createElement('div')
  textarea.hidden = true
  textarea.parentElement?.appendChild(host)
  const root = createRoot(host)
  let onUploadImg: MarkdownUploadHandler | undefined
  if (options.uploadProblem) {
    onUploadImg = uploadWithFeedback((files) => options.uploadProblem!(files), options.onProblemUploaded)
  } else if (options.uploadToUserFiles) {
    onUploadImg = uploadWithFeedback(async (files) => uploadUserFiles(files))
  } else if (options.resource && options.tid) {
    const { resource, tid } = options
    onUploadImg = uploadWithFeedback(async (files) => uploadActivityFiles(resource, tid, files), options.onUploaded)
  }
  const uploadHandler = onUploadImg ? async (files: File[], callback: (urls: string[]) => void) => {
    await onUploadImg(randomizeImageNames(files), callback)
  } : undefined
  root.render(
    <LegacyMarkdownEditor
      value={textarea.value}
      placeholder={textarea.placeholder}
      onChange={(value) => {
        textarea.value = value
        textarea.dispatchEvent(new Event('change', { bubbles: true }))
        options.onChange?.()
      }}
      hideImageUpload={!uploadHandler}
      onUploadImg={uploadHandler}
    />,
  )
  return () => {
    host.remove()
    textarea.hidden = false
    queueMicrotask(() => root.unmount())
  }
}


function attachHotkeys(root: HTMLElement): Cleanup {
  if (!root.querySelector('[data-hotkey*="ctrl+enter"]')) return () => {}
  const onKeyDown = (event: KeyboardEvent) => {
    if (event.defaultPrevented || !(event.ctrlKey || event.metaKey) || event.key !== 'Enter') return
    const target = event.target as Element | null
    const form = target?.closest('form')
    if (!form) return
    event.preventDefault()
    form.requestSubmit()
  }
  root.addEventListener('keydown', onKeyDown)
  return () => root.removeEventListener('keydown', onKeyDown)
}

function attachContestSwitches(root: HTMLElement): Cleanup {
  const cleanups: Cleanup[] = []
  const rule = root.querySelector<HTMLSelectElement>('[name="rule"]')
  if (rule) {
    const update = () => {
      root.querySelectorAll<HTMLInputElement>('.contest-rule-settings input').forEach((input) => {
        input.disabled = true
      })
      root.querySelectorAll<HTMLElement>('.contest-rule-settings').forEach((item) => {
        item.style.display = 'none'
      })
      root.querySelectorAll<HTMLInputElement>(`.contest-rule--${rule.value} input`).forEach((input) => {
        input.disabled = false
      })
      root.querySelectorAll<HTMLElement>(`.contest-rule--${rule.value}`).forEach((item) => {
        item.style.display = ''
      })
    }
    cleanups.push(addListener(rule, 'change', update))
    update()
  }

  const permission = root.querySelector<HTMLSelectElement>('[name="permission"]')
  if (permission) {
    permission.disabled = false
    const update = () => {
      root.querySelectorAll<HTMLInputElement>('[data-perm] input').forEach((input) => {
        input.disabled = true
      })
      root.querySelectorAll<HTMLElement>('[data-perm]').forEach((item) => {
        item.style.display = 'none'
      })
      root.querySelectorAll<HTMLInputElement>(`[data-perm="${permission.value}"] input`).forEach((input) => {
        input.disabled = false
      })
      root.querySelectorAll<HTMLElement>(`[data-perm="${permission.value}"]`).forEach((item) => {
        item.style.display = ''
      })
    }
    cleanups.push(addListener(permission, 'change', update))
    update()
  }
  return () => cleanups.forEach((cleanup) => cleanup())
}

function attachDateTime(root: HTMLElement): Cleanup {
  const cleanups: Cleanup[] = []
  root.querySelectorAll<HTMLInputElement>('[data-pick-date]').forEach((input) => cleanups.push(attachDatePicker(input)))
  root.querySelectorAll<HTMLInputElement>('[data-pick-time]').forEach((input) => cleanups.push(attachTimePicker(input)))

  const beginDate = root.querySelector<HTMLInputElement>('[name="beginAtDate"]')
  const beginTime = root.querySelector<HTMLInputElement>('[name="beginAtTime"]')
  const duration = root.querySelector<HTMLInputElement>('[name="duration"]')
  const endAt = root.querySelector<HTMLInputElement>('[name="endAt"]')
  if (beginDate && beginTime && duration && endAt) {
    const update = () => {
      const begin = new Date(`${beginDate.value}T${beginTime.value || '00:00'}:00`)
      const hours = Number(duration.value)
      if (Number.isNaN(begin.getTime()) || !Number.isFinite(hours)) return
      const end = new Date(begin.getTime() + hours * 3600000)
      const pad = (value: number) => String(value).padStart(2, '0')
      endAt.value = `${end.getFullYear()}-${pad(end.getMonth() + 1)}-${pad(end.getDate())} ${pad(end.getHours())}:${pad(end.getMinutes())}`
    }
    for (const input of [beginDate, beginTime, duration]) cleanups.push(addListener(input, 'change', update))
    update()
  }
  return () => cleanups.forEach((cleanup) => cleanup())
}

type HydrateOptions = MarkdownHydrationOptions & {
  autocomplete?: Partial<Record<AutocompleteKind, string>>
  contestSwitches?: boolean
  dateTime?: boolean
}

export function hydrateHydroEditForm(root: HTMLElement, options: HydrateOptions = {}) {
  const cleanups: Cleanup[] = [attachHotkeys(root)]
  if (options.contestSwitches) cleanups.push(attachContestSwitches(root))
  if (options.dateTime) cleanups.push(attachDateTime(root))

  for (const [kind, name] of Object.entries(options.autocomplete ?? {}) as Array<[AutocompleteKind, string]>) {
    const input = root.querySelector<HTMLInputElement>(`input[name="${name}"]`)
    if (input) cleanups.push(mountAutocomplete(input, kind))
  }

  root.querySelectorAll<HTMLTextAreaElement>('textarea[data-editor]').forEach((textarea) => {
    textarea.hidden = true
    if (textarea.parentElement) textarea.parentElement.style.display = 'none'
  })

  for (const textarea of root.querySelectorAll<HTMLTextAreaElement>('textarea[data-markdown], textarea[data-markdown-upload]')) {
    cleanups.push(mountMarkdownEditor(textarea, options))
  }
  return () => cleanups.forEach((cleanup) => cleanup())
}
