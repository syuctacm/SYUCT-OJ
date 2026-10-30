// Ported from @hydrooj/ui-default pages/contest.page.ts (AGPL-3.0).
// The timer markup, NProgress settings and clock behavior stay upstream.
import NProgress from 'nprogress'
import { useEffect } from 'react'

type ContestTimerInput = {
  beginAt: string | Date
  endAt: string | Date
  rule?: string
  status?: {
    startAt?: string | Date
    endAt?: string | Date
  }
}

function pad(value: number) {
  return value < 10 ? `0${value}` : String(value)
}

// Hydro framework/utils/lib/common.ts formatSeconds.
function formatSeconds(seconds: number) {
  const value = Math.max(0, Math.floor(seconds))
  return `${pad(Math.floor(value / 3600))}:${pad(Math.floor((value % 3600) / 60))}:${pad(value % 60)}`
}

export function useHydroContestTimer(contest: ContestTimerInput | null) {
  useEffect(() => {
    if (!contest || contest.rule === 'homework') return
    const beginAt = new Date(contest.status?.startAt ?? contest.beginAt).getTime()
    const endAt = new Date(contest.status?.endAt ?? contest.endAt).getTime()
    if (!Number.isFinite(beginAt) || !Number.isFinite(endAt) || endAt <= beginAt) return

    const timer = document.createElement('pre')
    timer.className = 'contest-timer'
    timer.style.display = 'none'
    document.body.append(timer)

    NProgress.configure({ trickle: false, showSpinner: false, minimum: 0 })
    NProgress.start()

    const update = () => {
      const now = Date.now()
      if (beginAt <= now && now <= endAt) {
        NProgress.set((now - beginAt) / (endAt - beginAt))
        timer.style.display = ''
        timer.textContent = formatSeconds(Math.floor((endAt - now) / 1000))
      } else {
        NProgress.set(now > endAt ? 1 : 0)
        timer.style.display = 'none'
      }
    }

    update()
    const interval = window.setInterval(update, 1000)
    return () => {
      window.clearInterval(interval)
      timer.remove()
      NProgress.done(true)
      NProgress.remove()
    }
  }, [contest])
}
