// The page templates and Chinese translations are loaded verbatim from the
// pinned @hydrooj/ui-default package. React supplies data and browser events;
// the markup itself must stay identical to Hydro.
import nunjucks from 'nunjucks'
import yaml from 'js-yaml'
import zhYaml from '@hydrooj/ui-default/locales/zh.yaml?raw'
import tagStatTemplate from './templates/problem_tag_stat.html?raw'
import problemMainTemplate from './templates/problem_main.html?raw'
import tagStatCategoryTemplate from './templates/partials/category.html?raw'
import checkinHomeTemplate from './templates/partials/homepage/checkin.html?raw'
import countdownHomeTemplate from './templates/partials/homepage/countdown.html?raw'
import navTemplate from './templates/partials/nav.html?raw'
import headerMobileTemplate from './templates/partials/header_mobile.html?raw'
import footerTemplate from './templates/partials/footer.html?raw'
import { size } from './format.ts'
import manageAuditTemplate from './templates/manage_audit.html?raw'
import { renderMarkdown } from './markdown.ts'
import { contentLanguages, selectLocalizedContent } from './problem-content.ts'
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

// The tag-stat addon pages are copied verbatim from the local production plugin.
sources.set('problem_tag_stat.html', tagStatTemplate)
sources.set('problem_main.html', problemMainTemplate)
sources.set('partials/category.html', tagStatCategoryTemplate)
sources.set('partials/homepage/checkin.html', checkinHomeTemplate)
sources.set('partials/homepage/countdown.html', countdownHomeTemplate)
sources.set('partials/nav.html', navTemplate)
sources.set('partials/header_mobile.html', headerMobileTemplate)
sources.set('partials/footer.html', footerTemplate)
sources.set('manage_audit.html', manageAuditTemplate)

// The app already owns the shared <nav>/<footer> shell. Page templates continue
// to extend the upstream layout so their block structure stays untouched, but
// this layout only exposes the content block.
sources.set('layout/basic.html', '{% block content %}{% endblock %}')
sources.set('layout/immersive.html', '{% block content %}{% endblock %}')
sources.set('layout/html5.html', '{% block body %}{% block content %}{% endblock %}{% endblock %}')

// Single-site deployment: the upstream account sidebar links to domain
// settings, which this rewrite dropped. The rest of the macro is upstream.
sources.set('components/home.html', `{% import "components/sidemenu.html" as sidemenu with context %}
{% macro render_sidebar() %}
<div class="section side">
  <ol class="menu">
    {{ sidemenu.render_item('account--circle', 'user_detail', {uid:handler.user._id}, label='My Profile') }}
    {{ sidemenu.render_item('comment--multiple', 'home_messages') }}
    <li class="menu__seperator"></li>
    {% if handler.user.hasPriv(PRIV.PRIV_CREATE_FILE) %}
      {{ sidemenu.render_item('file', 'home_files', label='My Files') }}
      <li class="menu__seperator"></li>
    {% endif %}
    {{ sidemenu.render_item('wrench', 'home_settings', {category:'account'}, label='Account Settings', menu_item='home_account') }}
    {{ sidemenu.render_item('sliders', 'home_settings', {category:'preference'}, label='Preference Settings', menu_item='home_preference')}}
    {{ sidemenu.render_item('security', 'home_security') }}
  </ol>
</div>
{% endmacro %}`)

// fps-importer ships its page outside the ui-default package; the template is
// copied verbatim from packages/fps-importer/template/problem_import_fps.html.
sources.set('problem_import_fps.html', `{% extends "layout/basic.html" %}
{% block content %}
<div class="row">
  <div class="medium-9 columns">
    <div class="section">
      <div class="section__body">
        <form method="post" enctype="multipart/form-data">
          <p>{{ _('Upload fps file') }}:
            <input type="file" name="file">
            <input type="submit" value="{{ _('Upload') }}" class="rounded primary button">
          </p>
          <p>{{ _('problem.import.fps.hint1') }}</p>
          <p>{{ _('problem.import.fps.hint2') }}</p>
          <p>{{ _('problem.import.fps.hint3') }}</p>
        </form>
      </div>
    </div>
  </div>
  <div class="medium-3 columns">
    <div class="section side">
      <div class="section__body typo">
        <h2>{{ _('What is this?') }}</h2>
        <p>{{ _('With this feature, you can import problems that you can view from a site to here. Their title, content, tags and categories will be imported.') }}</p>
      </div>
    </div>
  </div>
</div>
{% endblock %}`)

