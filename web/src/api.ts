export type User = {
  id: number
  username: string
  email: string
  displayName?: string
  studentId?: string
  avatar?: string
  priv?: number
}

export type ProblemSummary = {
  id: string
  docId: number
  pid: string
  title: string
  tags: string[] | null
  difficulty?: number
  nSubmit: number
  nAccept: number
}

export type ProblemDetail = ProblemSummary & {
  content: string
  owner?: number
}

export type Submission = {
  id: string
  uid: number
  pid: number
  lang: string
  code?: string
  status: number
  score: number
  time: number
  memory: number
  judgeTexts?: Array<{ message?: string } | string>
  compilerTexts?: string[]
  judgeAt?: string
}

type Envelope<T> = {
  code: number
  message: string
  data: T
}

type LoginResponse = { user: User }

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    credentials: 'include',
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...(init?.headers ?? {}),
    },
  })
  const payload = await response.json().catch(() => null) as { error?: string } | T | null
  if (!response.ok) {
    const message = payload && typeof payload === 'object' && 'error' in payload
      ? payload.error
      : `HTTP ${response.status}`
    throw new Error(message || `HTTP ${response.status}`)
  }
  return payload as T
}

export async function getMe(): Promise<User> {
  const result = await request<LoginResponse>('/api/v1/auth/me')
  return result.user
}

export async function login(username: string, password: string): Promise<User> {
  const result = await request<LoginResponse>('/api/v1/auth/login', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  })
  return result.user
}

export async function logout(): Promise<void> {
  await request<void>('/api/v1/auth/logout', { method: 'POST' })
}

export type ProblemPage = {
  items: ProblemSummary[]
  total: number
  page: number
  limit: number
}

export async function getProblems(query = ''): Promise<ProblemPage> {
  const params = new URLSearchParams()
  if (query.trim()) params.set('q', query.trim())
  const suffix = params.toString() ? `?${params.toString()}` : ''
  const result = await request<Envelope<ProblemPage>>(`/api/v1/problems${suffix}`)
  return result.data
}

export async function getProblem(pid: string): Promise<ProblemDetail> {
  const result = await request<Envelope<ProblemDetail>>(`/api/v1/problems/${encodeURIComponent(pid)}`)
  return result.data
}

export async function createSubmission(pid: string, lang: string, code: string, contestId = ''): Promise<Submission> {
  const result = await request<Envelope<Submission>>('/api/v1/submissions', {
    method: 'POST',
    body: JSON.stringify({ pid, lang, code, contestId }),
  })
  return result.data
}

export async function getSubmission(rid: string): Promise<Submission> {
  const result = await request<Envelope<Submission>>(`/api/v1/submissions/${encodeURIComponent(rid)}`)
  return result.data
}

export type SubmissionPage = {
  items: Submission[]
  total: number
  page: number
  limit: number
}

export async function getSubmissions(page = 1, limit = 20): Promise<SubmissionPage> {
  const params = new URLSearchParams({ page: String(page), limit: String(limit) })
  const result = await request<Envelope<SubmissionPage>>(`/api/v1/submissions?${params.toString()}`)
  return result.data
}

export type Contest = {
  id: string
  docId: string
  title: string
  content?: string
  beginAt: string
  endAt: string
  rule: string
  attend: number
  pids: number[] | null
  duration?: number
  rated?: boolean
  lockAt?: string
  unlocked?: boolean
}

export async function getContests(): Promise<Contest[]> {
  const result = await request<Envelope<{ items: Contest[]; total: number }>>('/api/v1/contests')
  return result.data.items
}

export async function getContest(tid: string): Promise<Contest> {
  const result = await request<Envelope<Contest>>(`/api/v1/contests/${encodeURIComponent(tid)}`)
  return result.data
}

export type ContestProblemPage = {
  contest: Contest
  problems: ProblemSummary[]
}

export async function getContestProblems(tid: string): Promise<ContestProblemPage> {
  const result = await request<Envelope<ContestProblemPage>>(`/api/v1/contests/${encodeURIComponent(tid)}/problems`)
  return result.data
}

export type ScoreboardCell = {
  solved: boolean
  attempts: number
  time: number
}

export type ScoreboardRow = {
  rank: number
  uid: number
  solved: number
  penalty: number
  problems: Record<string, ScoreboardCell>
}

export type ContestScoreboard = {
  contest: Contest
  rows: ScoreboardRow[]
}

export async function getContestScoreboard(tid: string): Promise<ContestScoreboard> {
  const result = await request<Envelope<ContestScoreboard>>(`/api/v1/contests/${encodeURIComponent(tid)}/scoreboard`)
  return result.data
}

export type Training = {
  id: string
  docId: string
  title: string
  description?: string
  content?: string
  pin: boolean
}

export async function getTrainings(): Promise<Training[]> {
  const result = await request<Envelope<{ items: Training[]; total: number }>>('/api/v1/trainings')
  return result.data.items
}

export async function getTraining(tid: string): Promise<Training> {
  const result = await request<Envelope<Training>>(`/api/v1/trainings/${encodeURIComponent(tid)}`)
  return result.data
}

export async function getHomeworks(): Promise<Contest[]> {
  const result = await request<Envelope<{ items: Contest[]; total: number }>>('/api/v1/homeworks')
  return result.data.items
}

export async function getHomework(tid: string): Promise<Contest> {
  const result = await request<Envelope<Contest>>(`/api/v1/homeworks/${encodeURIComponent(tid)}`)
  return result.data
}

export type Discussion = {
  id: string
  docId: string
  title: string
  content?: string
  owner: number
  updateAt: string
  nReply: number
  views: number
  pin: boolean
  highlight: boolean
}

export async function getDiscussions(): Promise<Discussion[]> {
  const result = await request<Envelope<{ items: Discussion[]; total: number }>>('/api/v1/discussions')
  return result.data.items
}

export async function getDiscussion(did: string): Promise<Discussion> {
  const result = await request<Envelope<Discussion>>(`/api/v1/discussions/${encodeURIComponent(did)}`)
  return result.data
}

export type PublicUser = {
  id: number
  username: string
  displayName?: string
  studentId?: string
  avatar?: string
  regAt: string
}

export async function getPublicUser(uid: string): Promise<PublicUser> {
  const result = await request<Envelope<PublicUser>>(`/api/v1/users/${encodeURIComponent(uid)}`)
  return result.data
}

export type AdminStats = {
  users: number
  problems: number
  contests: number
  homeworks: number
  trainings: number
  discussions: number
  records: number
  submissions_today: number
  queued_tasks: number
}

export async function getAdminStats(): Promise<AdminStats> {
  const result = await request<Envelope<AdminStats>>('/api/v1/admin/stats')
  return result.data
}
