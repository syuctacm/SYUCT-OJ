// Page markup is rendered from the pinned Hydro ui-default template. Keep this
// file limited to data and interaction adapters.
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useParams } from 'react-router-dom'
import { getProblem, rejudgeProblem, starProblem, type ProblemDetail } from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { renderMarkdown } from '../upstream/markdown.ts'
import { avatarUrl } from '../upstream/avatar.ts'

function configSummary(problem: ProblemDetail) {
  return {
    type: problem.problemType || 'default',
    timeMin: String(problem.timeLimit || '1000').replace(/ms$/i, ''),
    timeMax: String(problem.timeLimit || '1000').replace(/ms$/i, ''),
    memoryMin: String(problem.memoryLimit || '256').replace(/[a-z]+$/i, ''),
    memoryMax: String(problem.memoryLimit || '256').replace(/[a-z]+$/i, ''),
  }
}

export function ProblemDetailPage() {
  const { pid = '' } = useParams()
  const { user } = useAuth()
  const [problem, setProblem] = useState<ProblemDetail | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    setProblem(null)
    setError('')
    getProblem(pid)
      .then(setProblem)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '题目加载失败'))
  }, [pid])

  const handleReady = useCallback((root: HTMLDivElement) => {
    const cleanups: Array<() => void> = []
    root.querySelectorAll('[name="problem-sidebar__download"], [name="problem-sidebar__copy"]')
      .forEach((element) => element.closest('li')?.remove())

    for (const form of root.querySelectorAll<HTMLFormElement>('form')) {
      const operation = form.querySelector<HTMLInputElement>('input[name="operation"]')?.value
      if (operation === 'star') {
        const onSubmit = (event: SubmitEvent) => {
          event.preventDefault()
          const next = form.querySelector<HTMLInputElement>('input[name="star"]')?.value === 'true'
          void starProblem(pid, next)
            .then(() => getProblem(pid))
            .then(setProblem)
            .catch((err: unknown) => setError(err instanceof Error ? err.message : '收藏失败'))
        }
        form.addEventListener('submit', onSubmit)
        cleanups.push(() => form.removeEventListener('submit', onSubmit))
      } else if (operation === 'rejudge') {
        const onSubmit = (event: SubmitEvent) => {
          event.preventDefault()
          if (!window.confirm('确定重测该题的全部提交吗？')) return
          void rejudgeProblem(pid)
            .then(() => getProblem(pid))
            .then(setProblem)
            .catch((err: unknown) => setError(err instanceof Error ? err.message : '重测失败'))
        }
        form.addEventListener('submit', onSubmit)
        cleanups.push(() => form.removeEventListener('submit', onSubmit))
      }
    }

    const tagToggle = root.querySelector<HTMLElement>('[name="show_tags"]')
    if (tagToggle) {
      const onClick = () => {
        const tags = root.querySelectorAll<HTMLElement>('.problem__tag-item.tags')
        const visible = tags.length > 0 && !tags[0].classList.contains('hasjs--hide')
        tags.forEach((tag) => {
          tag.classList.toggle('hasjs--hide', visible)
          tag.style.display = visible ? 'none' : ''
        })
      }
      tagToggle.addEventListener('click', onClick)
      cleanups.push(() => tagToggle.removeEventListener('click', onClick))
    }
    return () => cleanups.forEach((cleanup) => cleanup())
  }, [pid])

  const context = useMemo(() => {
    if (!problem) return null
    const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
    const pdoc = {
      ...problem,
      tag: problem.tags ?? [],
      config: configSummary(problem),
      // The server-side template only uses this list to decide whether to show a warning.
      data: [{ name: '__testdata__', size: 0 }],
      reference: false,
    }
    const owner = problem.owner ? {
      _id: problem.owner,
      uname: `UID ${problem.owner}`,
      avatar: '',
      hasPriv: () => false,
      hasPerm: () => false,
    } : null
    return {
      page_name: 'problem_detail',
      pdoc,
      tdoc: null,
      rdoc: null,
      psdoc: { star: Boolean(problem.starred), status: 0 },
      udoc: owner,
      owner_udoc: owner,
      mode: 'normal',
      showAccept: true,
      // Imported FPS problemsets keep pre-rendered HTML; the rest is markdown.
      renderredContent: problem.html ? problem.content : renderMarkdown(problem.content || ''),
      handler: {
        user: {
          _id: user?.id ?? 0,
          viewLang: 'zh',
          codeLang: 'cpp',
          _dudoc: { join: true },
          hasPerm: (permission: number) => {
            if (permission === 8) return true
            if (permission === 7) return isAdmin
            return Boolean(user) && [1, 2, 3, 4].includes(permission)
          },
          hasPriv: () => Boolean(user),
          own: () => Boolean(user && (problem.owner === user.id || isAdmin)),
        },
        args: { domainId: 'system' },
        request: { query: { lang: 'zh' }, headers: { 'user-agent': navigator.userAgent } },
        session: { legacy: false, viewLang: 'zh' },
        ctx: { setting: { get: () => '' }, i18n: { get: () => '' } },
      },
      perm: {
        PERM_EDIT_PROBLEM: 1,
        PERM_EDIT_PROBLEM_SELF: 2,
        PERM_SUBMIT_PROBLEM: 3,
        PERM_READ_PROBLEM_DATA: 4,
        PERM_VIEW_DISCUSSION: 5,
        PERM_CREATE_DISCUSSION: 6,
        PERM_REJUDGE_PROBLEM: 7,
        PERM_VIEW_PROBLEM_SOLUTION: 8,
      },
      PRIV: { PRIV_USER_PROFILE: 1, PRIV_READ_PROBLEM_DATA: 2, PRIV_REGISTER_USER: 3 },
      STATUS: { STATUS_ACCEPTED: 1 },
      model: {
        builtin: { LEVELS: {}, STATUS_CODES: {}, STATUS_TEXTS: {} },
        document: { TYPE_CONTEST: 30 },
        setting: { langs: {} },
        system: { get: (key: string) => key === 'server.login' },
        contest: { canShowSelfRecord: () => true },
      },
      lib: { difficulty: () => 0 },
      avatarUrl,
      utils: { getAlphabeticId: () => '' },
      UiContext: {
        cdn_prefix: '/',
        problemId: problem.pid || String(problem.docId),
        problemNumId: problem.docId,
        codeLang: 'cpp',
        codeTemplate: '',
        pdoc,
        canViewRecord: true,
        postSubmitUrl: `/p/${encodeURIComponent(problem.pid || String(problem.docId))}/submit`,
        getSubmissionsUrl: `/record?pid=${encodeURIComponent(String(problem.docId))}`,
        getRecordDetailUrl: '/record/{rid}',
        pretestConnUrl: '',
      },
      UserContext: user,
    }
  }, [problem, user])

  if (error) return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  if (!problem || !context) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>

  return (
    <HydroTemplate
      template="problem_detail.html"
      context={context}
      url={(name, args = {}) => {
        const id = encodeURIComponent(String(args.pid ?? problem.pid ?? problem.docId))
        const query = (args.query ?? {}) as Record<string, unknown>
        const routes: Record<string, string> = {
          problem_main: query.q ? `/p?q=${encodeURIComponent(String(query.q))}` : '/p',
          problem_detail: `/p/${id}`,
          problem_submit: `/p/${id}/submit`,
          problem_files: `/p/${id}/files`,
            problem_solution: `/p/${id}/solution`,
          problem_config: `/p/${id}/config`,
          problem_edit: `/p/${id}/edit`,
          problem_statistics: `/p/${id}/statistics`,
          problem_hack: `/p/${id}`,
          record_main: `/record${query.pid ? `?pid=${encodeURIComponent(String(query.pid))}` : ''}`,
          record_detail: `/record/${encodeURIComponent(String(args.rid ?? ''))}`,
          user_detail: `/user/${encodeURIComponent(String(args.uid ?? ''))}`,
          discussion_node: '/discuss',
        }
        return routes[name] ?? '/'
      }}
      onReady={handleReady}
    />
  )
}
