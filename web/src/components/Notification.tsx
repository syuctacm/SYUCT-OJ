// Ported from Hydro ui-default components/notification/index.ts (AGPL-3.0).
// The host is mounted once by Layout; helpers can be called from anywhere.
import type { ReactNode } from 'react'
import { MantineProvider } from '@mantine/core'
import { Notifications, notifications } from '@mantine/notifications'
import { hydroMantineTheme } from '../upstream/mantine.ts'

export function NotificationsHost() {
  return (
    <MantineProvider theme={hydroMantineTheme}>
      <Notifications position="bottom-left" zIndex={99999} />
    </MantineProvider>
  )
}

export function notifyMessage(options: {
  title: string
  message: ReactNode
  avatar?: string
  onClick?: () => void
  duration?: number
}) {
  return notifications.show({
    title: options.title,
    message: options.message,
    color: '#2d72d2',
    icon: options.avatar
      ? <img src={options.avatar} alt="" width={28} height={28} style={{ borderRadius: '50%', objectFit: 'cover' }} />
      : <i className="icon icon-comment--multiple" />,
    autoClose: options.duration ?? 15000,
    onClick: options.onClick,
  })
}

export function notifySuccess(message: string, duration?: number) {
  return notifications.show({
    title: message,
    color: '#238551',
    message: '',
    icon: <i className="icon icon-check" />,
    autoClose: duration,
  })
}

export function notifyInfo(message: string, duration?: number) {
  return notifications.show({
    title: message,
    color: '#2d72d2',
    message: '',
    icon: <i className="icon icon-info--circle" />,
    autoClose: duration,
  })
}

export function notifyWarn(message: string, duration?: number) {
  return notifications.show({
    title: message,
    color: '#fbb360',
    message: '',
    icon: <i className="icon icon-warning" />,
    autoClose: duration,
  })
}

export function notifyError(message: string, duration?: number) {
  return notifications.show({
    title: message,
    color: '#cd4246',
    message: '',
    icon: <i className="icon icon-close--circle" />,
    autoClose: duration,
  })
}
