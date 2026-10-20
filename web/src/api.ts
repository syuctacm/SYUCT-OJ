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

export async function createSubmission(pid: string, lang: string, code: string): Promise<Submission> {
  const result = await request<Envelope<Submission>>('/api/v1/submissions', {
    method: 'POST',
    body: JSON.stringify({ pid, lang, code }),
  })
  return result.data
}

export async function getSubmission(rid: string): Promise<Submission> {
  const result = await request<Envelope<Submission>>(`/api/v1/submissions/${encodeURIComponent(rid)}`)
  return result.data
}
