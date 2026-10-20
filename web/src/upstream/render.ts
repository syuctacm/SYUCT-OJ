// The page templates and Chinese translations are loaded verbatim from the
// pinned @hydrooj/ui-default package. React supplies data and browser events;
// the markup itself must stay identical to Hydro.
import nunjucks from 'nunjucks'
import yaml from 'js-yaml'
import zhYaml from '@hydrooj/ui-default/locales/zh.yaml?raw'
import { size } from './format.ts'

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

function translate(key: string) {
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
  }
  return routes[name] ?? '/'
}

export type HydroRenderOptions = {
  context?: RenderContext
  url?: (name: string, args?: Record<string, unknown>, query?: Record<string, unknown>) => string
  urlForFile?: (filename: string, filetype?: string) => string
}

export function renderHydroTemplate(template: string, options: HydroRenderOptions = {}) {
  const context: RenderContext = {
    _: translate,
    url: options.url ?? defaultUrl,
    urlForFile: options.urlForFile ?? ((filename: string) => filename),
    size,
    isIE: () => false,
    set: (target: RenderContext, keyOrValues: string | RenderContext, value?: unknown) => {
      if (typeof keyOrValues === 'string') target[keyOrValues] = value
      else Object.assign(target, keyOrValues)
      return ''
    },
    ...options.context,
  }
  return env.renderString(
    `{% import "components/form.html" as form with context %}{% import "components/noscript_note.html" as noscript_note with context %}{% include "${template}" %}`,
    context,
  )
}
