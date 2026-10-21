// The page templates and Chinese translations are loaded verbatim from the
// pinned @hydrooj/ui-default package. React supplies data and browser events;
// the markup itself must stay identical to Hydro.
import nunjucks from 'nunjucks'
import yaml from 'js-yaml'
import zhYaml from '@hydrooj/ui-default/locales/zh.yaml?raw'
import { size } from './format.ts'
import { renderMarkdown } from './markdown.ts'
import { datetimeSpan } from './datetime.ts'
import { difficulty } from './difficulty.ts'
import {
  STATUS, STATUS_CODES, STATUS_SHORT_TEXTS, STATUS_TEXTS, getScoreColor,
} from './status.ts'

type TranslationMap = Record<string, string>
type RenderContext = Record<string, unknown>

const templateFiles: Record<string, string> = {
  ...import.meta.glob('/node_modules/@hydrooj/ui-default/templates/**/*.html', {
    query: '?raw', import: 'default', eager: true,
  }),
  ...import.meta.glob('/node_modules/@hydrooj/ui-default/templates/**/*.yaml', {
    query: '?raw', import: 'default', eager: true,
  }),
  ...import.meta.glob('/node_modules/@hydrooj/ui-default/templates/**/*.json', {
    query: '?raw', import: 'default', eager: true,
  }),
  ...import.meta.glob('/node_modules/@hydrooj/ui-default/templates/**/*.md', {
    query: '?raw', import: 'default', eager: true,
  }),
} as Record<string, string>

const prefix = '/node_modules/@hydrooj/ui-default/templates/'
const sources = new Map<string, string>(
  Object.entries(templateFiles).map(([path, source]) => [path.slice(prefix.length), source]),
)

// The app already owns the shared <nav>/<footer> shell. Page templates continue
// to extend the upstream layout so their block structure stays untouched, but
// this layout only exposes the content block.
sources.set('layout/basic.html', '{% block content %}{% endblock %}')
sources.set('layout/immersive.html', '{% block content %}{% endblock %}')
sources.set('layout/html5.html', '{% block body %}{% block content %}{% endblock %}{% endblock %}')

const translations = yaml.load(zhYaml) as TranslationMap

const loader = {
  async: false,
  getSource(name: string) {
    const src = sources.get(name)
    if (src === undefined) throw new Error(`Missing Hydro template: ${name}`)
    return { src, path: name, noCache: true }
  },
}

const env = new nunjucks.Environment(loader as unknown as nunjucks.ILoader, {
  autoescape: true,
  noCache: true,
  throwOnUndefined: false,
})

