export const statusLabels: Record<number, string> = {
  0: '等待评测',
  1: 'Accepted',
  2: 'Wrong Answer',
  3: 'Time Limit Exceeded',
  4: 'Memory Limit Exceeded',
  5: 'Output Limit Exceeded',
  6: 'Runtime Error',
  7: 'Compile Error',
  8: 'System Error',
  9: 'Cancelled',
  10: 'Unknown Error',
  11: 'Hacked',
  20: '运行中',
  21: '编译中',
  22: '已领取',
  30: '已忽略',
  31: '格式错误',
  32: 'Hack Successful',
  33: 'Hack Unsuccessful',
}

export function statusLabel(status: number) {
  return statusLabels[status] || `状态 ${status}`
}

export function statusClass(status: number) {
  if (status === 1 || status === 32) return 'pass'
  if (status === 0) return 'pending'
  if ([20, 21, 22].includes(status)) return 'progress'
  if ([9, 30, 31].includes(status)) return 'ignored'
  return 'fail'
}
