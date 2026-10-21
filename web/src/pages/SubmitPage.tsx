// Page markup is rendered from the pinned Hydro ui-default template. Keep this
// file limited to language selection, submission and error state.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { createSubmission, getProblem, getProblemLanguages, type ProblemDetail } from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { languageOptions } from '../components/autocomplete/LanguageSelectAutoComplete.tsx'
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

export function SubmitPage() {
  const { pid = '' } = useParams()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const contestId = searchParams.get('contest') ?? searchParams.get('tid') ?? ''
  const { user, loading } = useAuth()
  const [problem, setProblem] = useState<ProblemDetail | null>(null)
  const [languages, setLanguages] = useState(languageOptions)
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const submitRef = useRef<(event: SubmitEvent) => void>(() => {})

  useEffect(() => {
    getProblem(pid).then(setProblem).catch(() => setProblem(null))
  }, [pid])

  useEffect(() => {
    getProblemLanguages(pid, contestId)
      .then((items) => {
        if (items.length) {
          setLanguages(items.map((item) => ({ _id: item.id, name: item.display })))
        }
      })
      .catch(() => setLanguages(languageOptions))
  }, [contestId, pid])

  const handleReady = useCallback((root: HTMLDivElement) => {
    const form = root.querySelector('form')
    const onSubmit = (event: SubmitEvent) => submitRef.current(event)
    form?.addEventListener('submit', onSubmit)

    const fallback = root.querySelector('.hasjs--hide')
    fallback?.remove()
    const selector = root.querySelector<HTMLElement>('#codelang-selector')
    if (selector) {
      selector.classList.remove('nojs--hide')
      selector.style.display = ''
      selector.replaceChildren()
      const field = document.createElement('div')
      field.className = 'medium-5 columns form__item end'
      const label = document.createElement('label')
      label.textContent = '代码语言'
      const container = document.createElement('div')
      container.className = 'select-container'
      const select = document.createElement('select')
      select.name = 'lang'
      select.className = 'select'
      for (const language of languages) {
        const option = document.createElement('option')
        option.value = language._id
        option.textContent = language.name
        select.appendChild(option)
      }
      select.value = languages[0]?._id ?? 'cpp'
      container.appendChild(select)
      label.appendChild(container)
      field.appendChild(label)
      selector.appendChild(field)
    }

    const fileRow = root.querySelector<HTMLInputElement>('input[name="file"]')?.closest('.row')
    fileRow?.remove()

    const onKeyDown = (event: KeyboardEvent) => {
      if (!(event.ctrlKey || event.metaKey) || event.key !== 'Enter') return
      event.preventDefault()
      form?.requestSubmit()
    }
    form?.addEventListener('keydown', onKeyDown)
    return () => {
      form?.removeEventListener('submit', onSubmit)
      form?.removeEventListener('keydown', onKeyDown)
      selector?.replaceChildren()
    }
  }, [languages])

  const context = useMemo(() => {
    if (!problem) return null
    const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
    return {
      page_name: 'problem_submit',
      pdoc: { ...problem, tag: problem.tags ?? [], config: configSummary(problem) },
      tdoc: null,
      langRange: Object.fromEntries(languages.map((language) => [language._id, language.name])),
      handler: {
        user: {
          _id: user?.id ?? 0,
          viewLang: 'zh',
          codeLang: 'cpp',
          _dudoc: { join: true },
          hasPerm: (permission: number) => permission === 8 || (Boolean(user) && [1, 2, 3, 4].includes(permission)),
          hasPriv: () => Boolean(user),
          own: () => Boolean(user && (problem.owner === user.id || isAdmin)),
        },
        ctx: { setting: { get: () => '' }, i18n: { get: () => '' } },
        request: { query: {}, headers: { 'user-agent': navigator.userAgent } },
        session: { legacy: false, viewLang: 'zh' },
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
      psdoc: { status: 1 },
      tdocs: [],
      ctdocs: [],
      htdocs: [],
      discussionCount: 0,
      solutionCount: 0,
      UiContext: { cdn_prefix: '/' },
      avatarUrl,
      model: {
        builtin: { LEVELS: {} },
        system: { get: (key: string) => key === 'server.login' },
        setting: { SETTINGS_BY_KEY: {} },
      },
      lib: { difficulty: () => 0 },
      utils: { getAlphabeticId: () => '' },
      UserContext: user,
    }
  }, [languages, problem, user])

  async function handleSubmit(event: SubmitEvent) {
    event.preventDefault()
    if (!user) return
    const form = event.target as HTMLFormElement
    const data = new FormData(form)
    const lang = String(data.get('lang') ?? languages[0]?._id ?? 'cpp')
    const code = String(data.get('code') ?? '')
    setSubmitting(true)
    setError('')
    try {
      const submission = await createSubmission(pid, lang, code, contestId)
      navigate(`/record/${submission.id}`)
    } catch (err) {
      setError(err instanceof Error ? err.message : '提交失败')
    } finally {
      setSubmitting(false)
    }
  }

  submitRef.current = (event) => void handleSubmit(event)

  if (loading) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">正在确认登录状态...</div></div></div>
  }
  if (!user) {
    return <div className="row"><div className="columns"><blockquote className="note"><p>请先 <Link to="/login">登录</Link> 后提交。</p></blockquote></div></div>
  }
  if (!problem || !context) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>
  }

  return (
    <>
      {error ? <div className="row"><div className="columns"><blockquote className="warn"><p>{error}</p></blockquote></div></div> : null}
      {submitting ? <div className="row"><div className="columns"><blockquote className="note"><p>提交中...</p></blockquote></div></div> : null}
      {contestId ? <div className="row"><div className="columns"><blockquote className="note"><p>比赛提交 · {contestId}</p></blockquote></div></div> : null}
      <HydroTemplate
        template="problem_submit.html"
        context={context}
        url={(name, args = {}) => {
          const id = encodeURIComponent(String(args.pid ?? problem.pid ?? problem.docId))
          const query = (args.query ?? {}) as Record<string, unknown>
          const routes: Record<string, string> = {
            problem_main: query.q ? `/p?q=${encodeURIComponent(String(query.q))}` : '/p',
            problem_detail: `/p/${id}`,
            problem_submit: `/p/${id}/submit${contestId ? `?contest=${encodeURIComponent(contestId)}` : ''}`,
            problem_files: `/p/${id}/files`,
            problem_solution: `/p/${id}/solution`,
            problem_config: `/p/${id}/config`,
            problem_edit: `/p/${id}/edit`,
            problem_statistics: `/p/${id}/statistics`,
            record_main: `/record${query.pid ? `?pid=${encodeURIComponent(String(query.pid))}` : ''}`,
            user_detail: `/user/${encodeURIComponent(String(args.uid ?? ''))}`,
          }
          return routes[name] ?? '/'
        }}
        onReady={handleReady}
      />
    </>
  )
}
