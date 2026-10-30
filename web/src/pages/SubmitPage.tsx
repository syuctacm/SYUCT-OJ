// Page markup is rendered from the pinned Hydro ui-default template. Keep this
// file limited to language selection, submission and error state.
import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { createSubmission, getProblem, getProblemLanguages, getUsersByIDs, type ProblemDetail } from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { languageOptions } from '../components/autocomplete/LanguageSelectAutoComplete.tsx'
import { applyPageMeta } from '../page-meta.ts'
import { activityRoute } from '../upstream/activity-url.ts'
import { avatarUrl } from '../upstream/avatar.ts'
import { CONTEST_PERMS, createContestModel } from '../upstream/contest.ts'
import { useHydroContestTimer } from '../upstream/contest-timer.ts'

const MONACO_LANGUAGES: Record<string, string> = {
  cpp: 'cpp',
  c: 'c',
  python3: 'python',
  python: 'python',
  java: 'java',
  go: 'go',
  rust: 'rust',
  pascal: 'pascal',
  nodejs: 'javascript',
  javascript: 'javascript',
}

function monacoLanguage(language: string) {
  return MONACO_LANGUAGES[language] ?? 'plaintext'
}

const VIEW_PROBLEM_SOLUTION_PERM = 16

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
  const [users, setUsers] = useState<Array<{ id: number; username: string; displayName?: string; avatar?: string }>>([])
  const [languages, setLanguages] = useState(languageOptions)
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const submitRef = useRef<(event: SubmitEvent) => void>(() => {})
  const preferredLanguage = user?.codeLang && languages.some((language) => language._id === user.codeLang)
    ? user.codeLang
    : languages[0]?._id ?? 'cpp'
  const preferredTemplate = user?.codeTemplate ?? ''
  const timerContest = useMemo(() => problem?.contest ? {
    ...problem.contest,
    status: problem.contestStatus,
  } : null, [problem])
  useHydroContestTimer(timerContest)

  useEffect(() => {
    setError('')
    getProblem(pid, contestId)
      .then(setProblem)
      .catch((err: unknown) => {
        setProblem(null)
        setError(err instanceof Error ? err.message : '题目加载失败')
      })
  }, [contestId, pid])

  useEffect(() => {
    if (!problem) {
      setUsers([])
      return
    }
    const ids = [...new Set([problem.owner, problem.contest?.owner].filter((id): id is number => Boolean(id)))]
    getUsersByIDs(ids).then(setUsers).catch(() => setUsers([]))
  }, [problem])

  useLayoutEffect(() => {
    if (!problem) return
    const name = !problem.contest
      ? 'problem_submit'
      : problem.contest.rule === 'homework'
        ? 'homework_detail_problem_submit'
        : 'contest_detail_problem_submit'
    applyPageMeta({ name, layout: 'basic', title: `提交 - ${problem.title}` })
  }, [problem])

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
    let languageSelect: HTMLSelectElement | null = null
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
      select.value = preferredLanguage
      languageSelect = select
      container.appendChild(select)
      label.appendChild(container)
      field.appendChild(label)
      selector.appendChild(field)
    }

    const codeTextarea = root.querySelector<HTMLTextAreaElement>('textarea[name="code"]')
    const fileInput = root.querySelector<HTMLInputElement>('input[name="file"]')
    let disposed = false
    let editorController: { dispose: () => void; setValue: (value: string) => void; setLanguage: (language: string) => void; focus: () => void } | null = null
    let currentLanguage = monacoLanguage(languageSelect?.value ?? preferredLanguage)

    if (codeTextarea) {
      if (!codeTextarea.value && preferredTemplate) {
        codeTextarea.value = preferredTemplate
        codeTextarea.dispatchEvent(new Event('change', { bubbles: true }))
      }
      void import('../upstream/monaco/textarea.ts')
        .then(({ mountMonacoTextarea }) => {
          if (disposed) return
          editorController = mountMonacoTextarea(codeTextarea, currentLanguage)
        })
        .catch((err: unknown) => {
          if (!disposed) setError(err instanceof Error ? err.message : '代码编辑器加载失败')
        })
    }

    const onLanguageChange = () => {
      currentLanguage = monacoLanguage(languageSelect?.value ?? 'cpp')
      editorController?.setLanguage(currentLanguage)
    }
    languageSelect?.addEventListener('change', onLanguageChange)

    const onFileChange = async () => {
      const file = fileInput?.files?.[0]
      if (!file) return
      try {
        const content = await file.text()
        if (editorController) {
          editorController.setValue(content)
          editorController.focus()
        } else if (codeTextarea) {
          codeTextarea.value = content
          codeTextarea.dispatchEvent(new Event('input', { bubbles: true }))
          codeTextarea.dispatchEvent(new Event('change', { bubbles: true }))
        }
      } catch (err) {
        setError(err instanceof Error ? err.message : '代码文件读取失败')
      }
    }
    fileInput?.addEventListener('change', onFileChange)

    const onKeyDown = (event: KeyboardEvent) => {
      if (!(event.ctrlKey || event.metaKey) || event.key !== 'Enter') return
      event.preventDefault()
      form?.requestSubmit()
    }
    if (!codeTextarea) form?.addEventListener('keydown', onKeyDown)

    return () => {
      disposed = true
      editorController?.dispose()
      form?.removeEventListener('submit', onSubmit)
      form?.removeEventListener('keydown', onKeyDown)
      languageSelect?.removeEventListener('change', onLanguageChange)
      fileInput?.removeEventListener('change', onFileChange)
      selector?.replaceChildren()
    }
  }, [languages, preferredLanguage, preferredTemplate])

  const context = useMemo(() => {
    if (!problem) return null
    const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
    const contest = problem.contest
    const canEditProblem = Boolean(user && (problem.owner === user.id || isAdmin))
    const canEditContest = Boolean(user && contest && (contest.owner === user.id || isAdmin))
    const tdoc = contest ? {
      ...contest,
      _id: contest.docId,
      docType: 30,
      beginAt: new Date(contest.beginAt),
      endAt: new Date(contest.endAt),
      lockAt: contest.lockAt ? new Date(contest.lockAt) : null,
      penaltySince: contest.penaltySince ? new Date(contest.penaltySince) : null,
      pids: contest.pids ?? [],
      domainId: 'system',
    } : null
    const tsdoc = contest ? {
      attend: Boolean(problem.contestStatus?.attend),
      startAt: problem.contestStatus?.startAt ? new Date(problem.contestStatus.startAt) : undefined,
      endAt: problem.contestStatus?.endAt ? new Date(problem.contestStatus.endAt) : undefined,
      detail: problem.contestStatuses ?? {},
    } : null
    const udict = Object.fromEntries(users.map((item) => [String(item.id), {
      _id: item.id,
      uname: item.username,
      displayName: item.displayName ?? '',
      avatar: item.avatar ?? '',
    }]))
    const ownerID = contest?.owner ?? problem.owner
    const ownerUser = users.find((item) => item.id === ownerID)
    const ownerUdoc = ownerID ? {
      _id: ownerID,
      uname: ownerUser?.username ?? `UID ${ownerID}`,
      displayName: ownerUser?.displayName ?? '',
      avatar: ownerUser?.avatar ?? '',
      hasPriv: () => false,
      hasPerm: () => false,
    } : null
    const handler = {
      user: {
        _id: user?.id ?? 0,
        viewLang: 'zh',
        codeLang: preferredLanguage,
        codeTemplate: preferredTemplate,
        _dudoc: { join: true },
        hasPerm: (permission: number) => {
          if (!contest) {
            if (permission === VIEW_PROBLEM_SOLUTION_PERM || permission === 9) return true
            if (permission === 8) return Boolean(user)
            if (permission === 7) return canEditProblem
            return Boolean(user) && [1, 2, 3, 4, 5, 6].includes(permission)
          }
          if (permission === CONTEST_PERMS.PERM_VIEW_CONTEST_HIDDEN_SCOREBOARD) return canEditContest
          if (permission === CONTEST_PERMS.PERM_VIEW_HOMEWORK_HIDDEN_SCOREBOARD) return canEditContest
          if (permission === CONTEST_PERMS.PERM_ATTEND_CONTEST) return true
          if (permission === CONTEST_PERMS.PERM_EDIT_CONTEST) return canEditContest
          if (permission === CONTEST_PERMS.PERM_READ_RECORD_CODE) return canEditContest || canEditProblem
          if (permission === CONTEST_PERMS.PERM_CREATE_DISCUSSION) return Boolean(user)
          if (permission === CONTEST_PERMS.PERM_VIEW_DISCUSSION) return true
          if (permission === CONTEST_PERMS.PERM_ATTEND_HOMEWORK) return Boolean(user)
          if (permission === CONTEST_PERMS.PERM_EDIT_HOMEWORK) return canEditContest
          if (permission === CONTEST_PERMS.PERM_VIEW_HOMEWORK_SCOREBOARD) return true
          if (permission === 1) return canEditProblem
          if (permission === 2) return canEditProblem
          if (permission === 3) return canEditContest
          if (permission === 4) return canEditContest
          return false
        },
        hasPriv: (privilege: number) => privilege === 1 ? Boolean(user) : privilege === 2 ? canEditProblem : false,
        own: (doc?: unknown) => {
          if (!doc) return canEditProblem
          return Boolean(user && (doc as { owner?: number }).owner === user.id)
        },
      },
      ctx: { setting: { get: () => '' }, i18n: { get: () => '' } },
      request: {
        query: { ...(contestId ? { tid: contestId } : {}), lang: 'zh' },
        headers: { 'user-agent': navigator.userAgent },
      },
      session: { legacy: false, viewLang: 'zh' },
    }
    const contestModel = contest ? createContestModel(handler) : null
    return {
      page_name: !contest
        ? 'problem_submit'
        : contest.rule === 'homework'
          ? 'homework_detail_problem_submit'
          : 'contest_detail_problem_submit',
      pdoc: { ...problem, tag: contest ? [] : problem.tags ?? [], config: configSummary(problem), reference: false },
      tdoc,
      tsdoc,
      langRange: Object.fromEntries(languages.map((language) => [language._id, language.name])),
      udict,
      owner_udoc: ownerUdoc,
      handler,
      perm: {
        ...{
          PERM_EDIT_PROBLEM: 1,
          PERM_EDIT_PROBLEM_SELF: 2,
          PERM_SUBMIT_PROBLEM: 3,
          PERM_READ_PROBLEM_DATA: 4,
          PERM_VIEW_DISCUSSION: 5,
          PERM_CREATE_DISCUSSION: 6,
          PERM_REJUDGE_PROBLEM: 7,
          PERM_VIEW_PROBLEM_SOLUTION: VIEW_PROBLEM_SOLUTION_PERM,
        },
        ...CONTEST_PERMS,
        PERM_VIEW_PROBLEM_SOLUTION: VIEW_PROBLEM_SOLUTION_PERM,
      },
      PRIV: { PRIV_USER_PROFILE: 1, PRIV_READ_PROBLEM_DATA: 2, PRIV_REGISTER_USER: 3 },
      STATUS: { STATUS_ACCEPTED: 1 },
      psdoc: contest ? null : { status: 1 },
      tdocs: [],
      ctdocs: [],
      htdocs: [],
      discussionCount: 0,
      solutionCount: 0,
      UiContext: {
        cdn_prefix: '/', codeLang: preferredLanguage, codeTemplate: preferredTemplate,
        ...(contest ? { tdoc, tsdoc } : {}),
      },
      avatarUrl,
      model: {
        builtin: { LEVELS: {} },
        system: { get: (key: string) => key === 'server.login' },
        setting: { SETTINGS_BY_KEY: {} },
        contest: contestModel,
      },
      lib: { difficulty: () => 0 },
      utils: { getAlphabeticId: (index: number) => String.fromCharCode(65 + index) },
      UserContext: user,
    }
  }, [contestId, languages, preferredLanguage, preferredTemplate, problem, user, users])

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
  if (error && !problem) {
    return <div className="row"><div className="columns"><blockquote className="warn"><p>{error}</p></blockquote></div></div>
  }
  if (!problem || !context) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>
  }

  return (
    <>
      {error ? <div className="row"><div className="columns"><blockquote className="warn"><p>{error}</p></blockquote></div></div> : null}
      {submitting ? <div className="row"><div className="columns"><blockquote className="note"><p>提交中...</p></blockquote></div></div> : null}
      <HydroTemplate
        template="problem_submit.html"
        context={context}
        url={(name, args = {}) => {
          const id = encodeURIComponent(String(args.pid ?? problem.pid ?? problem.docId))
          const query = (args.query ?? {}) as Record<string, unknown>
          const activity = activityRoute(name, contestId, args)
          if (activity !== undefined) return activity
          const routes: Record<string, string> = {
            problem_main: query.q ? `/p?q=${encodeURIComponent(String(query.q))}` : '/p',
            problem_detail: `/p/${id}${contestId ? `?tid=${encodeURIComponent(contestId)}` : ''}`,
            problem_submit: `/p/${id}/submit${contestId ? `?tid=${encodeURIComponent(contestId)}` : ''}`,
            problem_files: `/p/${id}/files`,
            problem_solution: `/p/${id}/solution`,
            problem_config: `/p/${id}/config`,
            problem_edit: `/p/${id}/edit`,
            problem_statistics: `/p/${id}/statistics`,
            user_detail: `/user/${encodeURIComponent(String(args.uid ?? ''))}`,
          }
          return routes[name] ?? '/'
        }}
        onReady={handleReady}
      />
    </>
  )
}
