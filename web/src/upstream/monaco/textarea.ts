import { monaco } from './base.ts'

export function mountMonacoTextarea(textarea: HTMLTextAreaElement, language: string) {
  if (!textarea.isConnected) return () => {}

  const height = textarea.getBoundingClientRect().height || 500
  const host = document.createElement('div')
  host.className = 'monaco-textarea-host'
  Object.assign(host.style, {
    width: '100%',
    height: `${height}px`,
    border: '1px solid var(--border-color, rgba(0, 0, 0, .15))',
    boxSizing: 'border-box',
  })
  textarea.hidden = true
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
  const subscription = editor.onDidChangeModelContent(() => {
    textarea.value = editor.getValue()
    textarea.dispatchEvent(new Event('input', { bubbles: true }))
    textarea.dispatchEvent(new Event('change', { bubbles: true }))
  })

  return () => {
    subscription.dispose()
    editor.dispose()
    model.dispose()
    host.remove()
    textarea.hidden = false
  }
}
