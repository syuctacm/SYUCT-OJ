// Minimal port of the Hydro ui-default ActionDialog chrome
// (components/dialog/index.tsx). The markup and classes match upstream so the
// stylesheet renders the same dialog as the old site.
import { useEffect, type ReactNode } from 'react'
import { createPortal } from 'react-dom'

type Props = {
  children: ReactNode
  okLabel?: string
  cancelLabel?: string
  width?: string
  onOk?: () => void
  onCancel?: () => void
}

export function Dialog({ children, okLabel = '确定', cancelLabel = '取消', width, onOk, onCancel }: Props) {
  useEffect(() => {
    const onKeyUp = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onCancel?.()
    }
    document.addEventListener('keyup', onKeyUp)
    return () => document.removeEventListener('keyup', onKeyUp)
  }, [onCancel])

  return createPortal(
    <div
      className="dialog withBg"
      style={{ display: 'flex', opacity: 1 }}
      onClick={(event) => {
        if (event.target === event.currentTarget) onCancel?.()
      }}
    >
      <div className="dialog__content" style={width ? { width, maxWidth: width } : undefined}>
        <div className="dialog__body">{children}</div>
        <div className="row"><div className="columns clearfix">
          <div className="float-right dialog__action">
            <button type="button" className="rounded button" onClick={onCancel}>{cancelLabel}</button>
            <button type="button" className="primary rounded button" onClick={onOk}>{okLabel}</button>
          </div>
        </div></div>
      </div>
    </div>,
    document.body,
  )
}
