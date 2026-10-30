// @ts-nocheck -- animation values and dialog structure are ported from
// @hydrooj/ui-default components/dialog/DomDialog.ts and dialog/index.tsx.
import 'jquery.easing'
import 'jquery.transit'

import $ from 'jquery'
import { useCallback, useEffect, useLayoutEffect, useRef, type ReactNode } from 'react'
import { createPortal } from 'react-dom'

type Props = {
  children: ReactNode
  okLabel?: string
  cancelLabel?: string
  width?: string
  onOk?: () => void
  onCancel?: () => void
  validate?: () => boolean
}

export function Dialog({
  children,
  okLabel = '确定',
  cancelLabel = '取消',
  width,
  onOk,
  onCancel,
  validate,
}: Props) {
  const wrapRef = useRef<HTMLDivElement>(null)
  const contentRef = useRef<HTMLDivElement>(null)
  const closingRef = useRef(false)
  const previousFocusRef = useRef<HTMLElement | null>(null)
  const onOkRef = useRef(onOk)
  const onCancelRef = useRef(onCancel)
  onOkRef.current = onOk
  onCancelRef.current = onCancel

  const close = useCallback(async (action: 'ok' | 'cancel') => {
    if (closingRef.current) return
    closingRef.current = true
    const $wrap = $(wrapRef.current)
    const $content = $(contentRef.current)
    $wrap.css({ opacity: 1 }).transition({ opacity: 0 }, { duration: 200 })
    $content.css({ scale: 1 })
    await $content.transition({ scale: 0.8 }, { duration: 200, easing: 'easeOutCubic' }).promise()
    previousFocusRef.current?.focus()
    if (action === 'ok') onOkRef.current?.()
    else onCancelRef.current?.()
  }, [])

  useLayoutEffect(() => {
    const wrap = wrapRef.current
    const content = contentRef.current
    if (!wrap || !content) return
    previousFocusRef.current = document.activeElement as HTMLElement | null
    const $wrap = $(wrap)
    const $content = $(content)
    $wrap.css({ display: 'flex', opacity: 0 })
    $wrap.width()
    $wrap.transition({ opacity: 1 }, { duration: 100, easing: 'easeOutCubic' })
    $content.css({ scale: 0.8 })
    $content.transition({ scale: 1 }, { duration: 200, easing: 'easeOutCubic' })
    const focusTarget = content.querySelector<HTMLElement>(
      '[data-autofocus], [autofocus], input:not([type="hidden"]), textarea, select, button',
    )
    focusTarget?.focus()
  }, [])

  useEffect(() => {
    const onKeyUp = (event: KeyboardEvent) => {
      if (event.key === 'Escape') void close('cancel')
    }
    document.addEventListener('keyup', onKeyUp)
    return () => document.removeEventListener('keyup', onKeyUp)
  }, [close])

  return createPortal(
    <div
      ref={wrapRef}
      className="dialog withBg"
      style={{ display: 'none', zIndex: 100000 }}
      onClick={(event) => {
        if (event.target === event.currentTarget) void close('cancel')
      }}
    >
      <div
        ref={contentRef}
        className="dialog__content"
        style={width ? { width, maxWidth: width } : undefined}
      >
        <div className="dialog__body">{children}</div>
        <div className="row"><div className="columns clearfix">
          <div className="float-right dialog__action">
            {cancelLabel ? (
              <button type="button" className="rounded button" onClick={() => void close('cancel')}>
                {cancelLabel}
              </button>
            ) : null}
            <button
              type="button"
              className="primary rounded button"
              onClick={() => {
                if (validate?.() === false) return
                void close('ok')
              }}
            >
              {okLabel}
            </button>
          </div>
        </div></div>
      </div>
    </div>,
    document.body,
  )
}
