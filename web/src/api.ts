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
  userStatus?: {
    status: number
    rid: string
  }
}

export type ProblemDetail = ProblemSummary & {
  content: string
  owner?: number
  problemType?: string
  timeLimit?: string
  memoryLimit?: string
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
  testCases?: Array<Record<string, unknown>>
  judgeAt?: string
  createdAt: string
  problem?: {
    docId: number
    pid: string
    title: string
  }
  submitter?: {
    id: number
    username: string
    displayName?: string
  }
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

export async function getProblems(query = '', page = 1, sort = 'default', tag = ''): Promise<ProblemPage> {
  const params = new URLSearchParams({ page: String(page), limit: '20' })
  if (query.trim()) params.set('q', query.trim())
  if (sort && sort !== 'default') params.set('sort', sort)
  if (tag) params.set('tag', tag)
  const result = await request<Envelope<ProblemPage>>(`/api/v1/problems?${params.toString()}`)
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

export type SubmissionFilter = {
  page?: number
  limit?: number
  uidOrName?: string
  pid?: string
  tid?: string
  lang?: string
  status?: string
}

export async function getSubmissions(filter: SubmissionFilter = {}): Promise<SubmissionPage> {
  const params = new URLSearchParams({
    page: String(filter.page ?? 1),
    limit: String(filter.limit ?? 20),
  })
  for (const key of ['uidOrName', 'pid', 'tid', 'lang', 'status'] as const) {
    const value = filter[key]
    if (value) params.set(key, value)
  }
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
  owner: number
  attend: number
  pids: number[] | null
  duration?: number
  rated?: boolean
  lockAt?: string
  unlocked?: boolean
  penaltySince?: string
}

export async function getContests(query = '', rule = ''): Promise<Contest[]> {
  const params = new URLSearchParams()
  if (query.trim()) params.set('q', query.trim())
  if (rule) params.set('rule', rule)
  const suffix = params.toString() ? `?${params.toString()}` : ''
  const result = await request<Envelope<{ items: Contest[]; total: number }>>(`/api/v1/contests${suffix}`)
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
  owner: number
  attend: number
  pin: boolean
  sectionCount: number
  problemCount: number
}

export type TrainingProblem = {
  docId: number
  pid: string
  title: string
  difficulty?: number
  nSubmit: number
  nAccept: number
  tried: boolean
  solved: boolean
  missing: boolean
}

export type TrainingSection = {
  id: number
  title: string
  content?: string
  requireNids: number[]
  status: 'done' | 'progress' | 'open' | 'invalid'
  progress: number
  problems: TrainingProblem[]
}

export type TrainingDetail = Training & {
  enrolled: boolean
  completed: boolean
  doneProblems: number
  totalProblems: number
  sections: TrainingSection[]
}

export async function getTrainings(query = ''): Promise<Training[]> {
  const params = new URLSearchParams()
  if (query.trim()) params.set('q', query.trim())
  const suffix = params.toString() ? `?${params.toString()}` : ''
  const result = await request<Envelope<{ items: Training[]; total: number }>>(`/api/v1/trainings${suffix}`)
  return result.data.items
}

export async function getTraining(tid: string): Promise<TrainingDetail> {
  const result = await request<Envelope<TrainingDetail>>(`/api/v1/trainings/${encodeURIComponent(tid)}`)
  return result.data
}

export async function enrollTraining(tid: string): Promise<{ enrolled: boolean; changed: boolean }> {
  const result = await request<Envelope<{ enrolled: boolean; changed: boolean }>>(`/api/v1/trainings/${encodeURIComponent(tid)}/enroll`, {
    method: 'POST',
  })
  return result.data
}

export async function getHomeworks(query = ''): Promise<Contest[]> {
  const params = new URLSearchParams()
  if (query.trim()) params.set('q', query.trim())
  const suffix = params.toString() ? `?${params.toString()}` : ''
  const result = await request<Envelope<{ items: Contest[]; total: number }>>(`/api/v1/homeworks${suffix}`)
  return result.data.items
}

export async function getHomework(tid: string): Promise<Contest> {
  const result = await request<Envelope<Contest>>(`/api/v1/homeworks/${encodeURIComponent(tid)}`)
  return result.data
}

export type HomeworkProblemPage = {
  homework: Contest
  problems: ProblemSummary[]
}

export async function getHomeworkProblems(tid: string): Promise<HomeworkProblemPage> {
  const result = await request<Envelope<HomeworkProblemPage>>(`/api/v1/homeworks/${encodeURIComponent(tid)}/problems`)
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
  lock: boolean
  parentType: number
  parentId?: string | number
  parentTitle?: string
}

export type DiscussionNode = {
  docId: string
  title: string
  category: string
  pic?: string
}

export type DiscussionTailReply = {
  id: string
  owner: number
  content: string
  edited: boolean
  createdAt: string
}

export type DiscussionReply = {
  id: string
  docId: string
  parentId: string
  owner: number
  content: string
  edited: boolean
  replies?: DiscussionTailReply[]
  createdAt: string
}

export async function getDiscussions(): Promise<Discussion[]> {
  const result = await request<Envelope<{ items: Discussion[]; total: number }>>('/api/v1/discussions')
  return result.data.items
}

export async function getDiscussionNodes(): Promise<DiscussionNode[]> {
  const result = await request<Envelope<{ items: DiscussionNode[]; total: number }>>('/api/v1/discussion-nodes')
  return result.data.items
}

export async function createDiscussion(input: {
  nodeId: string
  title: string
  content: string
  pin?: boolean
  highlight?: boolean
}): Promise<Discussion> {
  const result = await request<Envelope<Discussion>>('/api/v1/discussions', {
    method: 'POST',
    body: JSON.stringify(input),
  })
  return result.data
}

export async function getDiscussion(did: string): Promise<Discussion> {
  const result = await request<Envelope<Discussion>>(`/api/v1/discussions/${encodeURIComponent(did)}`)
  return result.data
}

export async function getDiscussionReplies(did: string): Promise<DiscussionReply[]> {
  const result = await request<Envelope<{ items: DiscussionReply[]; total: number }>>(`/api/v1/discussions/${encodeURIComponent(did)}/replies`)
  return result.data.items
}

export async function createDiscussionReply(did: string, content: string): Promise<DiscussionReply> {
  const result = await request<Envelope<DiscussionReply>>(`/api/v1/discussions/${encodeURIComponent(did)}/replies`, {
    method: 'POST',
    body: JSON.stringify({ content }),
  })
  return result.data
}

export type PublicUser = {
  id: number
  username: string
  displayName?: string
  studentId?: string
  avatar?: string
  regAt: string
  loginAt?: string
  bio?: string
  nSubmit: number
  nAccept: number
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

export type AdminUser = {
  id: number
  username: string
  displayName?: string
  email: string
  studentId?: string
  priv: number
  regAt: string
  loginAt: string
}

export type AdminUserPage = {
  items: AdminUser[]
  total: number
  page: number
  limit: number
}

export async function getAdminUsers(query = ''): Promise<AdminUserPage> {
  const params = new URLSearchParams()
  if (query.trim()) params.set('q', query.trim())
  const suffix = params.toString() ? `?${params.toString()}` : ''
  const result = await request<Envelope<AdminUserPage>>(`/api/v1/admin/users${suffix}`)
  return result.data
}

export async function updateAdminUserPriv(uid: number, priv: number): Promise<void> {
  await request<Envelope<{ id: number; priv: number }>>(`/api/v1/admin/users/${uid}/priv`, {
    method: 'PATCH',
    body: JSON.stringify({ priv }),
  })
}
