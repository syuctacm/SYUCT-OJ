// Desktop sign-in dialog, ported from Hydro ui-default
// components/signin/signInDialog.page.js and partials/login_dialog.html.
// The template, motion and markup stay upstream; React owns authentication.
import $ from 'jquery'
import { createPortal } from 'react-dom'
import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from '../auth.tsx'
import { loginPath } from '../login-redirect.ts'
import { buildLayoutUrl, layoutChromeContext } from '../upstream/layout.ts'
import { renderHydroTemplate } from '../upstream/render.ts'

declare global {
  interface Window {
    showSignInDialog?: () => void
  }
}

const MOBILE_QUERY = '(max-width: 599px)'

function isMobile() {
  return window.matchMedia(MOBILE_QUERY).matches
}

function messageForLoginError(error: unknown) {
  const message = error instanceof Error ? error.message : ''
  if (message === 'invalid credentials') return '用户名或密码错误'
  if (message === '2FA required') return '请输入双重验证码'
  if (message === 'invalid 2FA code') return '双重验证码错误'
  return message || '登录失败'
}

export function SignInDialog() {
  const { login } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const [open, setOpen] = useState(false)
  const [error, setError] = useState('')
  const [tfaRequired, setTfaRequired] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const rootRef = useRef<HTMLDivElement>(null)
  const closingRef = useRef(false)
  const previousFocusRef = useRef<HTMLElement | null>(null)
  const currentLocation = useRef(location)
  const submitRef = useRef<(event: SubmitEvent) => void>(() => {})
  currentLocation.current = location

  const context = useMemo(() => {
    const base = layoutChromeContext(null, 'login')
    return {
      ...base,
      loginMethods: [],
      handler: {
        ...base.handler,
        loginMethods: [],
      },
    }
  }, [])
  const html = useMemo(() => renderHydroTemplate('partials/login_dialog.html', {
    context,
    url: buildLayoutUrl,
  }), [context])

  const show = useCallback(() => {
    if (isMobile()) {
      const next = currentLocation.current
      window.location.assign(loginPath(`${next.pathname}${next.search}${next.hash}`))
      return
    }
    setError('')
    setTfaRequired(false)
    setSubmitting(false)
    closingRef.current = false
    previousFocusRef.current = document.activeElement as HTMLElement | null
    setOpen(true)
  }, [])

  const hide = useCallback(async () => {
    if (closingRef.current) return
    closingRef.current = true
    const root = rootRef.current
    if (!root) {
      setOpen(false)
      previousFocusRef.current?.focus()
      closingRef.current = false
      return
    }
    const $dialog = $(root).find('.dialog--signin')
    if (!$dialog.length) {
      setOpen(false)
      previousFocusRef.current?.focus()
      closingRef.current = false
      return
    }
    $dialog.css({ opacity: 1 }).transition({ opacity: 0 }, 200)
    const $content = $dialog.find('.dialog__content')
    $content.css({ scale: 1 })
    await $content.transition({ scale: 0.8 }, 200, 'easeOutCubic').promise()
    setOpen(false)
    previousFocusRef.current?.focus()
    closingRef.current = false
  }, [])

  useEffect(() => {
    window.showSignInDialog = show
    const onLoginClick = (event: MouseEvent) => {
      if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return
      const target = event.target
      if (!(target instanceof Element)) return
      const anchor = target.closest<HTMLAnchorElement>('a[name="nav_login"], a[href^="javascript:showSignInDialog"]')
      if (!anchor) return
      if (isMobile()) {
        if (anchor.getAttribute('name') === 'nav_login') return
        event.preventDefault()
        show()
        return
      }
      event.preventDefault()
      event.stopPropagation()
      show()
    }
    document.addEventListener('click', onLoginClick, true)
    return () => {
      document.removeEventListener('click', onLoginClick, true)
      if (window.showSignInDialog === show) delete window.showSignInDialog
    }
  }, [show])

  useEffect(() => {
    if (!open) return
    const dialog = rootRef.current?.querySelector<HTMLElement>('.dialog--signin')
    if (!dialog) return
    const $dialog = $(dialog)
    const $content = $dialog.find('.dialog__content')
    $dialog.css({ display: 'flex', opacity: 0 })
    $dialog.width()
    $dialog.transition({ opacity: 1 }, 100, 'easeOutCubic')
    $content.css({ scale: 0.8 })
    void $content.transition({ scale: 1 }, 200, 'easeOutCubic').promise()
    dialog.querySelector<HTMLElement>('[data-autofocus]')?.focus()

    const onKeyUp = (event: KeyboardEvent) => {
      if (event.key === 'Escape') void hide()
    }
    const onClick = (event: Event) => {
      const target = event.target
      if (target === dialog) {
        void hide()
        return
      }
      if (!(target instanceof Element)) return
      if (target.closest('[name="dialog--signin__close"]')) {
        event.preventDefault()
        void hide()
      } else if (target.closest('[data-lostpass]')) {
        event.preventDefault()
        void hide()
        navigate('/lostpass')
      }
    }
    const form = dialog.querySelector<HTMLFormElement>('form')
    const onSubmit = (event: SubmitEvent) => submitRef.current(event)
    form?.addEventListener('submit', onSubmit)
    document.addEventListener('keyup', onKeyUp)
    dialog.addEventListener('click', onClick)
    return () => {
      document.removeEventListener('keyup', onKeyUp)
      dialog.removeEventListener('click', onClick)
      form?.removeEventListener('submit', onSubmit)
    }
  }, [hide, navigate, open])

  useLayoutEffect(() => {
    if (!open || !rootRef.current) return
    const dialog = rootRef.current.querySelector<HTMLElement>('.dialog--signin')
    if (!dialog) return
    const main = dialog.querySelector<HTMLElement>('.dialog--signin__main')
    const form = dialog.querySelector<HTMLFormElement>('form')
    if (!main || !form) return

    let notice = main.querySelector<HTMLElement>('[data-signin-error]')
    if (error) {
      if (!notice) {
        notice = document.createElement('blockquote')
        notice.className = 'warn'
        notice.dataset.signinError = '1'
        notice.innerHTML = '<p></p>'
        main.insertBefore(notice, form)
      }
      const paragraph = notice.querySelector('p')
      if (paragraph) paragraph.textContent = error
    } else {
      notice?.remove()
    }

    if (tfaRequired) {
      let input = form.querySelector<HTMLInputElement>('input[name="tfa"][data-signin-tfa]')
      if (!input) {
        const hidden = form.querySelector<HTMLInputElement>('input[name="tfa"]')
        const submitRow = hidden?.closest('.row')
        if (hidden && submitRow?.parentElement) {
          const row = document.createElement('div')
          row.className = 'row'
          row.dataset.signinTfaRow = '1'
          row.innerHTML = '<div class="columns"><label class="material textbox">双重验证码<input name="tfa" type="text" inputmode="numeric" autocomplete="one-time-code" maxlength="6" required></label></div>'
          submitRow.parentElement.insertBefore(row, submitRow)
          hidden.remove()
          input = row.querySelector<HTMLInputElement>('input')
          if (input) input.dataset.signinTfa = '1'
          input?.focus()
        }
      }
    }

    const submit = form.querySelector<HTMLButtonElement>('input[type="submit"]')
    if (submit) {
      submit.disabled = submitting
      submit.value = submitting ? '登录中...' : '登录'
    }
  }, [error, open, submitting, tfaRequired])

  async function handleSubmit(event: SubmitEvent) {
    event.preventDefault()
    if (submitting) return
    const form = event.target as HTMLFormElement
    const data = new FormData(form)
    const username = String(data.get('uname') ?? '').trim()
    const password = String(data.get('password') ?? '')
    const tfa = String(data.get('tfa') ?? '').trim()
    setSubmitting(true)
    setError('')
    try {
      await login(username, password, tfa)
      if (currentLocation.current.pathname === '/login') {
        const redirect = new URLSearchParams(currentLocation.current.search).get('redirect')
        navigate(redirect || '/', { replace: true })
      } else {
        await hide()
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : ''
      if (message === '2FA required' || message === 'invalid 2FA code') setTfaRequired(true)
      setError(messageForLoginError(err))
    } finally {
      setSubmitting(false)
    }
  }
  submitRef.current = (event) => void handleSubmit(event)

  if (!open) return null
  return createPortal(
    <div ref={rootRef} dangerouslySetInnerHTML={{ __html: html }} />,
    document.body,
  )
}
