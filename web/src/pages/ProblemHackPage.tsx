// Page markup is rendered from the pinned Hydro ui-default template. React
// supplies the target record, permissions and multipart submit adapter.
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { createHack, getProblem, getSubmission, getUsersByIDs, type ProblemDetail, type Submission } from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { languageOptions } from '../components/autocomplete/LanguageSelectAutoComplete.tsx'
import { avatarUrl } from '../upstream/avatar.ts'
import { ErrorPage } from './ErrorPage.tsx'

const PROBLEM_PERMS = {
  PERM_EDIT_PROBLEM: 1,
  PERM_EDIT_PROBLEM_SELF: 2,
  PERM_SUBMIT_PROBLEM: 3,
  PERM_READ_PROBLEM_DATA: 4,
  PERM_VIEW_DISCUSSION: 5,
  PERM_CREATE_DISCUSSION: 6,
  PERM_REJUDGE_PROBLEM: 7,
  PERM_VIEW_PROBLEM_SOLUTION: 8,
  PERM_VIEW_PROBLEM_SOLUTION_ACCEPT: 9,
} as const

const languageMap = Object.fromEntries(languageOptions.map((language) => [
  language._id,
  { display: language.name, highlight: language._id },
]))

function configSummary(problem: ProblemDetail) {
  return {
    type: problem.problemType || 'default',
    hackable: Boolean(problem.hackable),
    timeMin: String(problem.timeLimit || '1000').replace(/ms$/i, ''),
    timeMax: String(problem.timeLimit || '1000').replace(/ms$/i, ''),
    memoryMin: String(problem.memoryLimit || '256').replace(/[a-z]+$/i, ''),
    memoryMax: String(problem.memoryLimit || '256').replace(/[a-z]+$/i, ''),
  }
}

