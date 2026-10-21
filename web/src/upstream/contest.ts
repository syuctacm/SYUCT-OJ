// Contest model helpers derived from Hydro `packages/hydrooj/src/model/contest.ts`
// (AGPL-3.0). Dates must be Date objects before they reach these helpers, and
// the permission ids must match the `perm` map passed to the page context.

export type ContestDocLike = {
  beginAt: Date
  endAt: Date
  duration?: number
  lockAt?: Date | null
  penaltySince?: Date | null
  unlocked?: boolean
  keepScoreboardHidden?: boolean
  rule: string
}

export type ContestStatusLike = {
  attend?: boolean
  startAt?: Date
  endAt?: Date
}

type ContestRule = {
  TEXT: string
  showScoreboard: (tdoc: ContestDocLike, now: Date) => boolean
  showSelfRecord: (tdoc: ContestDocLike, now: Date) => boolean
  showRecord: (tdoc: ContestDocLike, now: Date) => boolean
}

type HandlerLike = {
  user: {
    _id: number
    own: (doc?: unknown) => boolean
    hasPerm: (perm: number) => boolean
  }
}

const HOUR = 3600 * 1000
const DAY = 24 * HOUR

export const CONTEST_PERMS = {
  PERM_VIEW_USER_PRIVATE_INFO: 2,
  PERM_VIEW_CONTEST_HIDDEN_SCOREBOARD: 3,
  PERM_VIEW_HOMEWORK_HIDDEN_SCOREBOARD: 4,
  PERM_ATTEND_CONTEST: 5,
  PERM_EDIT_CONTEST: 6,
  PERM_READ_RECORD_CODE: 7,
  PERM_CREATE_DISCUSSION: 8,
  PERM_VIEW_DISCUSSION: 9,
  PERM_MOD_BADGE: 10,
  PERM_ATTEND_HOMEWORK: 11,
  PERM_EDIT_HOMEWORK: 12,
  PERM_VIEW_HOMEWORK_SCOREBOARD: 13,
  PERM_REJUDGE: 14,
  PERM_EDIT_TRAINING: 15,
} as const

export function isNew(tdoc: ContestDocLike, days = 1) {
  return Date.now() < tdoc.beginAt.getTime() - days * DAY
}

export function isUpcoming(tdoc: ContestDocLike, days = 7) {
  const now = Date.now()
  return now > tdoc.beginAt.getTime() - days * DAY && now < tdoc.beginAt.getTime()
}

export function isNotStarted(tdoc: ContestDocLike) {
  return new Date() < tdoc.beginAt
}

export function isOngoing(tdoc: ContestDocLike, tsdoc?: ContestStatusLike) {
  const now = new Date()
  if (tsdoc?.endAt && tsdoc.endAt <= now) return false
  if (tsdoc && tdoc.duration && tsdoc.startAt
    && tsdoc.startAt <= new Date(Date.now() - Math.floor(tdoc.duration * HOUR))) return false
  return tdoc.beginAt <= now && now < tdoc.endAt
}

export function isDone(tdoc: ContestDocLike, tsdoc?: ContestStatusLike) {
  if (tdoc.endAt <= new Date()) return true
  if (tsdoc?.endAt && tsdoc.endAt <= new Date()) return true
  if (tsdoc && tdoc.duration && tsdoc.startAt
    && tsdoc.startAt <= new Date(Date.now() - Math.floor(tdoc.duration * HOUR))) return true
  return false
}

export function isLocked(tdoc: ContestDocLike, time = new Date()) {
  if (!tdoc.lockAt) return false
  return tdoc.lockAt < time && !tdoc.unlocked
}

export function isExtended(tdoc: ContestDocLike) {
  if (!tdoc.penaltySince) return false
  const now = Date.now()
  return tdoc.penaltySince.getTime() <= now && now < tdoc.endAt.getTime()
}

