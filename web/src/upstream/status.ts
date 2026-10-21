// Submission status tables derived from Hydro `packages/common/status.ts` and
// `packages/common/status.ts` (AGPL-3.0). They are data tables used by the
// upstream templates (`components/record.html`), not custom UI.
export const STATUS = {
  STATUS_WAITING: 0,
  STATUS_ACCEPTED: 1,
  STATUS_WRONG_ANSWER: 2,
  STATUS_TIME_LIMIT_EXCEEDED: 3,
  STATUS_MEMORY_LIMIT_EXCEEDED: 4,
  STATUS_OUTPUT_LIMIT_EXCEEDED: 5,
  STATUS_RUNTIME_ERROR: 6,
  STATUS_COMPILE_ERROR: 7,
  STATUS_SYSTEM_ERROR: 8,
  STATUS_CANCELED: 9,
  STATUS_ETC: 10,
  STATUS_HACKED: 11,
  STATUS_JUDGING: 20,
  STATUS_COMPILING: 21,
  STATUS_FETCHED: 22,
  STATUS_IGNORED: 30,
  STATUS_FORMAT_ERROR: 31,
  STATUS_HACK_SUCCESSFUL: 32,
  STATUS_HACK_UNSUCCESSFUL: 33,
} as const

export const STATUS_TEXTS: Record<number, string> = {
  0: 'Waiting',
  1: 'Accepted',
  2: 'Wrong Answer',
  3: 'Time Exceeded',
  4: 'Memory Exceeded',
  5: 'Output Exceeded',
  6: 'Runtime Error',
  7: 'Compile Error',
  8: 'System Error',
  9: 'Cancelled',
  10: 'Unknown Error',
  11: 'Hacked',
  20: 'Running',
  21: 'Compiling',
  22: 'Fetched',
  30: 'Ignored',
  31: 'Format Error',
  32: 'Hack Successful',
  33: 'Hack Unsuccessful',
}

export const STATUS_SHORT_TEXTS: Record<number, string> = {
  1: 'AC',
  2: 'WA',
  3: 'TLE',
  4: 'MLE',
  5: 'OLE',
  6: 'RE',
  7: 'CE',
  8: 'SE',
  9: 'IGN',
  11: 'HK',
  30: 'IGN',
  31: 'FE',
}

export const STATUS_CODES: Record<number, string> = {
  0: 'pending',
  1: 'pass',
  2: 'fail',
  3: 'fail',
  4: 'fail',
  5: 'fail',
  6: 'fail',
  7: 'fail',
  8: 'fail',
  9: 'ignored',
  10: 'fail',
  11: 'fail',
  20: 'progress',
  21: 'progress',
  22: 'progress',
  30: 'ignored',
  31: 'ignored',
  32: 'pass',
  33: 'fail',
}

export function getScoreColor(score: number | string): string {
  if (score === null || score === undefined || !Number.isFinite(+score)) return '#000000'
  return [
    '#ff4f4f',
    '#ff694f',
    '#f8603a',
    '#fc8354',
    '#fa9231',
    '#f7bb3b',
    '#ecdb44',
    '#e2ec52',
    '#b0d628',
    '#93b127',
    '#25ad40',
  ][Math.floor((Number(score) || 0) / 10)]
}
