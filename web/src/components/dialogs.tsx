import { useState } from 'react'
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

export function alertDialog(message: string): Promise<void> {
  return openDialog<void>((resolve) => (
    <Dialog onOk={() => resolve()} onCancel={() => resolve()} okLabel="确定" cancelLabel="">
      <div className="typo"><p>{message}</p></div>
    </Dialog>
  ))
}
