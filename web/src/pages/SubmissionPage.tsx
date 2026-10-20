// Page markup is rendered from the pinned Hydro ui-default template. Keep this
// file limited to polling and mapping the submission API to Hydro rdoc fields.
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useParams } from 'react-router-dom'
import { getSubmission, type Submission } from '../api.ts'
import { useAuth } from '../auth.tsx'
import { languageOptions } from '../components/autocomplete/LanguageSelectAutoComplete.tsx'
import { HydroTemplate } from '../components/HydroTemplate.tsx'
import { statusClass, statusLabels } from '../status.ts'

const languageMap = Object.fromEntries(languageOptions.map((language) => [
  language._id,
  { display: language.name, highlight: language._id },
]))

const statusCodes = Object.fromEntries(Object.keys(statusLabels).map((status) => [
  status,
  statusClass(Number(status)),
]))

const statusShortTexts: Record<number, string> = {
  1: 'AC',
  2: 'WA',
  3: 'TLE',
  4: 'MLE',
  5: 'OLE',
  6: 'RE',
  7: 'CE',
  8: 'SE',
  9: 'Canceled',
  10: 'Unknown',
  11: 'Hacked',
  20: 'Running',
  21: 'Compiling',
  22: 'Fetched',
  30: 'Ignored',
}

function judgeText(value: unknown) {
  if (typeof value === 'string') return value
  if (value && typeof value === 'object' && 'message' in value) return String((value as { message: unknown }).message ?? '')
  return JSON.stringify(value)
}

function formatDate(value: string) {
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}

