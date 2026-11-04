export type PaginationKind = 'problem' | 'contest' | 'discussion' | 'reply' | 'record' | 'ranking' | 'solution' | 'training'

type PaginationFields = {
  paginationProblem: number
  paginationContest: number
  paginationDiscussion: number
  paginationReply: number
  paginationRecord: number
  paginationRanking: number
  paginationSolution: number
  paginationTraining: number
}

export const DEFAULT_PAGINATION: Record<PaginationKind, number> = {
  problem: 100,
  contest: 20,
  discussion: 50,
  reply: 50,
  record: 100,
  ranking: 100,
  solution: 20,
  training: 10,
}

const FIELD_BY_KIND: Record<PaginationKind, keyof PaginationFields> = {
  problem: 'paginationProblem',
  contest: 'paginationContest',
  discussion: 'paginationDiscussion',
  reply: 'paginationReply',
  record: 'paginationRecord',
  ranking: 'paginationRanking',
  solution: 'paginationSolution',
  training: 'paginationTraining',
}

let current = { ...DEFAULT_PAGINATION }

export function applyPaginationSettings(settings: Partial<PaginationFields>) {
  const next = { ...current }
  for (const [kind, field] of Object.entries(FIELD_BY_KIND) as Array<[PaginationKind, keyof PaginationFields]>) {
    const value = settings[field]
    if (typeof value === 'number' && Number.isInteger(value) && value >= 1 && value <= 1000) {
      next[kind] = value
    }
  }
  current = next
}

export function getPaginationLimit(kind: PaginationKind): number {
  return current[kind]
}
