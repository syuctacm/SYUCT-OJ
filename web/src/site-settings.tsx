import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { getPublicSettings, type PublicSettings } from './api.ts'
import { setGravatarURL } from './upstream/avatar.ts'
import { applyPaginationSettings } from './pagination.ts'

export const DEFAULT_SITE_SETTINGS: PublicSettings = {
  bulletin: '',
  siteName: 'SYUCT ACM OJ',
  navLogo: '/nav_logo_dark.png',
  footerExtraHtml: '沈阳化工大学 ACM-ICPC 集训队',
  enableScratchpad: true,
  gravatarUrl: '//cn.gravatar.com/avatar/',
  homepageYaml: `- width: 9
  bulletin: true
  contest: 5
  homework: 10
  training: 10
  ranking: 10
  discussion: 20
- width: 3
  checkin: true
  countdown: true
  hitokoto: true
  starred_problems: 50
  recent_problems: 10
  discussion_nodes: true
  problem_search: true
  suggestion: true
`,
  aboutMarkdown: '',
  paginationProblem: 100,
  paginationContest: 20,
  paginationDiscussion: 50,
  paginationRecord: 100,
  paginationRanking: 100,
  paginationSolution: 20,
  paginationTraining: 10,
}

let currentSiteName = DEFAULT_SITE_SETTINGS.siteName
let initialLoad: Promise<PublicSettings> | null = null

export function getCurrentSiteName() {
  return currentSiteName
}

function applyRuntimeSettings(settings: PublicSettings) {
  const previousSiteName = currentSiteName
  currentSiteName = settings.siteName || DEFAULT_SITE_SETTINGS.siteName
  setGravatarURL(settings.gravatarUrl)
  applyPaginationSettings(settings)
  for (const name of new Set([previousSiteName, DEFAULT_SITE_SETTINGS.siteName])) {
    const suffix = ` - ${name}`
    if (!document.title.endsWith(suffix)) continue
    document.title = `${document.title.slice(0, -suffix.length)} - ${currentSiteName}`
    break
  }
}

type SiteSettingsContextValue = {
  settings: PublicSettings
  loading: boolean
  refresh: () => Promise<PublicSettings>
}

const SiteSettingsContext = createContext<SiteSettingsContextValue | null>(null)

export function SiteSettingsProvider({ children }: { children: ReactNode }) {
  const [settings, setSettings] = useState(DEFAULT_SITE_SETTINGS)
  const [loading, setLoading] = useState(true)

  const refresh = useCallback(async () => {
    const next = await getPublicSettings()
    applyRuntimeSettings(next)
    setSettings(next)
    setLoading(false)
    return next
  }, [])

  useEffect(() => {
    let active = true
    initialLoad ??= getPublicSettings()
    initialLoad
      .then((next) => {
        if (!active) return
        applyRuntimeSettings(next)
        setSettings(next)
      })
      .catch(() => {})
      .finally(() => {
        if (active) setLoading(false)
      })
    return () => {
      active = false
    }
  }, [])

  const value = useMemo(() => ({ settings, loading, refresh }), [loading, refresh, settings])
  return <SiteSettingsContext.Provider value={value}>{children}</SiteSettingsContext.Provider>
}

export function useSiteSettings() {
  const value = useContext(SiteSettingsContext)
  if (!value) throw new Error('useSiteSettings must be used inside SiteSettingsProvider')
  return value
}
