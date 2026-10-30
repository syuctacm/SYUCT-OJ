// @ts-nocheck -- Prism component modules are CommonJS/global scripts.
// Ported from @hydrooj/ui-default components/highlighter (AGPL-3.0).
import Prism from 'prismjs'
import 'prismjs/plugins/line-numbers/prism-line-numbers'
import 'prismjs/plugins/line-highlight/prism-line-highlight'
import { notifyError, notifySuccess } from '../components/Notification.tsx'
import { fillScratchpadPretest } from './scratchpad-pretest.ts'

const loaders = {
  c: () => import('prismjs/components/prism-c'),
  cpp: () => import('prismjs/components/prism-cpp'),
  java: () => import('prismjs/components/prism-java'),
  python: () => import('prismjs/components/prism-python'),
  go: () => import('prismjs/components/prism-go'),
  rust: () => import('prismjs/components/prism-rust'),
  javascript: () => import('prismjs/components/prism-javascript'),
  typescript: () => import('prismjs/components/prism-typescript'),
  bash: () => import('prismjs/components/prism-bash'),
  shell: () => import('prismjs/components/prism-shell-session'),
  json: () => import('prismjs/components/prism-json'),
  yaml: () => import('prismjs/components/prism-yaml'),
  sql: () => import('prismjs/components/prism-sql'),
  'markup-templating': () => import('prismjs/components/prism-markup-templating'),
  php: () => import('prismjs/components/prism-php'),
  ruby: () => import('prismjs/components/prism-ruby'),
  csharp: () => import('prismjs/components/prism-csharp'),
  kotlin: () => import('prismjs/components/prism-kotlin'),
  swift: () => import('prismjs/components/prism-swift'),
  scala: () => import('prismjs/components/prism-scala'),
  lua: () => import('prismjs/components/prism-lua'),
  perl: () => import('prismjs/components/prism-perl'),
  r: () => import('prismjs/components/prism-r'),
  pascal: () => import('prismjs/components/prism-pascal'),
  haskell: () => import('prismjs/components/prism-haskell'),
}

const dependencies = {
  cpp: ['c'],
  java: ['clike'],
  javascript: ['clike'],
  typescript: ['javascript'],
  php: ['markup-templating', 'clike'],
  csharp: ['clike'],
  kotlin: ['clike'],
  swift: ['clike'],
  scala: ['java'],
  shell: ['bash'],
}

const aliases = {
  'c++': 'cpp',
  py: 'python',
  js: 'javascript',
  ts: 'typescript',
  sh: 'bash',
  'c#': 'csharp',
  golang: 'go',
  node: 'javascript',
  nodejs: 'javascript',
  py3: 'python',
  python3: 'python',
  html: 'markup',
  xml: 'markup',
  shellscript: 'bash',
  console: 'shell',
  rs: 'rust',
  rb: 'ruby',
  yml: 'yaml',
  kt: 'kotlin',
}

const loaded = new Set()

async function loadLanguage(language) {
  if (!language || Prism.languages[language] || loaded.has(language)) return
  loaded.add(language)
  for (const dependency of dependencies[language] ?? []) await loadLanguage(dependency)
  const loader = loaders[language]
  if (!loader) return
  try {
    await loader()
  } catch (error) {
    console.warn('unable to load Prism language', language, error)
  }
}

function languageName(code) {
  const match = (code.className || '').match(/language-([a-z0-9+#-]+)/i)
  if (!match) return ''
  const raw = match[1].toLowerCase()
  return aliases[raw] ?? raw
}

function copyText(value) {
  if (navigator.clipboard?.writeText) return navigator.clipboard.writeText(value)
  const textarea = document.createElement('textarea')
  textarea.value = value
  textarea.style.position = 'fixed'
  textarea.style.opacity = '0'
  document.body.appendChild(textarea)
  textarea.select()
  document.execCommand('copy')
  textarea.remove()
  return Promise.resolve()
}

function addToolbar(pre, code) {
  if (pre.querySelector(':scope > .toolbar')) return
  const toolbar = document.createElement('div')
  toolbar.className = 'toolbar'
  const copy = document.createElement('div')
  copy.className = 'toolbar-item'
  const copyButton = document.createElement('a')
  copyButton.href = 'javascript:;'
  copyButton.textContent = '复制'
  copyButton.addEventListener('click', () => {
    void copyText(code.textContent ?? '')
      .then(() => notifySuccess('内容已复制到剪贴板', 1000))
      .catch(() => notifyError('复制失败'))
  })
  copy.appendChild(copyButton)
  toolbar.appendChild(copy)
  pre.appendChild(toolbar)
}

function addSample(pre, code, container) {
  const match = (code.className || '').match(/language-input([0-9]+)/i)
  if (!match) return
  const id = Number(match[1])
  if (!Number.isSafeInteger(id)) return
  const typo = pre.closest('.typo')
  const scope = typo ?? container
  const output = scope.querySelector(`pre.language-output${id}`)
  if (!output) return
  if (!pre.classList.contains('sample')) {
    const row = document.createElement('div')
    row.className = 'row'
    pre.after(row)
    const inputTitle = document.createElement('h2')
    inputTitle.textContent = `样例输入 ${id}`
    pre.prepend(inputTitle)
    pre.classList.add('medium-6', 'columns', 'sample')
    row.appendChild(pre)
    const outputTitle = document.createElement('h2')
    outputTitle.textContent = `样例输出 ${id}`
    output.prepend(outputTitle)
    output.classList.add('medium-6', 'columns', 'sample')
    row.appendChild(output)
  }
  const toolbar = pre.querySelector(':scope > .toolbar')
  if (!toolbar || toolbar.querySelector('.scratchpad-fill-button')) return
  const item = document.createElement('div')
  item.className = 'toolbar-item scratchpad-fill-button'
  const button = document.createElement('a')
  button.href = 'javascript:;'
  button.textContent = '填入预测试'
  button.addEventListener('click', () => {
    fillScratchpadPretest(code.textContent ?? '')
    notifySuccess('输入已填入预测试', 2000)
  })
  item.appendChild(button)
  toolbar.appendChild(item)
}

export async function initHydroHighlighter(root: ParentNode) {
  const codes = [
    ...(root instanceof Element && root.matches('pre > code') ? [root] : []),
    ...root.querySelectorAll<HTMLElement>('pre > code'),
  ]
  for (const code of codes) {
    const pre = code.closest('pre')
    if (!pre) continue

    const rawClass = (code.className || '').trim()
    if (rawClass.includes('line-numbers')) pre.classList.add('line-numbers')
    const lineRange = rawClass.match(/language-([a-z0-9+#-]+)\|([\d,-]+)/i)
    if (lineRange) {
      pre.dataset.line = lineRange[2]
      code.className = `language-${lineRange[1]}`
    }

    pre.classList.add('syntax-hl')
    addToolbar(pre, code)
    addSample(pre, code, root)
    if (code.dataset.hydroHighlighted) continue
    const language = languageName(code)
    if (language && !language.startsWith('input') && !language.startsWith('output')) {
      await loadLanguage(language)
    }
    try {
      Prism.highlightElement(code)
    } catch (error) {
      console.warn('unable to highlight code block', error)
    }
    code.dataset.hydroHighlighted = '1'
  }
}
