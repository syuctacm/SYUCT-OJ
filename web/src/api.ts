export type User = {
  id: number
  username: string
  email: string
  displayName?: string
  studentId?: string
  avatar?: string
  priv?: number
  bio?: string
  qq?: string
  gender?: number
  school?: string
  backgroundImage?: string
}

export type PublicSettings = { bulletin: string }

export async function getPublicSettings(): Promise<PublicSettings> {
  const result = await request<Envelope<PublicSettings>>('/api/v1/settings')
  return result.data
}

export async function getAdminSettings(): Promise<PublicSettings> {
  const result = await request<Envelope<PublicSettings>>('/api/v1/admin/settings')
  return result.data
}

export async function updateAdminSettings(input: PublicSettings): Promise<void> {
  await request<Envelope<PublicSettings>>('/api/v1/admin/settings', {
    method: 'PUT',
    body: JSON.stringify(input),
  })
}

export type UserSettingsInput = {
  avatar?: string
  qq?: string
  gender?: number
  bio?: string
  school?: string
  studentId?: string
  backgroundImage?: string
}

export async function updateUserSettings(input: UserSettingsInput): Promise<void> {
  await request<Envelope<Record<string, unknown>>>('/api/v1/user/settings', {
    method: 'PATCH',
    body: JSON.stringify(input),
  })
}

export type ProblemSummary = {
  id: string
  docId: number
  pid: string
  title: string
  tags: string[] | null
  difficulty?: number
  hidden?: boolean
  nSubmit: number
  nAccept: number
  problemType?: string
  timeMin?: string
  timeMax?: string
  memoryMin?: string
  memoryMax?: string
  userStatus?: {
    status: number
    rid: string
  }
}

export type UserChoice = {
  id: number
  username: string
  displayName?: string
  avatar?: string
}

export type GroupChoice = {
  name: string
  uids: number[]
}

export async function searchUsers(query: string): Promise<UserChoice[]> {
  const params = new URLSearchParams({ search: query })
  const result = await request<Envelope<{ items: UserChoice[] }>>(`/api/v1/users?${params.toString()}`)
  return result.data.items
}

export async function getUsersByIDs(ids: number[]): Promise<UserChoice[]> {
  if (!ids.length) return []
  const params = new URLSearchParams({ auto: ids.join(',') })
  const result = await request<Envelope<{ items: UserChoice[] }>>(`/api/v1/users?${params.toString()}`)
  return result.data.items
}

export async function searchGroups(query: string): Promise<GroupChoice[]> {
  const params = new URLSearchParams({ search: query })
  const result = await request<Envelope<{ items: GroupChoice[] }>>(`/api/v1/groups?${params.toString()}`)
  return result.data.items
}

export async function getGroupsByNames(names: string[]): Promise<GroupChoice[]> {
  if (!names.length) return []
  const params = new URLSearchParams({ names: names.join(',') })
  const result = await request<Envelope<{ items: GroupChoice[] }>>(`/api/v1/groups?${params.toString()}`)
  return result.data.items
}

export type ProblemEditInput = {
  pid: string
  title: string
  content: string
  tags: string[]
  difficulty: number
  hidden: boolean
  config: string
}

export type ProblemEditorData = {
  problem: ProblemDetail
  config: string
}

export async function getProblemEditor(pid: string): Promise<ProblemEditorData> {
  const result = await request<Envelope<ProblemEditorData>>(`/api/v1/problems/${encodeURIComponent(pid)}/edit`)
  return result.data
}

export async function createProblem(input: ProblemEditInput): Promise<ProblemEditorData> {
  const result = await request<Envelope<ProblemEditorData>>('/api/v1/problems', {
    method: 'POST',
    body: JSON.stringify(input),
  })
  return result.data
}

export async function updateProblem(pid: string, input: ProblemEditInput): Promise<ProblemEditorData> {
  const result = await request<Envelope<ProblemEditorData>>(`/api/v1/problems/${encodeURIComponent(pid)}`, {
    method: 'PATCH',
    body: JSON.stringify(input),
  })
  return result.data
}

export type ProblemAsset = {
  id: string
  name: string
  size: number
  etag?: string
  lastModified: string
  downloadUrl: string
}

