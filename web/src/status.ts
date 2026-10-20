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
  20: '运行中',
  21: '编译中',
  22: '已领取',
  31: '格式错误',
}

export function statusLabel(status: number) {
  return statusLabels[status] || `状态 ${status}`
}
