// Ported from @hydrooj/ui-default components/media/media.page.js (AGPL-3.0).
// The original asks the server for rendered fragments. This single-site SPA
// resolves the same four internal resource types through the existing APIs and
// renders the upstream partials locally.
import { getContest, getHomework, getProblem, getUsersByIDs, type ProblemDetail, type UserChoice } from '../api.ts'
import { avatarUrl } from './avatar.ts'
import { renderHydroTemplate } from './render.ts'

type MediaKind = 'user' | 'problem' | 'contest' | 'homework'

type Candidate = {
  element: HTMLElement
  kind: MediaKind
  id: string
}

const userCache = new Map<number, Promise<UserChoice | null>>()
const problemCache = new Map<string, Promise<ProblemDetail | null>>()
const contestCache = new Map<string, Promise<Awaited<ReturnType<typeof getContest>> | null>>()
const homeworkCache = new Map<string, Promise<Awaited<ReturnType<typeof getHomework>> | null>>()

function internalPath(href: string) {
  try {
    const url = new URL(href, window.location.origin)
    if (url.origin !== window.location.origin) return ''
    return url.pathname.replace(/^\/d\/[^/]+/, '')
  } catch {
    return ''
  }
}

function routeCandidate(path: string): { kind: MediaKind; id: string } | null {
  const match = /^\/(user|p|contest|homework)\/([^/]+)\/?$/.exec(path)
  if (!match || !match[2]) return null
  const kind = match[1] === 'p' ? 'problem' : match[1] as MediaKind
  return { kind, id: decodeURIComponent(match[2]) }
}

function candidateElements(root: ParentNode) {
  const typoElements = [
    ...(root instanceof Element && root.matches('.typo') ? [root] : []),
    ...root.querySelectorAll<HTMLElement>('.typo'),
  ]
  const candidates: Candidate[] = []
  for (const typo of typoElements) {
    if (typo.classList.contains('no-media')) continue
    for (const element of typo.querySelectorAll<HTMLElement>('a[href], div[data-user]')) {
      if (element.dataset.hydroMedia) continue
      if (element.closest('.user-profile-link, .media-link')) continue
      if (element instanceof HTMLAnchorElement) {
        const route = routeCandidate(internalPath(element.getAttribute('href') ?? ''))
        if (!route) continue
        element.dataset.hydroMedia = route.kind
        candidates.push({ element, ...route })
      } else {
        const id = Number(element.textContent?.trim())
        if (!Number.isSafeInteger(id) || id <= 0) continue
        element.dataset.hydroMedia = 'user'
        candidates.push({ element, kind: 'user', id: String(id) })
      }
    }
  }
  return candidates
}

function userPromise(id: number) {
  let promise = userCache.get(id)
  if (!promise) {
    promise = getUsersByIDs([id]).then((items) => items[0] ?? null).catch(() => null)
    userCache.set(id, promise)
  }
  return promise
}

function problemPromise(id: string) {
  let promise = problemCache.get(id)
  if (!promise) {
    promise = getProblem(id).catch(() => null)
    problemCache.set(id, promise)
  }
  return promise
}

function contestPromise(id: string) {
  let promise = contestCache.get(id)
  if (!promise) {
    promise = getContest(id).catch(() => null)
    contestCache.set(id, promise)
  }
  return promise
}

function homeworkPromise(id: string) {
  let promise = homeworkCache.get(id)
  if (!promise) {
    promise = getHomework(id).catch(() => null)
    homeworkCache.set(id, promise)
  }
  return promise
}

function renderUser(user: UserChoice) {
  return renderHydroTemplate('partials/user.html', {
    context: {
      udoc: {
        _id: user.id,
        uname: user.username,
        displayName: user.displayName ?? '',
        avatar: user.avatar ?? '',
        hasPriv: () => false,
        hasPerm: () => false,
      },
      handler: { user: { hasPerm: () => false } },
      perm: { PERM_VIEW_USER_PRIVATE_INFO: 1 },
      PRIV: { PRIV_MOD_BADGE: 1 },
      avatarUrl,
      UiContext: { cdn_prefix: '/' },
    },
  })
}

function renderProblem(problem: ProblemDetail) {
  return renderHydroTemplate('partials/problem.html', {
    context: {
      pdoc: { ...problem, tag: problem.tags ?? [] },
    },
  })
}

function renderContest(contest: Awaited<ReturnType<typeof getContest>>) {
  return renderHydroTemplate('partials/contest.html', {
    context: { tdoc: { ...contest, domainId: 'system', docId: contest.docId } },
  })
}

function renderHomework(homework: Awaited<ReturnType<typeof getHomework>>) {
  return renderHydroTemplate('partials/homework.html', {
    context: { tdoc: { ...homework, domainId: 'system', docId: homework.docId } },
  })
}

function replaceElement(element: HTMLElement, html: string) {
  if (!element.isConnected || !html.trim()) return
  const template = document.createElement('template')
  template.innerHTML = html.trim()
  for (const node of template.content.children) {
    if (node instanceof HTMLElement && node.matches('a[href], div[data-user]')) node.dataset.hydroMedia = 'rendered'
  }
  for (const node of template.content.querySelectorAll<HTMLElement>('a[href], div[data-user]')) {
    node.dataset.hydroMedia = 'rendered'
  }
  element.replaceWith(...template.content.childNodes)
}

async function renderCandidate(candidate: Candidate) {
  const { element, kind, id } = candidate
  let html = ''
  if (kind === 'user') {
    const user = await userPromise(Number(id))
    if (user) html = renderUser(user)
  } else if (kind === 'problem') {
    const problem = await problemPromise(id)
    if (problem) html = renderProblem(problem)
  } else if (kind === 'contest') {
    const contest = await contestPromise(id)
    if (contest) html = renderContest(contest)
  } else if (kind === 'homework') {
    const homework = await homeworkPromise(id)
    if (homework) html = renderHomework(homework)
  }
  if (html) replaceElement(element, html)
}

export async function initHydroMedia(root: ParentNode) {
  const candidates = candidateElements(root)
  if (!candidates.length) return
  await Promise.all(candidates.map((candidate) => renderCandidate(candidate)))
}
