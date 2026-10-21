// Page markup is rendered from the pinned Hydro ui-default template. The section
// list mirrors the upstream `hydrooj.homepage` config for the parts backed by
// this backend (bulletin/ranking/hitokoto/starred have no data source yet).
import { useEffect, useMemo, useState } from 'react'
import {
  getContests, getDiscussionNodes, getDiscussions, getHomeworks, getProblems, getRanking, getTrainings, getUsersByIDs,
  type Contest, type Discussion, type DiscussionNode, type ProblemSummary, type RankingUser, type Training, type UserChoice,
} from '../api.ts'
import { useAuth } from '../auth.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { createContestModel } from '../upstream/contest.ts'
import { avatarUrl } from '../upstream/avatar.ts'

const HOME_PERMS = {
  PERM_VIEW_DISCUSSION: 1,
  PERM_CREATE_DISCUSSION: 2,
  PERM_VIEW_PROBLEM_HIDDEN: 3,
  PERM_EDIT_DISCUSSION: 4,
} as const

type HomeData = {
  contests: Contest[]
  homeworks: Contest[]
  trainings: Training[]
  discussions: Discussion[]
  problems: ProblemSummary[]
  nodes: DiscussionNode[]
  ranking: RankingUser[]
}

function withDates<T extends { beginAt: string; endAt: string; lockAt?: string; penaltySince?: string }>(item: T) {
  return {
    ...item,
    beginAt: new Date(item.beginAt),
    endAt: new Date(item.endAt),
    lockAt: item.lockAt ? new Date(item.lockAt) : null,
    penaltySince: item.penaltySince ? new Date(item.penaltySince) : null,
  }
}

function trainingDag(training: Training) {
  const dag = Array.from({ length: training.sectionCount }, () => ({ pids: [] as number[] }))
  for (let index = 0; index < training.problemCount; index++) {
    dag[index % Math.max(1, dag.length)]?.pids.push(index + 1)
  }
  return dag
}