export function SubmissionPage() {
  const { rid = '' } = useParams()
  const { user } = useAuth()
  const [submission, setSubmission] = useState<Submission | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
    let timer: number | undefined
    const load = async () => {
      try {
        const next = await getSubmission(rid)
        if (cancelled) return
        setSubmission(next)
        setError('')
        if ([0, 20, 21, 22].includes(next.status)) timer = window.setTimeout(load, 1000)
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : '提交加载失败')
      }
    }
    load()
    return () => {
      cancelled = true
      if (timer) window.clearTimeout(timer)
    }
  }, [rid])

  const handleReady = useCallback((root: HTMLDivElement) => {
    root.querySelector('a[href="?download=true"]')?.closest('.section__tools')?.remove()
    root.querySelectorAll('.compiler-text-mask').forEach((element) => element.remove())
  }, [])

  const context = useMemo(() => {
    if (!submission) return null
    const isAdmin = Boolean(user && ((user.priv ?? 0) & 1) !== 0)
    const testCases = (submission.testCases ?? []).map((item, index) => ({
      id: Number(item.id ?? index + 1),
      subtaskId: Number(item.subtaskId ?? 1),
      status: Number(item.status ?? 0),
      score: Number(item.score ?? 0),
      time: Number(item.time ?? 0),
      memory: Number(item.memory ?? 0),
      message: item.message ?? '',
    }))
    const subtaskGroups = new Map<number, typeof testCases>()
    for (const testCase of testCases) {
      const group = subtaskGroups.get(testCase.subtaskId) ?? []
      group.push(testCase)
      subtaskGroups.set(testCase.subtaskId, group)
    }
    const subtasks = Object.fromEntries(Array.from(subtaskGroups.entries()).map(([id, cases]) => [
      id,
      {
        type: 'min',
        status: cases.every((testCase) => testCase.status === 1) ? 1 : 2,
        score: cases.reduce((sum, testCase) => sum + testCase.score, 0),
      },
    ]))
    const compilerTexts = (submission.compilerTexts ?? []).map(String)
    const judgeTexts = (submission.judgeTexts ?? []).map(judgeText)
    const submitter = submission.submitter ? {
      _id: submission.submitter.id,
      uname: submission.submitter.displayName || submission.submitter.username,
      avatar: '',
      hasPriv: () => false,
      hasPerm: () => false,
    } : null
    const pdoc = submission.problem ? {
      docId: submission.problem.docId,
      pid: submission.problem.pid,
      title: submission.problem.title,
      hidden: false,
      tag: [],
      config: {
        type: 'default',
        hackable: false,
        timeMin: '1000',
        timeMax: '1000',
        memoryMin: '256',
        memoryMax: '256',
      },
    } : null
    const dates = new Map([[submission.id, submission.createdAt]])
    const rdoc = {
      _id: submission.id,
      uid: submission.uid,
      pid: submission.pid,
      domainId: 'system',
      lang: submission.lang,
      code: submission.code || '',
      status: submission.status,
      score: submission.score,
      time: submission.time,
      memory: submission.memory,
      compilerTexts,
      judgeTexts,
      testCases,
      subtasks,
      judgeAt: submission.judgeAt,
      createdAt: submission.createdAt,
      files: { hack: false },
      contest: null,
    }
    return {
      page_name: 'record_detail',
      rdoc,
      pdoc,
      tdoc: null,
      udoc: submitter,
      judge_udoc: null,
      allRevs: [],
      rev: false,
      handler: {
        user: {
          _id: user?.id ?? 0,
          viewLang: 'zh',
          _dudoc: { join: true },
          hasPerm: (permission: number) => Boolean(user) && permission === 2,
          hasPriv: () => Boolean(user),
          own: () => Boolean(user && isAdmin),
        },
        args: { domainId: 'system' },
        request: { query: {}, headers: { 'user-agent': navigator.userAgent } },
        session: { legacy: false, viewLang: 'zh' },
        ctx: { setting: { get: () => '' }, i18n: { get: () => '' } },
      },
      perm: { PERM_REJUDGE: 1, PERM_SUBMIT_PROBLEM: 2, PERM_VIEW_USER_PRIVATE_INFO: 3, PERM_MOD_BADGE: 4 },
      PRIV: { PRIV_USER_PROFILE: 1, PRIV_READ_PROBLEM_DATA: 2 },
      STATUS: {
        STATUS_ACCEPTED: 1,
        STATUS_TIME_LIMIT_EXCEEDED: 3,
        STATUS_MEMORY_LIMIT_EXCEEDED: 4,
        STATUS_OUTPUT_LIMIT_EXCEEDED: 5,
        STATUS_JUDGING: 20,
      },
      model: {
        builtin: {
          LEVELS: {},
          STATUS_CODES: statusCodes,
          STATUS_TEXTS: statusLabels,
          STATUS_SHORT_TEXTS: statusShortTexts,
        },
        setting: { langs: languageMap },
        system: { get: () => false },
        contest: { canShowSelfRecord: () => true },
      },
      utils: {
        status: {
          getScoreColor: (score: number) => score >= 100 ? '#25ad40' : score > 0 ? '#e64c72' : '#999',
        },
      },
      datetimeSpan: (value: string) => {
        const date = dates.get(String(value)) || String(value)
        return `<span class="time">${formatDate(date)}</span>`
      },
      formatJudgeTexts: (values: unknown[]) => values.map(judgeText).join('\n'),
      avatarUrl: (value?: string) => value || '/logo.png',
      UiContext: {},
      UserContext: user,
    }
  }, [submission, user])

  if (error) return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  if (!submission || !context) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>

  return (
    <HydroTemplate
      template="record_detail.html"
      context={context}
      onReady={handleReady}
      url={(name, args = {}) => {
        if (name === 'record_detail') return `/record/${encodeURIComponent(String(args.rid ?? ''))}`
        if (name === 'problem_detail') return `/p/${encodeURIComponent(String(args.pid ?? ''))}`
        if (name === 'problem_hack') return `/p/${encodeURIComponent(String(args.pid ?? ''))}`
        if (name === 'user_detail') return `/user/${encodeURIComponent(String(args.uid ?? ''))}`
        if (name === 'contest_detail' || name === 'homework_detail') {
          return `/${name === 'contest_detail' ? 'contest' : 'homework'}/${encodeURIComponent(String(args.tid ?? ''))}`
        }
        return '/'
      }}
    />
  )
}
