// Page markup is rendered from the pinned Hydro ui-default template. Keep this
// file limited to data mapping, request handling and small interactions.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useParams } from 'react-router-dom'
import {
  enrollTraining, getTraining, getUsersByIDs, type TrainingDetail, type UserChoice,
} from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'

export function TrainingDetailPage() {
  const { tid = '' } = useParams()
  const { user } = useAuth()
  const [training, setTraining] = useState<TrainingDetail | null>(null)
  const [users, setUsers] = useState<UserChoice[]>([])
  const [error, setError] = useState('')
  const [enrollError, setEnrollError] = useState('')
  const [enrolling, setEnrolling] = useState(false)
  const submitRef = useRef<(event: SubmitEvent) => void>(() => {})

  useEffect(() => {
    setTraining(null)
    setError('')
    getTraining(tid)
      .then(setTraining)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '训练加载失败'))
  }, [tid])

  useEffect(() => {
    if (!training) return
    const ids = [...new Set([training.owner, ...(training.enrolledUsers ?? [])])].filter((id) => id > 0)
    getUsersByIDs(ids)
      .then(setUsers)
      .catch(() => setUsers([]))
  }, [training])

  const context = useMemo(() => {
    if (!training) return {}
    const canEdit = Boolean(user && (user.id === training.owner || ((user.priv ?? 0) & 1) !== 0))
    const problems = training.sections.flatMap((section) => section.problems)
    const pids = problems.map((problem) => problem.docId)
    const donePids = problems.filter((problem) => problem.solved).map((problem) => problem.docId)
    const pdict = Object.fromEntries(problems.map((problem) => [String(problem.docId), {
      docId: problem.docId,
      pid: problem.pid,
      title: problem.title,
      difficulty: problem.difficulty,
      nSubmit: problem.nSubmit,
      nAccept: problem.nAccept,
      hidden: false,
    }]))
    const psdict = Object.fromEntries(problems
      .filter((problem) => problem.rid)
      .map((problem) => [String(problem.docId), {
        rid: problem.rid,
        status: problem.status ?? 0,
        score: problem.score ?? 0,
        domainId: 'system',
      }]))
    const nsdict = Object.fromEntries(training.sections.map((section) => [String(section.id), {
      isDone: section.status === 'done',
      isProgress: section.status === 'progress',
      isOpen: section.status === 'open',
      isInvalid: section.status === 'invalid',
      progress: section.progress,
    }]))
    const ndict = Object.fromEntries(training.sections.map((section) => [
      String(section.id),
      { title: section.title },
    ]))
    const toUdoc = (item: UserChoice) => ({
      _id: item.id,
      uname: item.username,
      displayName: item.displayName ?? '',
      avatar: item.avatar ?? '',
    })
    const enrolled = new Set(training.enrolledUsers ?? [])
    const udict = Object.fromEntries(users
      .filter((item) => enrolled.has(item.id))
      .map((item) => [String(item.id), toUdoc(item)]))
    const owner = users.find((item) => item.id === training.owner)
    const dag = training.sections.map((section) => ({
      _id: section.id,
      title: section.title,
      content: section.content ?? '',
      requireNids: section.requireNids,
      pids: section.problems.map((problem) => problem.docId),
    }))
    return {
      page_name: 'training_detail',
      tdoc: { ...training, _id: training.id, dag },
      tsdoc: { enroll: training.enrolled, done: training.completed, donePids },
      udict,
      udoc: owner ? toUdoc(owner) : null,
      pids,
      pdict,
      psdict,
      selfPsdict: {},
      nsdict,
      ndict,
      missing: problems.filter((problem) => problem.missing).map((problem) => problem.docId),
      groups: [],
      handler: {
        user: {
          _id: user?.id ?? 0,
          viewLang: 'zh',
          _dudoc: { join: true },
          hasPerm: (permission: number) => (
            (permission === 1 && canEdit) || (permission === 2 && Boolean(user))
          ),
          hasPriv: (privilege: number) => privilege === 1 && Boolean(user),
          own: (doc?: { owner?: number }) => Boolean(user && doc && doc.owner === user.id),
        },
        args: { domainId: 'system' },
        request: {
          query: {},
          path: `/training/${training.docId}`,
          headers: { 'user-agent': navigator.userAgent },
        },
        session: { legacy: false, viewLang: 'zh' },
        ctx: { setting: { get: () => '' }, i18n: { get: () => '' } },
      },
      perm: { PERM_EDIT_TRAINING: 1, PERM_VIEW_USER_PRIVATE_INFO: 2 },
      PRIV: { PRIV_USER_PROFILE: 1 },
      avatarUrl: (value?: string) => value || '/logo.png',
      UiContext: {},
      UserContext: user,
    }
  }, [training, user, users])

  const buildUrl = useCallback((name: string, args: Record<string, unknown> = {}) => {
    const id = String(args.tid ?? training?.docId ?? tid)
    switch (name) {
      case 'training_detail': return `/training/${encodeURIComponent(id)}`
      case 'training_edit': return `/training/${encodeURIComponent(id)}/edit`
      case 'training_files': return `/training/${encodeURIComponent(id)}/file`
      case 'problem_detail': return `/p/${encodeURIComponent(String(args.pid ?? ''))}`
      case 'record_detail': return `/record/${encodeURIComponent(String(args.rid ?? ''))}`
      case 'user_detail': return `/user/${encodeURIComponent(String(args.uid ?? ''))}`
      case 'wiki_help': return '/discuss'
      default: return '/'
    }
  }, [tid, training?.docId])

  const handleReady = useCallback((root: HTMLDivElement) => {
    const cleanups: Array<() => void> = []
    const onClick = (event: Event) => {
      const target = event.target as HTMLElement
      const action = target.closest('[name="training__section__expand"]') ? 'expanded'
        : target.closest('[name="training__section__collapse"]') ? 'collapsed'
          : null
      if (!action) return
      const section = target.closest('.training__section')
      if (!section) return
      event.preventDefault()
      section.classList.toggle('expanded', action === 'expanded')
      section.classList.toggle('collapsed', action === 'collapsed')
    }
    root.addEventListener('click', onClick)
    cleanups.push(() => root.removeEventListener('click', onClick))

    const form = root.querySelector<HTMLInputElement>('input[name="operation"][value="enroll"]')?.closest('form')
    if (form) {
      const onSubmit = (event: SubmitEvent) => submitRef.current(event)
      form.addEventListener('submit', onSubmit)
      cleanups.push(() => form.removeEventListener('submit', onSubmit))
    }

    const search = root.querySelector<HTMLInputElement>('input[name="uid"]')
    if (search) {
      const onInput = () => {
        const value = search.value.trim().toLowerCase()
        for (const item of root.querySelectorAll<HTMLElement>('.enroll_user_menu_item')) {
          const name = (item.dataset.uname ?? '').toLowerCase()
          const displayName = (item.dataset.displayname ?? '').toLowerCase()
          const matches = !value || name.includes(value) || displayName.includes(value) || item.dataset.uid === value
          item.style.display = matches ? '' : 'none'
        }
      }
      search.addEventListener('input', onInput)
      cleanups.push(() => search.removeEventListener('input', onInput))
    }

    return () => cleanups.forEach((cleanup) => cleanup())
  }, [])

  async function handleEnroll(event: SubmitEvent) {
    event.preventDefault()
    setEnrolling(true)
    setEnrollError('')
    try {
      await enrollTraining(tid)
      setTraining(await getTraining(tid))
    } catch (err) {
      setEnrollError(err instanceof Error ? err.message : '报名失败')
    } finally {
      setEnrolling(false)
    }
  }
  submitRef.current = (event) => void handleEnroll(event)

  if (error) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  }
  if (!training) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>
  }
  return (
    <>
      {enrollError ? <div className="row"><div className="columns"><blockquote className="warn"><p>{enrollError}</p></blockquote></div></div> : null}
      {enrolling ? <div className="row"><div className="columns"><blockquote className="note"><p>报名中...</p></blockquote></div></div> : null}
      <HydroTemplate
        template="training_detail.html"
        context={context}
        url={buildUrl}
        onReady={handleReady}
      />
    </>
  )
}
