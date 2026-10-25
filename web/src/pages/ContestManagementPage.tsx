// Page markup is rendered from the pinned Hydro ui-default template
// (contest_manage.html). Keep this file limited to data mapping and the file
// interactions that template expects; do not re-create the DOM in JSX.
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useParams } from 'react-router-dom'
import {
  deleteActivityFiles, getActivityFiles, getContest, getContestProblems, getUsersByIDs,
  setContestProblemScore, uploadActivityFiles,
  type ActivityFile, type ActivityFileType, type Contest, type ProblemSummary, type UserChoice,
} from '../api.ts'
import { useAuth } from '../auth.tsx'
import { Dialog } from '../components/Dialog.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { confirmDialog } from '../components/dialogs.tsx'
import { CONTEST_PERMS, createContestModel } from '../upstream/contest.ts'
import { avatarUrl } from '../upstream/avatar.ts'

export function ContestManagementPage() {
  const { tid = '' } = useParams()
  const { user } = useAuth()
  const [contest, setContest] = useState<Contest | null>(null)
  const [problems, setProblems] = useState<ProblemSummary[]>([])
  const [users, setUsers] = useState<UserChoice[]>([])
  const [files, setFiles] = useState<ActivityFile[]>([])
  const [privateFiles, setPrivateFiles] = useState<ActivityFile[]>([])
  const [error, setError] = useState('')
  const [actionError, setActionError] = useState('')
  const [scoring, setScoring] = useState<{ pid: number } | null>(null)
  const [scoreValue, setScoreValue] = useState('100')

  const load = useCallback(async () => {
    const [detail, problemPage] = await Promise.all([
      getContest(tid),
      getContestProblems(tid),
    ])
    setProblems(problemPage.problems)
    setContest(detail)
    setUsers(await getUsersByIDs([detail.owner]).catch(() => []))
    const isManager = Boolean(user && (user.id === detail.owner || ((user.priv ?? 0) & 1) !== 0))
    if (isManager) {
      const [publicList, privateList] = await Promise.all([
        getActivityFiles('contests', detail.docId, 'public'),
        getActivityFiles('contests', detail.docId, 'private'),
      ])
      setFiles(publicList)
      setPrivateFiles(privateList)
    } else {
      setFiles([])
      setPrivateFiles([])
    }
  }, [tid, user])

  useEffect(() => {
    setContest(null)
    setError('')
    load().catch((err: unknown) => setError(err instanceof Error ? err.message : '比赛管理加载失败'))
  }, [load])

  const tdoc = useMemo(() => {
    if (!contest) return null
    return {
      ...contest,
      _id: contest.docId,
      beginAt: new Date(contest.beginAt),
      endAt: new Date(contest.endAt),
      lockAt: contest.lockAt ? new Date(contest.lockAt) : null,
      penaltySince: contest.penaltySince ? new Date(contest.penaltySince) : null,
      pids: contest.pids ?? [],
      domainId: 'system',
    }
  }, [contest])

  const isManager = Boolean(user && contest && (
    user.id === contest.owner
    || ((user.priv ?? 0) & 1) !== 0
    || (contest.maintainer ?? []).includes(user.id)
  ))

  const context = useMemo(() => {
    if (!contest || !tdoc) return {}
    const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
    const handler = {
      user: {
        _id: user?.id ?? 0,
        viewLang: 'zh',
        _dudoc: { join: true },
        hasPerm: (permission: number) => {
          if (permission === CONTEST_PERMS.PERM_VIEW_USER_PRIVATE_INFO) return Boolean(user)
          if (permission === CONTEST_PERMS.PERM_VIEW_CONTEST_HIDDEN_SCOREBOARD) return isAdmin
          if (permission === CONTEST_PERMS.PERM_EDIT_CONTEST) return isManager
          if (permission === CONTEST_PERMS.PERM_READ_RECORD_CODE) return isManager
          return false
        },
        hasPriv: (privilege: number) => privilege === 1 && Boolean(user),
        own: (doc?: unknown) => Boolean(user && doc && (doc as { owner?: number }).owner === user.id),
      },
      args: { domainId: 'system' },
      request: {
        query: {},
        path: `/contest/${contest.docId}/management`,
        headers: { 'user-agent': navigator.userAgent },
      },
      session: { legacy: false, viewLang: 'zh' },
      ctx: { setting: { get: () => '' }, i18n: { get: () => '' } },
    }
    const pdict = Object.fromEntries(problems.map((problem) => [String(problem.docId), {
      docId: problem.docId,
      pid: problem.pid,
      title: problem.title,
      hidden: false,
      nSubmit: problem.nSubmit,
      nAccept: problem.nAccept,
      difficulty: problem.difficulty,
    }]))
    const owner = users.find((item) => item.id === contest.owner)
    return {
      page_name: 'contest_manage',
      tdoc,
      tsdoc: { attend: Boolean(contest.attended) },
      pdict,
      files: files.map((file) => ({ name: file.name, size: file.size })),
      privateFiles: privateFiles.map((file) => ({ name: file.name, size: file.size })),
      owner_udoc: owner ? {
        _id: owner.id,
        uname: owner.username,
        displayName: owner.displayName ?? '',
        avatar: owner.avatar ?? '',
      } : null,
      handler,
      perm: CONTEST_PERMS,
      PRIV: { PRIV_USER_PROFILE: 1, PRIV_MOD_BADGE: 2, PRIV_READ_RECORD_CODE: 3 },
      model: { contest: createContestModel(handler) },
      avatarUrl,
      UiContext: {},
      UserContext: user,
    }
  }, [contest, files, isManager, privateFiles, problems, tdoc, user, users])

  const buildUrl = useCallback((name: string, args: Record<string, unknown> = {}) => {
    const id = String(args.tid ?? contest?.docId ?? tid)
    const query = (args.query ?? {}) as Record<string, unknown>
    switch (name) {
      case 'contest_detail': return `/contest/${encodeURIComponent(id)}`
      case 'discussion_node': return `/discuss/contest/${encodeURIComponent(id)}`
      case 'contest_manage': return `/contest/${encodeURIComponent(id)}/management`
      case 'contest_edit': return `/contest/${encodeURIComponent(id)}/edit`
      case 'contest_clarification': return `/contest/${encodeURIComponent(id)}/clarification`
      case 'problem_detail': {
        const raw = String(args.pid ?? '')
        const problem = problems.find((item) => String(item.docId) === raw || item.pid === raw)
        return `/p/${encodeURIComponent(problem?.pid || raw)}`
      }
      case 'record_main': {
        const params = new URLSearchParams()
        for (const [key, value] of Object.entries(query)) {
          if (value !== undefined && value !== null) params.set(key, String(value))
        }
        return `/record${params.toString() ? `?${params.toString()}` : ''}`
      }
      case 'contest_user': return `/contest/${encodeURIComponent(id)}/user`
      case 'contest_code': return `/api/v1/contests/${encodeURIComponent(id)}/code`
      case 'contest_balloon': return `/contest/${encodeURIComponent(id)}/balloon`
      default: return '/'
    }
  }, [contest?.docId, problems, tid])

  const urlForFile = useCallback((name: string, fileType?: string) => {
    const type = fileType === 'private' ? '?type=private' : ''
    return `/api/v1/contests/${encodeURIComponent(tid)}/files/${encodeURIComponent(name)}${type}`
  }, [tid])

  async function submitScore() {
    if (!scoring) return
    const score = Number.parseInt(scoreValue, 10)
    if (Number.isNaN(score) || score < 0 || score > 10000) {
      setActionError('分值需要是 0-10000 之间的整数。')
      return
    }
    setActionError('')
    try {
      await setContestProblemScore(tid, scoring.pid, score)
      setScoring(null)
      await load()
    } catch (err) {
      setActionError(err instanceof Error ? err.message : '分值更新失败')
    }
  }

  const handleReady = useCallback((root: HTMLDivElement) => {
    const cleanups: Array<() => void> = []
    const listen = <K extends keyof HTMLElementEventMap>(
      element: Element,
      type: K,
      listener: (event: HTMLElementEventMap[K]) => void,
    ) => {
      element.addEventListener(type, listener as EventListener)
      cleanups.push(() => element.removeEventListener(type, listener as EventListener))
    }

    const sectionType = (element: Element): ActivityFileType => (
      element.closest('[data-type="private"]') ? 'private' : 'public'
    )

    root.querySelectorAll<HTMLElement>('[name="set_score"]').forEach((link) => {
      listen(link, 'click', (event) => {
        event.preventDefault()
        const pid = Number(link.getAttribute('data-pid') ?? 0)
        if (!pid) return
        setScoreValue(String(link.getAttribute('data-score') ?? '100'))
        setScoring({ pid })
      })
    })

    root.querySelectorAll<HTMLButtonElement>('[name="upload_file"]').forEach((button) => {
      listen(button, 'click', () => {
        const fileType = sectionType(button)
        const picker = document.createElement('input')
        picker.type = 'file'
        picker.multiple = true
        picker.onchange = async () => {
          const chosen = Array.from(picker.files ?? [])
          if (!chosen.length) return
          setActionError('')
          try {
            const updated = await uploadActivityFiles('contests', tid, chosen, fileType)
            if (fileType === 'private') setPrivateFiles(updated)
            else setFiles(updated)
          } catch (err) {
            setActionError(err instanceof Error ? err.message : '文件上传失败')
          }
        }
        picker.click()
      })
    })

    root.querySelectorAll<HTMLButtonElement>('[name="remove_selected"]').forEach((button) => {
      listen(button, 'click', async () => {
        const fileType = sectionType(button)
        const names = Array.from(root.querySelectorAll<HTMLInputElement>(`input[data-checkbox-group="${fileType}"]:checked`))
          .map((checkbox) => checkbox.closest('tr')?.getAttribute('data-filename') ?? '')
          .filter(Boolean)
        if (!names.length) {
          setActionError('请至少选择一个文件。')
          return
        }
        if (!(await confirmDialog(`确认删除选中的 ${names.length} 个文件？`))) return
        setActionError('')
        try {
          await deleteActivityFiles('contests', tid, names, fileType)
          const updated = await getActivityFiles('contests', tid, fileType)
          if (fileType === 'private') setPrivateFiles(updated)
          else setFiles(updated)
        } catch (err) {
          setActionError(err instanceof Error ? err.message : '文件删除失败')
        }
      })
    })

    root.querySelectorAll<HTMLElement>('.files').forEach((zone) => {
      listen(zone, 'dragover', (event) => event.preventDefault())
      listen(zone, 'drop', (event) => {
        event.preventDefault()
        const dropped = Array.from(event.dataTransfer?.files ?? [])
        if (!dropped.length) return
        const fileType = sectionType(zone)
        setActionError('')
        uploadActivityFiles('contests', tid, dropped, fileType)
          .then((updated) => {
            if (fileType === 'private') setPrivateFiles(updated)
            else setFiles(updated)
          })
          .catch((err: unknown) => setActionError(err instanceof Error ? err.message : '文件上传失败'))
      })
    })

    return () => cleanups.forEach((cleanup) => cleanup())
  }, [tid])

  if (error) return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  if (!contest || !tdoc) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>
  if (!isManager) {
    return <div className="row"><div className="columns"><blockquote className="warn"><p>需要比赛管理权限。</p></blockquote></div></div>
  }
  return (
    <>
      {actionError ? <div className="row"><div className="columns"><blockquote className="warn"><p>{actionError}</p></blockquote></div></div> : null}
      <HydroTemplate
        template="contest_manage.html"
        context={context}
        url={buildUrl}
        urlForFile={urlForFile}
        onReady={handleReady}
      />
      {scoring ? (
        <Dialog
          onOk={() => void submitScore()}
          onCancel={() => setScoring(null)}
          okLabel="保存"
          cancelLabel="取消"
        >
          <h1>设置题目分值</h1>
          <label>
            分值
            <input
              className="textbox"
              type="number"
              min={0}
              max={10000}
              value={scoreValue}
              onChange={(event) => setScoreValue(event.target.value)}
            />
          </label>
        </Dialog>
      ) : null}
    </>
  )
}
