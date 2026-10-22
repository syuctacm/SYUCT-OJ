// @ts-nocheck
const translations: Record<string, string> = {
  'Add new subtask': '添加子任务',
  'Add testcase': '添加测试点',
  'Are you sure you want to delete this subtask?': '确定删除这个子任务吗？',
  'Auto configure': '自动配置',
  'Auto detect': '自动识别',
  Basic: '基础',
  Checker: 'Checker',
  Custom: '自定义',
  'Drag and drop testcases here:': '将测试点拖到这里：',
  Enabled: '启用',
  Errors: '错误',
  FileIO: '文件 IO',
  'FileIOConfig': '文件 IO',
  'Global settings': '全局设置',
  Input: '输入',
  Interface: '接口',
  Language: '语言',
  'Max Passes': '最大遍数',
  'Multi Pass': '多遍运行',
  'Multi-file': '多文件',
  'No Errors': '无错误',
  'No testcase here': '这里没有测试点',
  Output: '输出',
  Preset: '预设',
  'Problem Type': '题目类型',
  Save: '保存',
  Subtasks: '子任务',
  Type: '类型',
  Unlimited: '不限',
  'Set time and memory limits': '设置时间和内存限制',
  'Confirm save?': '确认保存？',
  'Errors detected in the config. Confirm save?': '配置存在错误，仍要保存吗？',
}

export function i18n(key: string, ...args: Array<string | number>) {
  let value = translations[key] ?? key
  args.forEach((arg, index) => {
    value = value.replaceAll(`{${index}}`, String(arg))
  })
  return value
}

export function confirm(message: string) {
  return Promise.resolve(window.confirm(message))
}
