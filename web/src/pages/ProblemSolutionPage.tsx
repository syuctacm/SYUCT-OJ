// Page markup is rendered from the pinned Hydro ui-default template
// (problem_solution.html + components/comments_solution.html). Keep this file
// limited to data mapping and the vote/comment interaction islands.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useLocation, useNavigate, useParams } from 'react-router-dom'
import {
  createProblemSolution, createSolutionReply, deleteProblemSolution, deleteSolutionReply,
  getProblem, getProblemSolutions, updateProblemSolution, updateSolutionReply, voteProblemSolution,
  type ProblemDetail, type ProblemSolutionPage,
} from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { confirmDialog } from '../components/dialogs.tsx'
import { hydrateHydroEditForm } from '../upstream/hydrate.tsx'
import { mountClipboard } from '../upstream/clipboard.ts'
import { avatarUrl } from '../upstream/avatar.ts'

// Local permission ids; the upstream template only compares them through
// handler.user.hasPerm, so the values stay internal to this page.
const PERMS = {
  PERM_EDIT_PROBLEM: 1,
  PERM_EDIT_PROBLEM_SELF: 2,
  PERM_SUBMIT_PROBLEM: 3,
  PERM_READ_PROBLEM_DATA: 4,
  PERM_VIEW_DISCUSSION: 5,
  PERM_REJUDGE_PROBLEM: 7,
  PERM_VIEW_PROBLEM_SOLUTION: 8,
  PERM_CREATE_PROBLEM_SOLUTION: 20,
  PERM_EDIT_PROBLEM_SOLUTION_SELF: 21,
  PERM_DELETE_PROBLEM_SOLUTION_SELF: 22,
  PERM_REPLY_PROBLEM_SOLUTION: 23,
  PERM_EDIT_PROBLEM_SOLUTION_REPLY_SELF: 24,
  PERM_DELETE_PROBLEM_SOLUTION_REPLY_SELF: 25,
  PERM_VOTE_PROBLEM_SOLUTION: 26,
  PERM_EDIT_PROBLEM_SOLUTION: 27,
  PERM_DELETE_PROBLEM_SOLUTION: 28,
  PERM_DELETE_PROBLEM_SOLUTION_REPLY: 29,
  PERM_VIEW_PROBLEM_SOLUTION_ACCEPT: 30,
  PERM_VIEW_USER_PRIVATE_INFO: 31,
} as const

const PAGE_SIZE = 20

function configSummary(problem: ProblemDetail) {
  return {
    type: problem.problemType || 'default',
    timeMin: String(problem.timeLimit || '1000').replace(/ms$/i, ''),
    timeMax: String(problem.timeLimit || '1000').replace(/ms$/i, ''),
    memoryMin: String(problem.memoryLimit || '256').replace(/[a-z]+$/i, ''),
    memoryMax: String(problem.memoryLimit || '256').replace(/[a-z]+$/i, ''),
  }
}