export type ActivityFile = {
  id: string
  name: string
  size: number
  etag?: string
  lastModified: string
  downloadUrl: string
}

export type ActivityFileResource = 'contests' | 'homeworks' | 'trainings'
export type ActivityFileType = 'public' | 'private'

function activityFileQuery(fileType: ActivityFileType) {
  return fileType === 'private' ? '?type=private' : ''
}

export async function getActivityFiles(resource: ActivityFileResource, tid: string, fileType: ActivityFileType = 'public'): Promise<ActivityFile[]> {
  const result = await request<Envelope<{ items: ActivityFile[]; total: number }>>(`/api/v1/${resource}/${encodeURIComponent(tid)}/files${activityFileQuery(fileType)}`)
  return result.data.items
}

export async function uploadActivityFiles(resource: ActivityFileResource, tid: string, files: File[], fileType: ActivityFileType = 'public'): Promise<ActivityFile[]> {
  const body = new FormData()
  for (const file of files) body.append('files', file)
  const result = await request<Envelope<{ items: ActivityFile[]; total: number }>>(`/api/v1/${resource}/${encodeURIComponent(tid)}/files${activityFileQuery(fileType)}`, {
    method: 'POST',
    body,
  })
  return result.data.items
}

export async function deleteActivityFiles(resource: ActivityFileResource, tid: string, names: string[], fileType: ActivityFileType = 'public'): Promise<void> {
  await request(`/api/v1/${resource}/${encodeURIComponent(tid)}/files${activityFileQuery(fileType)}`, {
    method: 'DELETE',
    body: JSON.stringify({ names }),
  })
}

export async function getUserFiles(): Promise<ActivityFile[]> {
  const result = await request<Envelope<{ items: ActivityFile[]; total: number }>>('/api/v1/user/files')
  return result.data.items
}

export async function uploadUserFiles(files: File[]): Promise<ActivityFile[]> {
  const body = new FormData()
  for (const file of files) body.append('files', file)
  const result = await request<Envelope<{ items: ActivityFile[]; total: number }>>('/api/v1/user/files', {
    method: 'POST',
    body,
  })
  return result.data.items
}

export async function deleteUserFiles(names: string[]): Promise<void> {
  await request('/api/v1/user/files', {
    method: 'DELETE',
    body: JSON.stringify({ names }),
  })
}

export type ProblemAssetList = {
  problem: ProblemDetail
  testdata: ProblemAsset[]
  additional: ProblemAsset[]
}

export async function getProblemAssets(pid: string): Promise<ProblemAssetList> {
  const result = await request<Envelope<ProblemAssetList>>(`/api/v1/problems/${encodeURIComponent(pid)}/assets`)
  return result.data
}

export async function uploadProblemAssets(pid: string, type: 'testdata' | 'additional', files: File[]): Promise<ProblemAssetList> {
  const body = new FormData()
  body.set('type', type)
  for (const file of files) body.append('files', file)
  const result = await request<Envelope<ProblemAssetList>>(`/api/v1/problems/${encodeURIComponent(pid)}/assets`, {
    method: 'POST',
    body,
  })
  return result.data
}