// Strings this rewrite adds on top of the pinned ui-default locale.
const LOCAL_TRANSLATIONS: TranslationMap = {
  setting_basic: '基本设置',
  manage_audit: '操作审计',
  'No audit records': '暂无审计记录',
  Bulletin: '站点公告',
  // fps-importer addon locale strings.
  'From FPS File': '从 FPS 文件导入',
  'Upload fps file': '上传 FPS 文件',
  'problem.import.fps.hint1': '我们推荐的最大导入大小为 64MiB，若文件超出此大小，强烈建议您在本机使用 EasyFPSViewer 等工具将其拆分或是移除测试数据后单独上传。',
  'problem.import.fps.hint2': '由于 xml 格式无法随机读写，解析需要消耗大量内存，在内存过小的机器上导入大型题目包很可能导致崩溃或死机。',
  'problem.import.fps.hint3': '若您确有需要，此限制可在系统设置中更改。我们建议您使用 Hydro 自带的 zip 格式存储或是交换题目。',
}
const translations = { ...(yaml.load(zhYaml) as TranslationMap), ...LOCAL_TRANSLATIONS }

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
env.addFilter('contentLang', (value: unknown) => contentLanguages(value))
env.addFilter('content', (value: unknown, language = '', html = false) => {
  const selected = selectLocalizedContent(value, String(language))
  return html ? selected : renderMarkdown(selected)
})
env.addFilter('bitand', (left: unknown, right: unknown) => Number(left) & Number(right))
env.addFilter('base64_encode', (value: unknown) => {
  const bytes = new TextEncoder().encode(String(value ?? ''))
  let binary = ''
  for (const byte of bytes) binary += String.fromCharCode(byte)
  return btoa(binary)
})
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
env.addFilter('markdownInline', (value: unknown) => renderMarkdown(String(value ?? ''))
  .replace(/^<p>/, '')
  .replace(/<\/p>\s*$/, ''))


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
    status: '/status',
    homework_create: '/homework/new',
    training_create: '/training/new',
    contest_edit: `/contest/${encodeURIComponent(id)}/edit`,
    homework_edit: `/homework/${encodeURIComponent(id)}/edit`,
    training_edit: `/training/${encodeURIComponent(id)}/edit`,
    contest_detail: `/contest/${encodeURIComponent(id)}`,
    homework_detail: `/homework/${encodeURIComponent(id)}`,
    training_detail: `/training/${encodeURIComponent(id)}`,
    homework_files: `/homework/${encodeURIComponent(id)}/file`,
    training_files: `/training/${encodeURIComponent(id)}/file`,
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
    // Globals provided by ui-default/backendlib/template.ts.
    Date,
    Object,
    JSON,
    Math,
    String,
    Array,
    instanceof: (value: unknown, constructor: unknown) => value instanceof (constructor as never),
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
    // Only the control panel nodes that this rewrite implements are advertised;
    // manage_base.html renders whatever getNodes('ControlPanel') returns.
    ui: {
      getNodes: (family: string) => (family === 'ControlPanel'
        ? [{ name: 'manage_dashboard' }, { name: 'manage_user_priv' }, { name: 'manage_user_import' }, { name: 'manage_setting' }, { name: 'manage_audit' }]
        : []),
    },
    STATUS,
    datetimeSpan,
    findSubModule: (prefix: string) => (prefix === 'partials/user_detail/' ? ['partials/user_detail/activity.html'] : []),
    templateExists: (name: string) => sources.has(name),
    model: {
      builtin: {
        STATUS_CODES, STATUS_TEXTS, STATUS_SHORT_TEXTS,
        // packages/common/status.ts
        USER_GENDER_MALE: 0,
        USER_GENDER_FEMALE: 1,
        USER_GENDER_OTHER: 2,
        USER_GENDER_RANGE: { 0: 'Boy ♂', 1: 'Girl ♀', 2: 'Other' },
        USER_GENDER_ICONS: { 0: '♂', 1: '♀', 2: '?' },
      },
    },
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
