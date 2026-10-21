// Ported from Hydro ui-default components/notification/index.ts (AGPL-3.0).
// The host is mounted once by Layout; helpers can be called from anywhere.
import { MantineProvider } from '@mantine/core'
import { Notifications, notifications } from '@mantine/notifications'

export function NotificationsHost() {
  return (
    <MantineProvider>
      <Notifications position="bottom-left" zIndex={99999} />
    </MantineProvider>
  )
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