export async function deleteProblemAssets(pid: string, type: 'testdata' | 'additional', names: string[]): Promise<ProblemAssetList> {
  const result = await request<Envelope<ProblemAssetList>>(`/api/v1/problems/${encodeURIComponent(pid)}/assets`, {
    method: 'DELETE',
    body: JSON.stringify({ type, names }),
  })
  return result.data
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
  const headers = new Headers(init?.headers)
  if (!(init?.body instanceof FormData) && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }
  const response = await fetch(path, {
    credentials: 'include',
    ...init,
    headers,
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

export async function getProblemsByDocIDs(ids: number[]): Promise<ProblemDetail[]> {
  const items = await Promise.all(ids.map((id) => getProblem(String(id))))
  return items
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

export type ContestEditInput = {
  title: string
  content: string
  beginAt: string
  endAt: string
  rule: string
  pids: number[]
  duration: number
  rated: boolean
  penaltySince?: string
  penaltyRules?: Record<string, number>
  lockAt?: string
  unlocked: boolean
  maintainer?: number[]
  assign?: string[]
  langs?: string[]
  autoHide?: boolean
  allowViewCode?: boolean
  allowPrint?: boolean
  allowTeam?: boolean
  keepScoreboardHidden?: boolean
  invitationCode?: string
}

export async function getContestEditor(tid: string): Promise<Contest> {
  const result = await request<Envelope<Contest>>(`/api/v1/contests/${encodeURIComponent(tid)}/edit`)
  return result.data
}

export async function createContest(input: ContestEditInput): Promise<Contest> {
  const result = await request<Envelope<Contest>>('/api/v1/contests', {
    method: 'POST',
    body: JSON.stringify(input),
  })
  return result.data
}

export async function updateContest(tid: string, input: ContestEditInput): Promise<Contest> {
  const result = await request<Envelope<Contest>>(`/api/v1/contests/${encodeURIComponent(tid)}`, {
    method: 'PATCH',
    body: JSON.stringify(input),
  })
  return result.data
}

export type Contest = {
  id: string
  docId: string
  attended?: boolean
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
  penaltyRules?: Record<string, number>
  maintainer?: number[]
  assign?: string[]
  langs?: string[]
  autoHide?: boolean
  allowViewCode?: boolean
  allowPrint?: boolean
  allowTeam?: boolean
  keepScoreboardHidden?: boolean
  invitationCode?: string
  privateFiles?: ActivityFile[]
}

export type ListPage<T> = {
  items: T[]
  total: number
}

export async function getContests(query = '', rule = '', page = 1, limit = 20): Promise<ListPage<Contest>> {
  const params = new URLSearchParams({ page: String(page), limit: String(limit) })
  if (query.trim()) params.set('q', query.trim())
  if (rule) params.set('rule', rule)
  const result = await request<Envelope<ListPage<Contest>>>(`/api/v1/contests?${params.toString()}`)
  return result.data
}

export async function getContest(tid: string): Promise<Contest> {
  const result = await request<Envelope<Contest>>(`/api/v1/contests/${encodeURIComponent(tid)}`)
  return result.data
}

export async function unlockContest(tid: string): Promise<Contest> {
  const result = await request<Envelope<Contest>>(`/api/v1/contests/${encodeURIComponent(tid)}/unlock`, {
    method: 'POST',
  })
  return result.data
}

export async function attendContest(tid: string): Promise<{ attended: boolean; changed: boolean }> {
  const result = await request<Envelope<{ attended: boolean; changed: boolean }>>(`/api/v1/contests/${encodeURIComponent(tid)}/attend`, {
    method: 'POST',
  })
  return result.data
}

export type ProblemStatus = {
  rid: string
  status: number
  score: number
}

export type ContestProblemPage = {
  contest: Contest
  problems: ProblemSummary[]
  attended: boolean
  statuses: Record<string, ProblemStatus>
}

export type ContestAttendee = {
  uid: number
  unrank?: boolean
  startAt?: string
  endAt?: string
}

export async function getContestAttendees(tid: string): Promise<{ items: ContestAttendee[]; users: UserChoice[] }> {
  const result = await request<Envelope<{ items: ContestAttendee[]; users: UserChoice[] }>>(`/api/v1/contests/${encodeURIComponent(tid)}/attendees`)
  return result.data
}

export async function addContestAttendees(tid: string, uids: number[]): Promise<void> {
  await request(`/api/v1/contests/${encodeURIComponent(tid)}/attendees`, {
    method: 'POST',
    body: JSON.stringify({ uids }),
  })
}

export async function setContestAttendeeRank(tid: string, uid: number, unrank: boolean): Promise<void> {
  await request(`/api/v1/contests/${encodeURIComponent(tid)}/attendees/${uid}`, {
    method: 'PATCH',
    body: JSON.stringify({ unrank }),
  })
}

export async function cancelContestAttendee(tid: string, uid: number): Promise<void> {
  await request(`/api/v1/contests/${encodeURIComponent(tid)}/attendees/${uid}`, { method: 'DELETE' })
}

export async function getContestProblems(tid: string): Promise<ContestProblemPage> {
  const result = await request<Envelope<ContestProblemPage>>(`/api/v1/contests/${encodeURIComponent(tid)}/problems`)
  return result.data
}

export type ScoreboardCell = {
  solved: boolean
  attempts: number
  time: number
  rid?: string
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

export type ClarificationReply = {
  id: string
  owner: number
  content: string
}

export type Clarification = {
  id: string
  docId: string
  parentId: string
  owner: number
  subject: number
  content: string
  reply: ClarificationReply[]
}

export async function getClarifications(tid: string): Promise<Clarification[]> {
  const result = await request<Envelope<{ items: Clarification[]; total: number }>>(`/api/v1/contests/${encodeURIComponent(tid)}/clarifications`)
  return result.data.items
}

export async function createClarification(tid: string, input: { subject: number; content: string }): Promise<Clarification> {
  const result = await request<Envelope<Clarification>>(`/api/v1/contests/${encodeURIComponent(tid)}/clarifications`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
  return result.data
}

export async function broadcastClarification(tid: string, input: { subject: number; content: string }): Promise<Clarification> {
  const result = await request<Envelope<Clarification>>(`/api/v1/contests/${encodeURIComponent(tid)}/clarifications/broadcast`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
  return result.data
}

export async function replyClarification(tid: string, cid: string, content: string): Promise<Clarification> {
  const result = await request<Envelope<Clarification>>(`/api/v1/contests/${encodeURIComponent(tid)}/clarifications/${encodeURIComponent(cid)}/reply`, {
    method: 'POST',
    body: JSON.stringify({ content }),
  })
  return result.data
}

export type TrainingNode = {
  id: number
  title: string
  content?: string
  requireNids: number[]
  pids: number[]
}

export type TrainingEditNode = {
  _id: number
  title: string
  content?: string
  requireNids: number[]
  pids: Array<number | string>
}

export type TrainingEditInput = {
  title: string
  content: string
  description: string
  pin: boolean
  dag: TrainingEditNode[]
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
  dag?: TrainingNode[]
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
  rid?: string
  status?: number
  score?: number
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
  enrolledUsers: number[]
  sections: TrainingSection[]
}

export async function getTrainings(query = '', page = 1, limit = 20): Promise<ListPage<Training>> {
  const params = new URLSearchParams({ page: String(page), limit: String(limit) })
  if (query.trim()) params.set('q', query.trim())
  const result = await request<Envelope<ListPage<Training>>>(`/api/v1/trainings?${params.toString()}`)
  return result.data
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

export async function getTrainingEditor(tid: string): Promise<Training> {
  const result = await request<Envelope<Training>>(`/api/v1/trainings/${encodeURIComponent(tid)}/edit`)
  return result.data
}

export async function createTraining(input: TrainingEditInput): Promise<Training> {
  const result = await request<Envelope<Training>>('/api/v1/trainings', {
    method: 'POST',
    body: JSON.stringify(input),
  })
  return result.data
}

export async function updateTraining(tid: string, input: TrainingEditInput): Promise<Training> {
  const result = await request<Envelope<Training>>(`/api/v1/trainings/${encodeURIComponent(tid)}`, {
    method: 'PATCH',
    body: JSON.stringify(input),
  })
  return result.data
}

export async function getHomeworks(query = '', page = 1, limit = 20): Promise<ListPage<Contest>> {
  const params = new URLSearchParams({ page: String(page), limit: String(limit) })
  if (query.trim()) params.set('q', query.trim())
  const result = await request<Envelope<ListPage<Contest>>>(`/api/v1/homeworks?${params.toString()}`)
  return result.data
}

export async function getHomeworkEditor(tid: string): Promise<Contest> {
  const result = await request<Envelope<Contest>>(`/api/v1/homeworks/${encodeURIComponent(tid)}/edit`)
  return result.data
}

export async function createHomework(input: ContestEditInput): Promise<Contest> {
  const result = await request<Envelope<Contest>>('/api/v1/homeworks', {
    method: 'POST',
    body: JSON.stringify(input),
  })
  return result.data
}

export async function updateHomework(tid: string, input: ContestEditInput): Promise<Contest> {
  const result = await request<Envelope<Contest>>(`/api/v1/homeworks/${encodeURIComponent(tid)}`, {
    method: 'PATCH',
    body: JSON.stringify(input),
  })
  return result.data
}

export async function getHomework(tid: string): Promise<Contest> {
  const result = await request<Envelope<Contest>>(`/api/v1/homeworks/${encodeURIComponent(tid)}`)
  return result.data
}

export type HomeworkProblemPage = {
  homework: Contest
  problems: ProblemSummary[]
  attended: boolean
  statuses: Record<string, ProblemStatus>
}

export async function getHomeworkProblems(tid: string): Promise<HomeworkProblemPage> {
  const result = await request<Envelope<HomeworkProblemPage>>(`/api/v1/homeworks/${encodeURIComponent(tid)}/problems`)
  return result.data
}

export async function attendHomework(tid: string): Promise<{ attended: boolean; changed: boolean }> {
  const result = await request<Envelope<{ attended: boolean; changed: boolean }>>(`/api/v1/homeworks/${encodeURIComponent(tid)}/attend`, {
    method: 'POST',
  })
  return result.data
}

export type Discussion = {
  id: string
  docId: string
  title: string
  starred?: boolean
  content?: string
  owner: number
  updateAt: string
  nReply: number
  views: number
  pin: boolean
  highlight: boolean
  lock: boolean
  edited: boolean
  parentType: number
  parentId?: string | number
  parentTitle?: string
  react?: Record<string, number>
  reactions?: Record<string, Record<string, number>>
}

export type DiscussionHistoryEntry = { uid: number; time: string }

export async function getDiscussionHistory(did: string, docId?: string): Promise<DiscussionHistoryEntry[]> {
  const params = new URLSearchParams({ all: '1' })
  if (docId) params.set('docId', docId)
  const result = await request<Envelope<{ history: DiscussionHistoryEntry[] }>>(`/api/v1/discussions/${encodeURIComponent(did)}/raw?${params.toString()}`)
  return result.data.history
}

export async function getDiscussionRevision(did: string, docId: string | undefined, time: string): Promise<string> {
  const millis = new Date(time).getTime()
  const params = new URLSearchParams({ time: String(millis) })
  if (docId) params.set('docId', docId)
  const result = await request<Envelope<{ content: string }>>(`/api/v1/discussions/${encodeURIComponent(did)}/raw?${params.toString()}`)
  return result.data.content
}

export type DiscussionReactionNode = 'did' | 'drid'

export type DiscussionReactionResult = {
  doc: { react: Record<string, number> }
  sdoc: { react: Record<string, number> }
}

export async function reactToDiscussion(
  did: string,
  input: { id: string; nodeType: DiscussionReactionNode; emoji: string; reverse?: boolean },
): Promise<DiscussionReactionResult> {
  const result = await request<Envelope<DiscussionReactionResult>>(`/api/v1/discussions/${encodeURIComponent(did)}/reaction`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
  return result.data
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
  react?: Record<string, number>
  createdAt: string
}

export async function getDiscussions(filter: { parentType?: number; parentId?: string; page?: number; limit?: number } = {}): Promise<ListPage<Discussion>> {
  const params = new URLSearchParams({
    page: String(filter.page ?? 1),
    limit: String(filter.limit ?? 20),
  })
  if (filter.parentType) params.set('parentType', String(filter.parentType))
  if (filter.parentId) params.set('parentId', filter.parentId)
  const result = await request<Envelope<ListPage<Discussion>>>(`/api/v1/discussions?${params.toString()}`)
  return result.data
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

export async function setDiscussionStar(did: string, star: boolean): Promise<{ starred: boolean }> {
  const result = await request<Envelope<{ starred: boolean }>>(`/api/v1/discussions/${encodeURIComponent(did)}/star`, {
    method: 'POST',
    body: JSON.stringify({ star }),
  })
  return result.data
}

export async function getDiscussion(did: string): Promise<Discussion> {
  const result = await request<Envelope<Discussion>>(`/api/v1/discussions/${encodeURIComponent(did)}`)
  return result.data
}

export type DiscussionEditInput = {
  title: string
  content: string
  pin: boolean
  highlight: boolean
}

export async function updateDiscussion(did: string, input: DiscussionEditInput): Promise<Discussion> {
  const result = await request<Envelope<Discussion>>(`/api/v1/discussions/${encodeURIComponent(did)}`, {
    method: 'PATCH',
    body: JSON.stringify(input),
  })
  return result.data
}

export async function deleteDiscussion(did: string): Promise<void> {
  await request<void>(`/api/v1/discussions/${encodeURIComponent(did)}`, { method: 'DELETE' })
}

export async function setDiscussionLock(did: string, lock: boolean): Promise<Discussion> {
  const result = await request<Envelope<Discussion>>(`/api/v1/discussions/${encodeURIComponent(did)}/lock`, {
    method: 'PATCH',
    body: JSON.stringify({ lock }),
  })
  return result.data
}

export async function getDiscussionReplies(did: string): Promise<DiscussionReply[]> {
  const result = await request<Envelope<{ items: DiscussionReply[]; total: number }>>(`/api/v1/discussions/${encodeURIComponent(did)}/replies`)
  return result.data.items
}

export async function createDiscussionReply(did: string, content: string, replyTo?: string): Promise<DiscussionReply | DiscussionTailReply> {
  const result = await request<Envelope<DiscussionReply | DiscussionTailReply>>(`/api/v1/discussions/${encodeURIComponent(did)}/replies`, {
    method: 'POST',
    body: JSON.stringify(replyTo ? { content, replyTo } : { content }),
  })
  return result.data
}

export async function updateDiscussionReply(did: string, rid: string, content: string): Promise<DiscussionReply | DiscussionTailReply> {
  const result = await request<Envelope<DiscussionReply | DiscussionTailReply>>(`/api/v1/discussions/${encodeURIComponent(did)}/replies/${encodeURIComponent(rid)}`, {
    method: 'PATCH',
    body: JSON.stringify({ content }),
  })
  return result.data
}

export async function deleteDiscussionReply(did: string, rid: string): Promise<void> {
  await request<void>(`/api/v1/discussions/${encodeURIComponent(did)}/replies/${encodeURIComponent(rid)}`, { method: 'DELETE' })
}


export type ProblemTagStat = {
  tag: string
  count: number
  isSource: boolean
}

export async function getProblemTags(): Promise<ProblemTagStat[]> {
  const result = await request<Envelope<{ items: ProblemTagStat[] }>>('/api/v1/problem-tags')
  return result.data.items
}

export type TagRankingItem = {
  rank: number
  uid: number
  username: string
  displayName?: string
  solved: number
  submissions: number
}

export type TagRanking = {
  tag: string
  problemCount: number
  totalUsers: number
  matchedCount: number
  page: number
  limit: number
  items: TagRankingItem[]
}

export async function getTagRanking(tag: string, page = 1, query = ''): Promise<TagRanking> {
  const params = new URLSearchParams({ page: String(page), limit: '20' })
  if (query.trim()) params.set('q', query.trim())
  const result = await request<Envelope<TagRanking>>(`/api/v1/problem-tags/${encodeURIComponent(tag)}/ranking?${params.toString()}`)
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
  rp?: number
  level?: number
  rank?: number
  acceptedProblems?: Array<{ docId: number; pid: string; title: string }>
  qq?: string
  gender?: number
  backgroundImage?: string
  school?: string
}

export type RankingUser = {
  id: number
  username: string
  displayName?: string
  avatar?: string
  bio?: string
  rp: number
  level: number
  nAccept: number
}

export async function getRanking(page = 1, limit = 20): Promise<ListPage<RankingUser>> {
  const result = await request<Envelope<ListPage<RankingUser>>>(`/api/v1/ranking?page=${page}&limit=${limit}`)
  return result.data
}

export async function getPublicUser(uid: string): Promise<PublicUser> {
  const result = await request<Envelope<PublicUser>>(`/api/v1/users/${encodeURIComponent(uid)}`)
  return result.data
}

export type ActivityDay = {
  submissions: number
  ac: number
}

export type UserActivity = {
  totalSubmissions: number
  activeDays: number
  maxStreak: number
  totalSolved: number
  dailyStats: Record<string, ActivityDay>
}

export async function getUserActivity(uid: string): Promise<UserActivity> {
  const result = await request<Envelope<UserActivity>>(`/api/v1/users/${encodeURIComponent(uid)}/activity`)
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
  avatar?: string
  regAt: string
  loginAt: string
}

export type AdminUserPage = {
  items: AdminUser[]
  total: number
  page: number
  limit: number
}

export async function getAdminUsers(query = '', limit = 20): Promise<AdminUserPage> {
  const params = new URLSearchParams({ limit: String(limit), sort: 'id' })
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
