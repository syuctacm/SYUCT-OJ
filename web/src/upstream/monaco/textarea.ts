import { monaco } from './base.ts'

export type MonacoTextareaController = {
  dispose: () => void
  getValue: () => string
  setValue: (value: string) => void
  setLanguage: (language: string) => void
  focus: () => void
}

function noopController(): MonacoTextareaController {
  return {
    dispose: () => {},
    getValue: () => '',
    setValue: () => {},
    setLanguage: () => {},
    focus: () => {},
  }
}

export function mountMonacoTextarea(
  textarea: HTMLTextAreaElement,
  language: string,
  options: { submitOnCtrlEnter?: boolean } = {},
): MonacoTextareaController {
  if (!textarea.isConnected) return noopController()

  const height = textarea.getBoundingClientRect().height || 500
  const host = document.createElement('div')
  host.className = 'monaco-textarea-host'
  Object.assign(host.style, {
    width: '100%',
    height: `${height}px`,
    border: '1px solid var(--border-color, rgba(0, 0, 0, .15))',
    boxSizing: 'border-box',
  })
  const previousDisplay = textarea.style.display
  textarea.style.display = 'none'
  textarea.after(host)

  const model = monaco.editor.createModel(
    textarea.value,
    language,
    monaco.Uri.parse(`hydro://textarea/${Date.now()}-${Math.random().toString(16).slice(2)}.${language}`),
  )
  const editor = monaco.editor.create(host, {
    model,
    theme: document.documentElement.dataset.mantineColorScheme === 'dark' ? 'vs-dark' : 'vs-light',
    automaticLayout: true,
    minimap: { enabled: false },
    scrollBeyondLastLine: false,
    tabSize: 2,
    fontFamily: globalThis.UserContext?.codeFontFamily || 'var(--code-font-family, "Source Code Pro", Consolas, monospace)',
  })
  const onKeyDown = (event: KeyboardEvent) => {
    if (!(event.ctrlKey || event.metaKey) || event.key !== 'Enter') return
    event.preventDefault()
    event.stopPropagation()
    textarea.form?.requestSubmit()
  }
  if (options.submitOnCtrlEnter !== false) host.addEventListener('keydown', onKeyDown, true)
  const syncTextarea = () => {
    textarea.value = editor.getValue()
    textarea.dispatchEvent(new Event('input', { bubbles: true }))
    textarea.dispatchEvent(new Event('change', { bubbles: true }))
  }
  const subscription = editor.onDidChangeModelContent(syncTextarea)

  return {
    dispose() {
      host.removeEventListener('keydown', onKeyDown, true)
      subscription.dispose()
      editor.dispose()
      model.dispose()
      host.remove()
      textarea.style.display = previousDisplay
    },
    getValue: () => editor.getValue(),
    setValue(value: string) {
      if (editor.getValue() === value) return
      editor.setValue(value)
      syncTextarea()
    },
    setLanguage(nextLanguage: string) {
      if (model.getLanguageId() !== nextLanguage) monaco.editor.setModelLanguage(model, nextLanguage)
    },
    focus: () => editor.focus(),
  }
}
