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

export function alertDialog(message: string): Promise<void> {
  return openDialog<void>((resolve) => (
    <Dialog onOk={() => resolve()} onCancel={() => resolve()} okLabel="确定" cancelLabel="">
      <div className="typo"><p>{message}</p></div>
    </Dialog>
  ))
}
