// Page markup is rendered from the production tag-stat plugin template. Keep
// this file limited to data mapping, routing and small DOM event adapters.
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { getProblemTags, getTagRanking, type ProblemTagStat, type TagRanking } from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { avatarUrl } from '../upstream/avatar.ts'

export function TagRankingPage() {
  const { tag = '' } = useParams()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const query = searchParams.get('q') ?? ''
  const page = Math.max(1, Number.parseInt(searchParams.get('page') ?? '1', 10) || 1)
  const { user } = useAuth()
  const [ranking, setRanking] = useState<TagRanking | null>(null)
  const [allTags, setAllTags] = useState<ProblemTagStat[]>([])
  const [error, setError] = useState('')

  useEffect(() => {
    Promise.all([getTagRanking(tag, page, query), getProblemTags()])
      .then(([data, tags]) => {
        setRanking(data)
        setAllTags(tags)
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '标签榜单加载失败'))
  }, [tag, page, query])

  const handleReady = useCallback((root: HTMLDivElement) => {
    const select = root.querySelector<HTMLSelectElement>('#tagSelectBox')
    const onSelect = () => {
      if (select?.value) navigate(`/p/tag/${encodeURIComponent(select.value)}`)
    }
    select?.addEventListener('change', onSelect)

    const form = root.querySelector<HTMLFormElement>('form[method="get"]')
    const onSubmit = (event: SubmitEvent) => {
      event.preventDefault()
      if (!form) return
      const data = new FormData(form)
      const nextQuery = String(data.get('q') ?? '').trim()
      const params = new URLSearchParams()
      if (nextQuery) params.set('q', nextQuery)
      navigate(`/p/tag/${encodeURIComponent(tag)}${params.toString() ? `?${params.toString()}` : ''}`)
    }
    form?.addEventListener('submit', onSubmit)

    return () => {
      select?.removeEventListener('change', onSelect)
      form?.removeEventListener('submit', onSubmit)
    }
  }, [navigate, tag])

  const context = useMemo(() => {
    if (!ranking) return null
    const totalPages = Math.max(1, Math.ceil(ranking.matchedCount / ranking.limit))
    const rankList = ranking.items.map((item) => {
      const udoc = {
        _id: item.uid,
        uname: item.username,
        displayName: item.displayName,
        avatar: item.avatar,
        hasPriv: () => false,
        hasPerm: () => false,
      }
      return {
        rank: item.rank,
        udoc,
        solved: item.solved,
        submissions: item.submissions,
      }
    })
    return {
      tag,
      allTags,
      problemCount: ranking.problemCount,
      totalUsers: ranking.totalUsers,
      matchedCount: ranking.matchedCount,
      q: query,
      page: ranking.page,
      ppcount: totalPages,
      pageSize: ranking.limit,
      rankList,
      handler: {
        user: {
          _id: user?.id ?? 0,
          avatar: user?.avatar ?? '',
          hasPerm: () => false,
          hasPriv: () => false,
        },
      },
      perm: {
        PERM_VIEW_USER_PRIVATE_INFO: 1,
        PERM_MOD_BADGE: 2,
      },
      PRIV: { PRIV_MOD_BADGE: 1 },
      avatarUrl,
      UiContext: { cdn_prefix: '' },
      model: {
        system: { get: (key: string) => key === 'server.name' ? 'SYUCT OJ' : '' },
        builtin: { LEVELS: {} },
      },
      UserContext: user,
    }
  }, [allTags, query, ranking, tag, user])

  if (error) return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  if (!context) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>

  return (
    <HydroTemplate
      template="problem_tag_stat.html"
      context={context}
      onReady={handleReady}
      url={(name, args = {}) => {
        if (name === 'problem_tag_stat') return `/p/tag/${encodeURIComponent(String(args.tag ?? tag))}`
        if (name === 'problem_tag_stat_export') {
          const filter = (args.query ?? {}) as Record<string, unknown>
          const search = String(filter.q ?? '').trim()
          return `/api/v1/problem-tags/${encodeURIComponent(tag)}/ranking/export${search ? `?q=${encodeURIComponent(search)}` : ''}`
        }
        if (name === 'problem_main') return '/p'
        if (name === 'user_detail') return `/user/${encodeURIComponent(String(args.uid ?? ''))}`
        return '/'
      }}
    />
  )
}