export function ProblemSolutionPage() {
  const { pid = '', sid = '' } = useParams()
  const location = useLocation()
  const navigate = useNavigate()
  const { user } = useAuth()
  const [problem, setProblem] = useState<ProblemDetail | null>(null)
  const [data, setData] = useState<ProblemSolutionPage | null>(null)
  const [error, setError] = useState('')
  const [actionError, setActionError] = useState('')
  const submitRef = useRef<(event: SubmitEvent) => void>(() => {})
  const clickRef = useRef<(event: Event) => void>(() => {})

  const page = Math.max(1, Number(new URLSearchParams(location.search).get('page')) || 1)

  const load = useCallback(async () => {
    const [pdoc, list] = await Promise.all([
      getProblem(pid),
      getProblemSolutions(pid, page, sid),
    ])
    setProblem(pdoc)
    setData(list)
  }, [page, pid, sid])

  useEffect(() => {
    setError('')
    load().catch((err: unknown) => setError(err instanceof Error ? err.message : '题解加载失败'))
  }, [load])

  const run = useCallback(async (action: () => Promise<void>) => {
    setActionError('')
    try {
      await action()
    } catch (err: unknown) {
      setActionError(err instanceof Error ? err.message : '操作失败')
    }
  }, [])

  const context = useMemo(() => {
    if (!problem || !data) return null
    const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
    const isOwner = Boolean(user && problem.owner === user.id)
    const pdoc = {
      ...problem,
      tag: problem.tags ?? [],
      config: configSummary(problem),
      data: [{ name: '__testdata__', size: 0 }],
      reference: false,
    }
    const psdocs = data.items.map((item) => ({
      ...item,
      _id: item.id,
      docId: item.id,
      parentId: item.pid,
      reply: (item.replies ?? []).map((reply) => ({ ...reply, _id: reply.id })),
    }))
    const udict = Object.fromEntries(data.users.map((item) => [String(item.id), {
      _id: item.id,
      uname: item.username,
      displayName: item.displayName ?? '',
      avatar: item.avatar ?? '',
      hasPriv: () => false,
      hasPerm: () => false,
    }]))
    const pssdict = Object.fromEntries(data.items.map((item) => [
      item.id,
      { vote: data.votes[item.id] ?? 0 },
    ]))
    return {
      page_name: 'problem_solution',
      pdoc,
      psdocs,
      udict,
      pssdict,
      pscount: data.total,
      pcount: Math.max(1, Math.ceil(data.total / PAGE_SIZE)),
      page,
      sid: sid || null,
      tdoc: null,
      handler: {
        user: {
          _id: user?.id ?? 0,
          uname: user?.username ?? '',
          avatar: user?.avatar ?? '',
          viewLang: 'zh',
          _dudoc: { join: true },
          hasPerm: (permission: number) => {
            if (permission === PERMS.PERM_VIEW_PROBLEM_SOLUTION) return true
            if (permission === PERMS.PERM_VIEW_USER_PRIVATE_INFO) return Boolean(user)
            if (permission === PERMS.PERM_VIEW_DISCUSSION) return false
            if (permission === PERMS.PERM_EDIT_PROBLEM) return isAdmin
            if (permission === PERMS.PERM_REJUDGE_PROBLEM) return isAdmin
            if (permission === PERMS.PERM_EDIT_PROBLEM_SOLUTION
              || permission === PERMS.PERM_DELETE_PROBLEM_SOLUTION
              || permission === PERMS.PERM_DELETE_PROBLEM_SOLUTION_REPLY
              || permission === PERMS.PERM_VIEW_PROBLEM_SOLUTION_ACCEPT) return isAdmin
            if (permission >= PERMS.PERM_CREATE_PROBLEM_SOLUTION
              && permission <= PERMS.PERM_VOTE_PROBLEM_SOLUTION) return Boolean(user)
            const loggedIn = [PERMS.PERM_EDIT_PROBLEM_SELF, PERMS.PERM_SUBMIT_PROBLEM, PERMS.PERM_READ_PROBLEM_DATA] as number[]
            return Boolean(user) && loggedIn.includes(permission)
          },
          hasPriv: (privilege: number) => privilege === 1 && Boolean(user),
          own: (doc?: unknown) => Boolean(user && doc && (doc as { owner?: number }).owner === user.id),
        },
        args: { domainId: 'system' },
        request: { query: { page }, headers: { 'user-agent': navigator.userAgent } },
        session: { legacy: false, viewLang: 'zh' },
        ctx: { setting: { get: () => '' }, i18n: { get: () => '' } },
      },
      perm: PERMS,
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
      isOwner,
      owner_udoc: udict[String(problem.owner ?? 0)] ?? null,
    }
  }, [data, page, problem, sid, user])

  const buildUrl = useCallback((name: string, args: Record<string, unknown> = {}) => {
    const id = encodeURIComponent(String(args.pid ?? pid))
    const query = (args.query ?? {}) as Record<string, unknown>
    const routes: Record<string, string> = {
      problem_main: query.q ? `/p?q=${encodeURIComponent(String(query.q))}` : '/p',
      problem_detail: `/p/${id}`,
      problem_submit: `/p/${id}/submit`,
      problem_files: `/p/${id}/files`,
      problem_config: `/p/${id}/config`,
      problem_edit: `/p/${id}/edit`,
      problem_statistics: `/p/${id}/statistics`,
      problem_solution: `/p/${id}/solution`,
      problem_solution_detail: `/p/${id}/solution/${encodeURIComponent(String(args.sid ?? ''))}`,
      problem_solution_raw: '#',
      problem_solution_reply_raw: '#',
      record_main: `/record${query.pid ? `?pid=${encodeURIComponent(String(query.pid))}` : ''}`,
      record_detail: `/record/${encodeURIComponent(String(args.rid ?? ''))}`,
      user_detail: `/user/${encodeURIComponent(String(args.uid ?? ''))}`,
      discussion_node: '/discuss',
      discussion_detail: '/discuss',
    }
    return routes[name] ?? '/'
  }, [pid])

  function findSolution(id: string) {
    return data?.items.find((item) => item.id === id) ?? null
  }

  function findReply(id: string, rid: string) {
    return findSolution(id)?.replies?.find((reply) => reply.id === rid) ?? null
  }

  // Inline boxes follow the markup the upstream commentbox script builds.
  function buildInlineForm(kind: 'reply' | 'edit_solution' | 'edit_reply', id: string, rid: string, initial = '') {
    const form = document.createElement('form')
    form.className = 'dczcomments__box'
    form.method = 'post'
    form.innerHTML = `
      <div><textarea class="textbox" name="content" required></textarea></div>
      <div>
        <input type="submit" class="rounded primary button" value="${kind === 'reply' ? '回复 (Ctrl+Enter)' : '更新 (Ctrl+Enter)'}">
        <input type="button" class="rounded button" name="cancel" value="取消 (Esc)">
      </div>
      <input type="hidden" name="operation" value="${kind}">
      <input type="hidden" name="psid" value="${id}">
      ${rid ? `<input type="hidden" name="psrid" value="${rid}">` : ''}
    `
    const textarea = form.querySelector('textarea') as HTMLTextAreaElement
    textarea.value = initial
    form.addEventListener('submit', (event) => {
      event.preventDefault()
      const content = textarea.value.trim()
      if (!content) return
      void run(async () => {
        if (kind === 'reply') await createSolutionReply(pid, id, content)
        else if (kind === 'edit_solution') await updateProblemSolution(pid, id, content)
        else await updateSolutionReply(pid, id, rid, content)
        await load()
      })
    })
    form.querySelector<HTMLInputElement>('input[name="cancel"]')?.addEventListener('click', () => {
      form.remove()
    })
    return form
  }

  async function handleClick(event: Event) {
    const target = (event.target as HTMLElement).closest<HTMLElement>('[data-op]')
    if (!target) return
    event.preventDefault()
    const operation = target.dataset.op
    const form = target.dataset.form ? JSON.parse(target.dataset.form) as Record<string, string> : {}
    const id = String(form.psid ?? '')
    const rid = String(form.psrid ?? '')
    if (operation === 'reply') {
      const holder = target.closest('.dczcomments__item')?.querySelector('.commentbox-reply-target')
      if (!holder) return
      const existing = holder.querySelector('form.dczcomments__box')
      if (existing) {
        existing.remove()
        return
      }
      const box = document.createElement('li')
      box.className = 'dczcomments__reply'
      box.appendChild(buildInlineForm('reply', id, ''))
      holder.appendChild(box)
    } else if (operation === 'edit') {
      const mediaBody = target.closest('.media__body')
      const content = mediaBody?.querySelector('.typo.richmedia')
      if (!mediaBody || !content) return
      if (mediaBody.querySelector('form.dczcomments__box')) {
        mediaBody.querySelector('form.dczcomments__box')?.remove()
        return
      }
      const initial = rid ? findReply(id, rid)?.content ?? '' : findSolution(id)?.content ?? ''
      const kind = rid ? 'edit_reply' : 'edit_solution'
      mediaBody.insertBefore(buildInlineForm(kind, id, rid, initial), content)
    } else if (operation === 'delete') {
      if (!(await confirmDialog(rid ? '确定删除这条回复吗？' : '确定删除这篇题解吗？'))) return
      void run(async () => {
        if (rid) await deleteSolutionReply(pid, id, rid)
        else await deleteProblemSolution(pid, id)
        if (!rid && sid) navigate(`/p/${encodeURIComponent(pid)}/solution`)
        else await load()
      })
    }
  }
  clickRef.current = handleClick

  async function handleSubmit(event: SubmitEvent) {
    const form = event.target as HTMLFormElement
    if (!(form instanceof HTMLFormElement)) return
    if (form.classList.contains('vote-op')) {
      event.preventDefault()
      const id = form.querySelector<HTMLInputElement>('input[name="psid"]')?.value ?? ''
      const operation = (event.submitter as HTMLButtonElement | null)?.value
      const value = operation === 'upvote' ? 1 : operation === 'downvote' ? -1 : 0
      if (!id || !value) return
      await run(async () => {
        await voteProblemSolution(pid, id, value)
        await load()
      })
      return
    }
    const operation = form.querySelector<HTMLInputElement>('input[name="operation"]')?.value
    if (operation !== 'submit') return
    event.preventDefault()
    const content = (form.querySelector<HTMLTextAreaElement>('textarea[name="content"]')?.value ?? '').trim()
    if (!content) return
    await run(async () => {
      const created = await createProblemSolution(pid, content)
      navigate(`/p/${encodeURIComponent(pid)}/solution/${created.id}`)
    })
  }
  submitRef.current = (event) => void handleSubmit(event)

  const handleReady = useCallback((root: HTMLDivElement) => {
    // The pinned template ships a no-JS fallback; this app always runs JavaScript.
    root.querySelectorAll('div.hasjs--hide').forEach((element) => element.classList.remove('hasjs--hide'))
    root.querySelectorAll('.dczcomments__dummy-box').forEach((element) => element.closest('.nojs--hide')?.remove())
    // The commentbox script hides the cancel button until a box is in edit mode.
    root.querySelectorAll('form.dczcomments__box').forEach((form) => {
      if (form.querySelector<HTMLInputElement>('input[name="operation"]')?.value === 'submit') {
        form.querySelector('input[name="dczcomments__box__cancel"]')?.remove()
      }
    })

    const cleanups: Array<() => void> = [hydrateHydroEditForm(root), mountClipboard(root)]
    const onSubmit = (event: SubmitEvent) => submitRef.current(event)
    root.addEventListener('submit', onSubmit)
    cleanups.push(() => root.removeEventListener('submit', onSubmit))
    const onClick = (event: Event) => clickRef.current(event)
    root.addEventListener('click', onClick)
    cleanups.push(() => root.removeEventListener('click', onClick))
    return () => cleanups.forEach((cleanup) => cleanup())
  }, [])

  if (error) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  }
  if (!problem || !data || !context) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>
  }
  return (
    <>
      {actionError ? <div className="row"><div className="columns"><blockquote className="warn"><p>{actionError}</p></blockquote></div></div> : null}
      <HydroTemplate
        template="problem_solution.html"
        context={context}
        url={buildUrl}
        onReady={handleReady}
      />
    </>
  )
}
