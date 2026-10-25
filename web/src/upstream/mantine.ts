// Ported from @hydrooj/ui-default components/mantine.page.tsx (AGPL-3.0).
import { createTheme, Notification as MantineNotification } from '@mantine/core'

const colorWhite = { color: 'var(--mantine-color-white)' } as const

export const hydroMantineTheme = createTheme({
  components: {
    Notification: MantineNotification.extend({
      classNames: { closeButton: 'mantine-notifications-close-button' },
      styles: {
        root: {
          backgroundColor: 'var(--notification-color, var(--mantine-primary-color-filled))',
          paddingInlineStart: 'var(--mantine-spacing-xs)',
        },
        title: { ...colorWhite, fontSize: 'var(--mantine-font-size-md)' },
        icon: { fontSize: '24px', marginInlineEnd: 'var(--mantine-spacing-xs)' },
        description: colorWhite,
        closeButton: colorWhite,
      },
    }),
  },
})