export function ProblemHackPage() {
  const { pid = '', rid = '' } = useParams()
  const navigate = useNavigate()
  const { user } = useAuth()
  const [problem, setProblem] = useState<ProblemDetail | null>(null)
  const [submission, setSubmission] = useState<Submission | null>(null)
  const [users, setUsers] = useState<Array<{ id: number; username: string; displayName?: string; avatar?: string }>>([])
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const submitRef = useCallback(async (form: HTMLFormElement) => {
    if (submitting) return
    setSubmitting(true)
    setError('')
    try {
      const record = await createHack(pid, rid, new FormData(form))
      navigate(`/record/${encodeURIComponent(record.id)}`)
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Hack 提交失败')
    } finally {
      setSubmitting(false)
    }
  }, [navigate, pid, rid, submitting])

  useEffect(() => {
    setError('')
    Promise.all([getProblem(pid), getSubmission(rid)])
      .then(([nextProblem, nextSubmission]) => {
        setProblem(nextProblem)
        setSubmission(nextSubmission)
      })
      .catch((reason: unknown) => setError(reason instanceof Error ? reason.message : 'Hack 页面加载失败'))
  }, [pid, rid])

  useEffect(() => {
    if (!problem) {
      setUsers([])
      return
    }
    getUsersByIDs([problem.owner, submission?.submitter?.id].filter((id): id is number => Boolean(id)))
      .then(setUsers)
      .catch(() => setUsers([]))
  }, [problem, submission?.submitter?.id])

  const handleReady = useCallback((root: HTMLDivElement) => {
    const form = root.querySelector<HTMLFormElement>('form')
    if (!form) return () => {}
    const onSubmit = (event: SubmitEvent) => {
      event.preventDefault()
      void submitRef(form)
    }
    form.addEventListener('submit', onSubmit)
    return () => form.removeEventListener('submit', onSubmit)
  }, [submitRef])

  const context = useMemo(() => {
    if (!problem || !submission) return null
    const userByID = new Map(users.map((item) => [item.id, item]))
    const submitter = userByID.get(submission.submitter?.id ?? submission.uid)
    const owner = userByID.get(problem.owner ?? 0)
    const pdoc = {
      _id: problem.id,
      docId: problem.docId,
      docType: 10,
      domainId: 'system',
      pid: problem.pid,
      title: problem.title,
      hidden: Boolean(problem.hidden),
      owner: problem.owner ?? 0,
      tag: problem.tags ?? [],
      difficulty: problem.difficulty ?? 0,
      nSubmit: problem.nSubmit,
      nAccept: problem.nAccept,
      config: configSummary(problem),
    }
    const udoc = submitter ? {
      _id: submitter.id,
      uname: submitter.displayName || submitter.username,
      avatar: submitter.avatar ?? '',
      hasPriv: () => false,
      hasPerm: () => false,
    } : null
    const ownerUdoc = owner ? {
      _id: owner.id,
      uname: owner.displayName || owner.username,
      avatar: owner.avatar ?? '',
      hasPriv: () => false,
      hasPerm: () => false,
    } : udoc
    return {
      page_name: 'problem_hack',
      pdoc,
      tdoc: null,
      tsdoc: null,
      psdoc: { status: 0 },
      udoc,
      owner_udoc: ownerUdoc,
      rid: submission.id,
      handler: {
        user: {
          _id: user?.id ?? 0,
          viewLang: 'zh',
          _dudoc: { join: true },
          hasPerm: (permission: number) => Boolean(user) && permission === PROBLEM_PERMS.PERM_SUBMIT_PROBLEM,
          hasPriv: () => Boolean(user),
          own: () => false,
        },
        args: { domainId: 'system' },
        request: { query: {}, path: `/p/${pid}/hack/${rid}`, headers: { 'user-agent': navigator.userAgent } },
        session: { legacy: false, viewLang: 'zh' },
        ctx: { setting: { get: () => '' }, i18n: { get: () => '' } },
      },
      perm: PROBLEM_PERMS,
      PRIV: { PRIV_USER_PROFILE: 1, PRIV_READ_PROBLEM_DATA: 2 },
      STATUS: { STATUS_ACCEPTED: 1 },
      model: { setting: { langs: languageMap }, system: { get: () => false } },
      lib: { difficulty: () => 0 },
      avatarUrl,
      UiContext: {},
      UserContext: user,
    }
  }, [pid, problem, rid, submission, user, users])

  if (!user) return <ErrorPage status={403} />
  if (error && !problem) return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  if (!problem || !submission || !context) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>

  return (
    <>
      {error ? <div className="row"><div className="columns"><blockquote className="warn"><p>{error}</p></blockquote></div></div> : null}
      {submitting ? <div className="row"><div className="columns"><blockquote className="note"><p>正在提交 Hack...</p></blockquote></div></div> : null}
      <HydroTemplate
        template="problem_hack.html"
        context={context}
        onReady={handleReady}
        url={(name, args = {}) => {
          const problemID = String(args.pid ?? problem.pid ?? problem.docId)
          switch (name) {
            case 'problem_main': return '/p'
            case 'problem_detail': return `/p/${encodeURIComponent(problemID)}`
            case 'problem_submit': return `/p/${encodeURIComponent(problemID)}/submit`
            case 'problem_files': return `/p/${encodeURIComponent(problemID)}/files`
            case 'problem_statistics': return `/p/${encodeURIComponent(problemID)}/statistics`
            case 'problem_solution': return `/p/${encodeURIComponent(problemID)}/solution`
            case 'problem_edit': return `/p/${encodeURIComponent(problemID)}/edit`
            case 'problem_config': return `/p/${encodeURIComponent(problemID)}/config`
            case 'problem_tag_stat': return `/p/tag/${encodeURIComponent(String(args.tag ?? ''))}`
            case 'record_main': return `/record?pid=${encodeURIComponent(String(args.pid ?? problem.docId))}`
            case 'discussion_node': {
              const type = String(args.type ?? 'problem')
              const name = String(args.name ?? problem.docId)
              return `/discuss/${encodeURIComponent(type)}/${encodeURIComponent(name)}`
            }
            default: return '/'
          }
        }}
      />
    </>
  )
}