export const RULES: Record<string, ContestRule> = {
  acm: {
    TEXT: 'XCPC',
    showScoreboard: (tdoc, now) => now > tdoc.beginAt,
    showSelfRecord: () => true,
    showRecord: (tdoc, now) => now > tdoc.endAt && !isLocked(tdoc, now),
  },
  oi: {
    TEXT: 'OI',
    showScoreboard: (tdoc, now) => now > tdoc.endAt && !tdoc.keepScoreboardHidden,
    showSelfRecord: (tdoc, now) => now > tdoc.endAt && !tdoc.keepScoreboardHidden,
    showRecord: (tdoc, now) => now > tdoc.endAt && !tdoc.keepScoreboardHidden,
  },
  ioi: {
    TEXT: 'IOI',
    showScoreboard: (tdoc, now) => now > tdoc.beginAt,
    showSelfRecord: () => true,
    showRecord: (tdoc, now) => now > tdoc.endAt && !isLocked(tdoc, now),
  },
  strictioi: {
    TEXT: 'IOI(Strict)',
    showScoreboard: (tdoc, now) => now > tdoc.endAt && !tdoc.keepScoreboardHidden,
    showSelfRecord: (tdoc) => !tdoc.keepScoreboardHidden || !isDone(tdoc),
    showRecord: (tdoc, now) => now > tdoc.endAt && !tdoc.keepScoreboardHidden,
  },
  ledo: {
    TEXT: 'Ledo',
    showScoreboard: (tdoc, now) => now > tdoc.beginAt,
    showSelfRecord: () => true,
    showRecord: (tdoc, now) => now > tdoc.endAt,
  },
  homework: {
    TEXT: 'Assignment',
    showScoreboard: () => true,
    showSelfRecord: () => true,
    showRecord: (tdoc, now) => now > tdoc.endAt,
  },
}

export const statusText = (tdoc: ContestDocLike, tsdoc?: ContestStatusLike) => (
  isNew(tdoc)
    ? 'New'
    : isUpcoming(tdoc)
      ? 'Ready (☆▽☆)'
      : isOngoing(tdoc, tsdoc)
        ? 'Live...'
        : 'Done'
)

function canViewHiddenScoreboard(handler: HandlerLike, tdoc: ContestDocLike) {
  if (handler.user.own(tdoc)) return true
  if (tdoc.rule === 'homework') return handler.user.hasPerm(CONTEST_PERMS.PERM_VIEW_HOMEWORK_HIDDEN_SCOREBOARD)
  return handler.user.hasPerm(CONTEST_PERMS.PERM_VIEW_CONTEST_HIDDEN_SCOREBOARD)
}

// Upstream templates call these either as `model.contest.canShowRecord.call(handler, tdoc)`
// or as `model.contest.canViewHiddenScoreboard(handler, tdoc)`. Nunjucks invokes
// them as methods of `model.contest` and drops the `this` binding, so the handler
// is captured here and the arguments are normalized for both call styles.
export function createContestModel(handler: HandlerLike) {
  function normalize(args: unknown[]) {
    const first = args[0]
    if (first && typeof first === 'object' && 'user' in (first as Record<string, unknown>)) {
      return {
        handler: first as HandlerLike,
        tdoc: args[1] as ContestDocLike,
        allow: args.length > 2 ? Boolean(args[2]) : true,
      }
    }
    return {
      handler,
      tdoc: first as ContestDocLike,
      allow: args.length > 1 ? Boolean(args[1]) : true,
    }
  }
  function canShowRecord(...args: unknown[]) {
    const { handler: current, tdoc, allow } = normalize(args)
    if (!tdoc) return false
    const rule = RULES[tdoc.rule]
    if (rule?.showRecord(tdoc, new Date())) return true
    if (allow && canViewHiddenScoreboard(current, tdoc)) return true
    return false
  }
  function canShowSelfRecord(...args: unknown[]) {
    const { handler: current, tdoc, allow } = normalize(args)
    if (!tdoc) return false
    const rule = RULES[tdoc.rule]
    if (rule?.showSelfRecord(tdoc, new Date())) return true
    if (allow && canViewHiddenScoreboard(current, tdoc)) return true
    return false
  }
  function canShowScoreboard(...args: unknown[]) {
    const { handler: current, tdoc, allow } = normalize(args)
    if (!tdoc) return false
    const rule = RULES[tdoc.rule]
    if (rule?.showScoreboard(tdoc, new Date())) return true
    if (allow && canViewHiddenScoreboard(current, tdoc)) return true
    return false
  }
  function canViewHiddenScoreboardFor(...args: unknown[]) {
    const { handler: current, tdoc } = normalize(args)
    if (!tdoc) return false
    return canViewHiddenScoreboard(current, tdoc)
  }
  return {
    RULES,
    statusText,
    isNew,
    isUpcoming,
    isNotStarted,
    isOngoing,
    isDone,
    isLocked,
    isExtended,
    canShowRecord,
    canShowSelfRecord,
    canShowScoreboard,
    canViewHiddenScoreboard: canViewHiddenScoreboardFor,
  }
}
