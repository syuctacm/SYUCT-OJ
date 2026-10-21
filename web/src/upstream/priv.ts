// Privilege bits copied from Hydro `packages/common/permission.ts` (AGPL-3.0).
// The numeric values must stay identical to the production data.
export const PRIV = {
  PRIV_MOD_BADGE: 1 << 25,
  PRIV_EDIT_SYSTEM: 1 << 0,
  PRIV_SET_PERM: 1 << 1,
  PRIV_USER_PROFILE: 1 << 2,
  PRIV_REGISTER_USER: 1 << 3,
  PRIV_READ_PROBLEM_DATA: 1 << 4,
  PRIV_READ_RECORD_CODE: 1 << 7,
  PRIV_VIEW_HIDDEN_RECORD: 1 << 8,
  PRIV_JUDGE: 1 << 9,
  PRIV_CREATE_DOMAIN: 1 << 10,
  PRIV_VIEW_ALL_DOMAIN: 1 << 11,
  PRIV_MANAGE_ALL_DOMAIN: 1 << 12,
  PRIV_REJUDGE: 1 << 13,
  PRIV_VIEW_USER_SECRET: 1 << 14,
  PRIV_VIEW_JUDGE_STATISTICS: 1 << 15,
  PRIV_UNLIMITED_ACCESS: 1 << 22,
  PRIV_VIEW_SYSTEM_NOTIFICATION: 1 << 23,
  PRIV_SEND_MESSAGE: 1 << 24,
  PRIV_CREATE_FILE: 1 << 16,
  PRIV_UNLIMITED_QUOTA: 1 << 17,
  PRIV_DELETE_FILE: 1 << 18,
} as const

export const PRIV_NONE = 0
export const PRIV_ALL = -1

// Mirrors `PRIV.PRIV_DEFAULT` in Hydro.
export const PRIV_DEFAULT = PRIV.PRIV_USER_PROFILE + PRIV.PRIV_CREATE_FILE + PRIV.PRIV_SEND_MESSAGE

// The dialog lists every bit except the aggregate values, like upstream
// `omit(PRIV, ['PRIV_DEFAULT', 'PRIV_NEVER', 'PRIV_NONE', 'PRIV_ALL'])`.
export const PRIV_OPTIONS = PRIV