export function HomePage() {
  const { user } = useAuth()
  const [data, setData] = useState<HomeData | null>(null)
  const [users, setUsers] = useState<UserChoice[]>([])
  const [error, setError] = useState('')

  useEffect(() => {
    setError('')
    Promise.all([
      getContests(),
      getHomeworks(''),
      getTrainings(''),
      getDiscussions(),
      getProblems('', 1, 'recent'),
      getDiscussionNodes(),
      getRanking(1, 10),
    ])
      .then(([contestPage, homeworkPage, trainingPage, discussionPage, problems, nodes, ranking]) => {
        setData({
          contests: contestPage.items,
          homeworks: homeworkPage.items,
          trainings: trainingPage.items,
          discussions: discussionPage.items,
          problems: problems.items,
          nodes,
          ranking: ranking.items,
        })
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '首页加载失败'))
  }, [])

  useEffect(() => {
    if (!data) return
    const ids = [...new Set(data.discussions.map((item) => item.owner))].filter((id) => id > 0)
    if (!ids.length) {
      setUsers([])
      return
    }
    getUsersByIDs(ids).then(setUsers).catch(() => setUsers([]))
  }, [data])

  const context = useMemo(() => {
    if (!data) return {}
    const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
    const handler = {
      user: {
        _id: user?.id ?? 0,
        viewLang: 'zh',
        _dudoc: { join: true },
        hasPerm: (permission: number) => {
          if (permission === HOME_PERMS.PERM_VIEW_DISCUSSION) return true
          if (permission === HOME_PERMS.PERM_CREATE_DISCUSSION) return Boolean(user)
          if (permission === HOME_PERMS.PERM_VIEW_PROBLEM_HIDDEN) return isAdmin
          if (permission === HOME_PERMS.PERM_EDIT_DISCUSSION) return isAdmin
          return false
        },
        hasPriv: (privilege: number) => privilege === 1 && Boolean(user),
        own: (doc?: unknown) => Boolean(user && doc && (doc as { owner?: number }).owner === user.id),
      },
      args: { domainId: 'system' },
      request: { query: {}, path: '/', headers: { 'user-agent': navigator.userAgent } },
      session: { legacy: false, viewLang: 'zh' },
      ctx: { setting: { get: () => '' }, i18n: { get: () => '' } },
    }
    const contestModel = createContestModel(handler)
    const contests = data.contests.slice(0, 5).map((item) => withDates(item))
    const homeworks = data.homeworks.slice(0, 10).map((item) => withDates(item))
    const trainings = data.trainings.slice(0, 10).map((item) => ({ ...item, dag: trainingDag(item) }))
    const discussions = data.discussions.slice(0, 20).map((item) => ({
      ...item,
      _id: item.docId,
      updateAt: new Date(item.updateAt),
    }))
    const vndict: Record<string, Record<string, { title: string }>> = { '20': {} }
    for (const node of data.nodes) vndict['20'][node.docId] = { title: node.title || node.docId }
    const vnodes = data.nodes.map((node) => ({
      docId: node.docId,
      title: node.title,
      content: node.category,
      pic: node.pic ?? '',
    }))
    const recentProblems = data.problems.slice(0, 10).map((problem) => ({
      _id: problem.id,
      docId: problem.docId,
      pid: problem.pid,
      title: problem.title,
      hidden: false,
    }))
    const udict: Record<string, Record<string, unknown>> = Object.fromEntries(users.map((item) => [String(item.id), {
      _id: item.id,
      uname: item.username,
      displayName: item.displayName ?? '',
      avatar: item.avatar ?? '',
    }]))
    for (const item of data.ranking) {
      udict[String(item.id)] = {
        _id: item.id,
        uname: item.username,
        displayName: item.displayName ?? '',
        avatar: item.avatar ?? '',
        bio: item.bio ?? '',
        rp: item.rp,
        level: item.level,
        nAccept: item.nAccept,
        hasPriv: () => false,
        hasPerm: () => false,
      }
    }
    return {
      page_name: 'homepage',
      contents: [
        {
          width: 9,
          sections: [
            ['contest', [contests, {}]],
            ['homework', [homeworks, {}]],
            ['training', [trainings, {}]],
            ['ranking', data.ranking.map((item) => item.id)],
            ['discussion', [discussions, vndict]],
          ],
        },
        {
          width: 3,
          sections: [
            ['recent_problems', [recentProblems, {}]],
            ['discussion_nodes', vnodes],
            ['problem_search', true],
            ['suggestion', true],
          ],
        },
      ],
      udict,
      domain: {},
      handler,
      perm: HOME_PERMS,
      PRIV: { PRIV_USER_PROFILE: 1, PRIV_CREATE_FILE: 2 },
      model: {
        contest: contestModel,
        training: {
          getPids: (dag: Array<{ pids?: Array<number | string> }>) => dag.flatMap((node) => node.pids ?? []),
        },
        discussion: { typeDisplay: { 10: 'problem', 20: 'node', 30: 'contest', 40: 'training' } },
        document: { TYPE_PROBLEM: 10, TYPE_CONTEST: 30, TYPE_DISCUSSION_NODE: 20, TYPE_TRAINING: 40 },
      },
      avatarUrl,
      UiContext: {},
      UserContext: user,
    }
  }, [data, user, users])

  const buildUrl = useMemo(() => (name: string, args: Record<string, unknown> = {}) => {
    switch (name) {
      case 'contest_main': return '/contest'
      case 'contest_detail': return `/contest/${encodeURIComponent(String(args.tid ?? ''))}`
      case 'homework_main': return '/homework'
      case 'homework_detail': return `/homework/${encodeURIComponent(String(args.tid ?? ''))}`
      case 'training_main': return '/training'
      case 'training_detail': return `/training/${encodeURIComponent(String(args.tid ?? ''))}`
      case 'discussion_main': return '/discuss'
      case 'discussion_detail': return `/discuss/${encodeURIComponent(String(args.did ?? ''))}`
      case 'discussion_node': return `/discuss/node/${encodeURIComponent(String(args.name ?? ''))}`
      case 'problem_detail': return `/p/${encodeURIComponent(String(args.pid ?? ''))}`
      case 'user_detail': return `/user/${encodeURIComponent(String(args.uid ?? ''))}`
      case 'ranking': return '/ranking'
      default: return '/'
    }
  }, [])

  if (error) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  }
  if (!data) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>
  }
  return <HydroTemplate template="main.html" context={context} url={buildUrl} />
}
