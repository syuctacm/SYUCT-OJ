import { useLayoutEffect, useMemo } from 'react'
import { useLocation } from 'react-router-dom'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { applyPageMeta } from '../page-meta.ts'

export type ErrorPageProps = {
  status: 403 | 404 | 500
  message?: string
  name?: string
}

const DEFAULTS: Record<ErrorPageProps['status'], { message: string; name: string }> = {
  403: { message: '您没有权限访问此页面。', name: 'PermissionDeniedError' },
  404: { message: '页面不存在。', name: 'NotFoundError' },
  500: { message: '页面加载失败，请稍后重试。', name: 'InternalServerError' },
}

export function ErrorPage({ status, message, name }: ErrorPageProps) {
  const location = useLocation()
  const defaults = DEFAULTS[status]
  const context = useMemo(() => ({
    page_name: 'error',
    error: {
      message: message || defaults.message,
      name: name || defaults.name,
      params: [],
    },
    handler: {
      request: {
        path: location.pathname,
        originalPath: `${location.pathname}${location.search}`,
        hostname: window.location.host,
      },
    },
  }), [defaults.message, defaults.name, location.pathname, location.search, message, name])

  useLayoutEffect(() => {
    applyPageMeta({
      name: 'error',
      layout: 'basic',
      title: `${status} ${status === 403 ? '无权限' : status === 404 ? '页面不存在' : '服务异常'}`,
    })
  }, [status])

  return <HydroTemplate template="error.html" context={context} />
}

export function NotFoundPage() {
  return <ErrorPage status={404} />
}