env.addFilter('assign', (base: RenderContext, extra: RenderContext) => ({ ...base, ...extra }))
env.addFilter('nl2br', (value: string) => String(value).replace(/\n/g, '<br>'))
env.addFilter('json', (value: unknown) => JSON.stringify(value))
env.addFilter('parseYaml', (value: string) => yaml.load(String(value)))
env.addFilter('toString', (value: unknown) => String(value ?? ''))
env.addFilter('contentLang', () => ['zh'])
env.addFilter('content', (value: unknown) => value)
env.addFilter('ansi', (value: unknown) => String(value ?? '')
  .replace(/\x1b\[[0-9;]*[A-Za-z]/g, '')
  .replace(/&/g, '&amp;')
  .replace(/</g, '&lt;')
  .replace(/>/g, '&gt;'))
env.addFilter('problemPreview', (value: unknown) => String(value ?? '')
  .replace(/```[\s\S]*?```/g, ' ')
  .replace(/[#>*_`\[\]]/g, ' ')
  .replace(/\s+/g, ' ')
  .trim())
env.addFilter('markdown', (value: unknown) => renderMarkdown(String(value ?? '')))


function paginate(page: number, numPages: number) {
  const result: Array<[string, number]> = []
  const radius = 5
  let first: number
  let last: number
  if (page > 1) {
    if (page > 2) result.push(['first', 1])
    result.push(['previous', page - 1])
  }
  if (page <= radius) {
    first = 1
    last = Math.min(1 + radius * 2, numPages)
  } else if (page >= numPages - radius) {
    first = Math.max(1, numPages - radius * 2)
    last = numPages
  } else {
    first = page - radius
    last = page + radius
  }
  if (first > 1) result.push(['ellipsis', 0])
  for (let current = first; current < last + 1; current++) {
    result.push([current !== page ? 'page' : 'current', current])
  }
  if (last < numPages) result.push(['ellipsis', 0])
  if (page < numPages) {
    result.push(['next', page + 1])
    if (page < numPages - 1) result.push(['last', numPages])
  }
  return result
}

export function translate(key: string) {
  return String(translations[key] ?? key)
}

function defaultUrl(name: string, args: Record<string, unknown> = {}) {
  const id = String(args.tid ?? args.pid ?? '')
  const routes: Record<string, string> = {
    contest_create: '/contest/new',
    homework_create: '/homework/new',
    training_create: '/training/new',
    contest_edit: `/contest/${encodeURIComponent(id)}/edit`,
    homework_edit: `/homework/${encodeURIComponent(id)}/edit`,
    training_edit: `/training/${encodeURIComponent(id)}/edit`,
    contest_detail: `/contest/${encodeURIComponent(id)}`,
    homework_detail: `/homework/${encodeURIComponent(id)}`,
    training_detail: `/training/${encodeURIComponent(id)}`,
    training_files: `/training/${encodeURIComponent(id)}/files`,
    problem_detail: `/p/${encodeURIComponent(String(args.pid ?? ''))}`,
    record_detail: `/record/${encodeURIComponent(String(args.rid ?? ''))}`,
    user_detail: `/user/${encodeURIComponent(String(args.uid ?? ''))}`,
    wiki_help: '/discuss',
  }
  return routes[name] ?? '/'
}

export type HydroRenderOptions = {
  context?: RenderContext
  url?: (name: string, args?: Record<string, unknown>, query?: Record<string, unknown>) => string
  urlForFile?: (filename: string, filetype?: string) => string
}

export function renderHydroTemplate(template: string, options: HydroRenderOptions = {}) {
  const base: RenderContext = {
    _: translate,
    url: options.url ?? defaultUrl,
    urlForFile: options.urlForFile ?? ((filename: string) => filename),
    size,
    isIE: () => false,
    Object,
    JSON,
    Math,
    paginate,
    typeof: (value: unknown) => typeof value,
    eval: (code: string) => {
      if (code.replace(/\s/g, '') === 'rdoc=>rdoc._id.toString()') return (rdoc: { _id: unknown }) => String(rdoc._id)
      return () => undefined
    },
    set: (target: RenderContext, keyOrValues: string | RenderContext, value?: unknown) => {
      if (typeof keyOrValues === 'string') target[keyOrValues] = value
      else Object.assign(target, keyOrValues)
      return ''
    },
    STATUS,
    datetimeSpan,
    findSubModule: () => [],
    templateExists: (name: string) => sources.has(name),
    model: { builtin: { STATUS_CODES, STATUS_TEXTS, STATUS_SHORT_TEXTS } },
    utils: {
      status: { getScoreColor },
      getAlphabeticId: (index: number) => String.fromCharCode(65 + index),
    },
    lib: { difficulty },
  }
  const extra = options.context ?? {}
  const context: RenderContext = {
    ...base,
    ...extra,
    model: { ...(base.model as RenderContext), ...(extra.model as RenderContext | undefined) },
    utils: { ...(base.utils as RenderContext), ...(extra.utils as RenderContext | undefined) },
  }
  return env.renderString(
    `{% import "components/user.html" as user with context %}{% import "components/problem.html" as problem with context %}{% import "components/nothing.html" as nothing with context %}{% import "components/record.html" as record with context %}{% import "components/paginator.html" as paginator with context %}{% import "components/form.html" as form with context %}{% import "components/sidemenu.html" as sidemenu with context %}{% import "components/noscript_note.html" as noscript_note with context %}{% include "${template}" %}`,
    context,
  )
}
