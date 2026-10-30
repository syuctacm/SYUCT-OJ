import { useMemo, useState } from 'react'
import { createRoot } from 'react-dom/client'
import { Dialog } from './Dialog.tsx'

function openDialog<T>(
  render: (resolve: (value: T) => void) => React.ReactNode,
): Promise<T> {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  return new Promise<T>((resolve) => {
    const finish = (value: T) => {
      root.unmount()
      container.remove()
      resolve(value)
    }
    root.render(render(finish))
  })
}

function PromptContent({ message, initialValue }: { message: string; initialValue: string }) {
  const [value, setValue] = useState(initialValue)
  return (
    <div className="typo">
      <p>{message}</p>
      <div className="textbox-container">
        <input
          className="textbox"
          value={value}
          onChange={(event) => setValue(event.currentTarget.value)}
          data-autofocus
        />
        <input type="hidden" value={value} name="dialog-value" readOnly />
      </div>
    </div>
  )
}

export function confirmDialog(message: string): Promise<boolean> {
  return openDialog<boolean>((resolve) => (
    <Dialog
      onOk={() => resolve(true)}
      onCancel={() => resolve(false)}
      okLabel="确定"
      cancelLabel="取消"
    >
      <div className="typo"><p>{message}</p></div>
    </Dialog>
  ))
}

export function promptDialog(message: string, initialValue = ''): Promise<string | null> {
  return openDialog<string | null>((resolve) => (
    <Dialog
      onOk={() => {
        const value = (document.querySelector<HTMLInputElement>('[name="dialog-value"]')?.value ?? '').trim()
        resolve(value || null)
      }}
      onCancel={() => resolve(null)}
    >
      <PromptContent message={message} initialValue={initialValue} />
    </Dialog>
  ))
}

type CreateFileResult = { name: string; content: string }

function CreateFileContent({ onResolve }: { onResolve: (value: CreateFileResult | null) => void }) {
  const [name, setName] = useState('file.txt')
  const [content, setContent] = useState('')
  return (
    <Dialog
      okLabel="创建"
      cancelLabel="取消"
      width="min(900px, calc(100vw - 80px))"
      onOk={() => {
        onResolve({ name: name.trim() || 'file.txt', content })
      }}
      onCancel={() => onResolve(null)}
    >
      <h1>创建文件</h1>
      <label>
        文件名
        <input
          className="textbox"
          value={name}
          onChange={(event) => setName(event.currentTarget.value)}
          data-autofocus
        />
      </label>
      <label>
        内容
        <textarea
          className="textbox"
          value={content}
          onChange={(event) => setContent(event.currentTarget.value)}
          spellCheck={false}
          style={{ width: '100%', minHeight: '45vh', fontFamily: 'var(--code-font-family)', whiteSpace: 'pre' }}
        />
      </label>
    </Dialog>
  )
}

export function createFileDialog(): Promise<CreateFileResult | null> {
  return openDialog<CreateFileResult | null>((resolve) => (
    <CreateFileContent onResolve={resolve} />
  ))
}

function buildRenamedNames(names: string[], original: string, replace: string, prefix: string, suffix: string) {
  let matcher: string | RegExp = original
  if (original.length > 2 && original.startsWith('/')) {
    let body = original.slice(1)
    let flags = ''
    while (body && 'gi'.includes(body.at(-1) ?? '')) {
      flags = `${body.at(-1)}${flags}`
      body = body.slice(0, -1)
    }
    if (body.endsWith('/')) {
      body = body.slice(0, -1)
      matcher = new RegExp(body, flags)
    }
  }
  return names.map((name) => `${prefix}${matcher ? name.replace(matcher, replace) : name}${suffix}`)
}

function BatchRenameContent({ names, onResolve }: { names: string[]; onResolve: (value: string[] | null) => void }) {
  const [original, setOriginal] = useState('')
  const [replace, setReplace] = useState('')
  const [prefix, setPrefix] = useState('')
  const [suffix, setSuffix] = useState('')
  const [submitError, setSubmitError] = useState('')
  const renameResult = useMemo(() => {
    try {
      return { names: buildRenamedNames(names, original, replace, prefix, suffix), error: '' }
    } catch {
      return { names, error: '正则表达式无效' }
    }
  }, [names, original, prefix, replace, suffix])
  const newNames = renameResult.names
  const error = renameResult.error || submitError

  return (
    <Dialog
      okLabel="重命名"
      cancelLabel="取消"
      width="min(900px, calc(100vw - 80px))"
      validate={() => {
        if (renameResult.error) return false
        setSubmitError('')
        if (newNames.every((name, index) => name === names[index])) {
          setSubmitError('没有需要修改的文件名。')
          return false
        }
        if (newNames.some((name) => !name.trim())) {
          setSubmitError('文件名不能为空。')
          return false
        }
        if (new Set(newNames).size !== newNames.length) {
          setSubmitError('重命名后存在重复文件名。')
          return false
        }
        return true
      }}
      onOk={() => onResolve(newNames)}
      onCancel={() => onResolve(null)}
    >
      <div className="typo" style={{ maxHeight: '60vh', overflow: 'auto' }}>
        <div className="row">
          <div className="medium-6 columns">
            <h2>批量替换</h2>
            <label>
              原内容
              <input className="textbox" value={original} onChange={(event) => setOriginal(event.currentTarget.value)} data-autofocus />
            </label>
            <label>
              替换为
              <input className="textbox" value={replace} onChange={(event) => setReplace(event.currentTarget.value)} />
            </label>
            <p className="help-text">支持正则表达式，使用 /pattern/flags 形式。</p>
          </div>
          <div className="medium-6 columns">
            <h2>添加前后缀</h2>
            <label>
              前缀
              <input className="textbox" value={prefix} onChange={(event) => setPrefix(event.currentTarget.value)} />
            </label>
            <label>
              后缀
              <input className="textbox" value={suffix} onChange={(event) => setSuffix(event.currentTarget.value)} />
            </label>
          </div>
        </div>
        {error ? <blockquote className="warn"><p>{error}</p></blockquote> : null}
        <table className="data-table rename-confirm-table">
          <thead><tr><th>原文件名</th><th>新文件名</th></tr></thead>
          <tbody>
            {names.map((name, index) => (
              <tr key={name}><td>{name}</td><td>{newNames[index]}</td></tr>
            ))}
          </tbody>
        </table>
      </div>
    </Dialog>
  )
}

export function batchRenameDialog(names: string[]): Promise<string[] | null> {
  return openDialog<string[] | null>((resolve) => (
    <BatchRenameContent names={names} onResolve={resolve} />
  ))
}

export function alertDialog(message: string): Promise<void> {
  return openDialog<void>((resolve) => (
    <Dialog onOk={() => resolve()} onCancel={() => resolve()} okLabel="确定" cancelLabel="">
      <div className="typo"><p>{message}</p></div>
    </Dialog>
  ))
}
