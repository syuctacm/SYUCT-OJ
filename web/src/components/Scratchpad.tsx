import { Allotment } from 'allotment'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  createPretest,
  createSubmission,
  getSubmission,
  getSubmissions,
  type ProblemDetail,
  type Submission,
  type User,
} from '../api.ts'
import { statusClass, statusLabels } from '../status.ts'
import { languageOptions } from './autocomplete/LanguageSelectAutoComplete.tsx'
import { notifyError, notifySuccess } from './Notification.tsx'
import { loadLanguages, type LanguageItem } from '../upstream/languages.ts'
import { monaco } from '../upstream/monaco/base.ts'
import { getScratchpadDraft, putScratchpadDraft } from '../upstream/scratchpad-drafts.ts'
import { subscribeScratchpadPretest } from '../upstream/scratchpad-pretest.ts'

type ScratchpadProps = {
  problem: ProblemDetail
  user: User
  contestId?: string
  canViewRecord: boolean
  onExit: () => void
  onSubmitted: (submission: Submission) => void
}

type PanelName = 'problem' | 'settings'

const terminalStatuses = new Set([1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 30])

function scratchpadStatus(record: Submission) {
  const lines = [`${statusLabels[record.status] ?? record.status} ${record.time}ms ${record.memory}KiB`]
  for (const text of record.compilerTexts ?? []) lines.push(text)
  for (const text of record.judgeTexts ?? []) {
    if (typeof text === 'string') lines.push(text)
    else if (text && typeof text === 'object' && 'message' in text) lines.push(String(text.message ?? ''))
  }
  for (const testCase of record.testCases ?? []) {
    if (testCase && typeof testCase === 'object' && 'message' in testCase && testCase.message) {
      lines.push(String(testCase.message))
      break
    }
  }
  return lines.filter(Boolean).join('\n')
}

function ProblemPanel() {
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const content = document.querySelector<HTMLElement>('.problem-content')
    const host = ref.current
    if (!content || !host) return
    const parent = content.parentElement
    const next = content.nextSibling
    host.appendChild(content)
    return () => {
      if (parent) parent.insertBefore(content, next)
    }
  }, [])
  return <div ref={ref} className="scratchpad-problem-content" />
}

function SettingsPanel({ fontSize, tabSize, theme }: {
  fontSize: number
  tabSize: number
  theme: 'vs-light' | 'vs-dark'
}) {
  const [font, setFont] = useState(fontSize)
  const [tab, setTab] = useState(tabSize)
  const [editorTheme, setEditorTheme] = useState(theme)
  useEffect(() => setFont(fontSize), [fontSize])
  useEffect(() => setTab(tabSize), [tabSize])
  useEffect(() => setEditorTheme(theme), [theme])
  return (
    <div className="section__body">
      <div className="row"><div className="medium-6 columns">
        <label>字体大小:
          <div className="textbox-container">
            <input
              type="number"
              className="textbox"
              value={font}
              onChange={(event) => {
                const value = Number(event.target.value)
                setFont(value)
                window.dispatchEvent(new CustomEvent('scratchpad-settings', { detail: { fontSize: value } }))
              }}
            />
          </div>
        </label>
      </div></div>
      <div className="row"><div className="medium-6 columns">
        <label>Tab 宽度:
          <div className="textbox-container">
            <input
              type="number"
              className="textbox"
              value={tab}
              onChange={(event) => {
                const value = Number(event.target.value)
                setTab(value)
                window.dispatchEvent(new CustomEvent('scratchpad-settings', { detail: { tabSize: value } }))
              }}
            />
          </div>
        </label>
      </div></div>
      <div className="row"><div className="columns">
        <label>编辑器主题:
          <div className="select-container">
            <select
              className="select"
              value={editorTheme}
              onChange={(event) => {
                const value = event.target.value as 'vs-light' | 'vs-dark'
                setEditorTheme(value)
                window.dispatchEvent(new CustomEvent('scratchpad-settings', { detail: { theme: value } }))
              }}
            >
              <option value="vs-light">Light</option>
              <option value="vs-dark">Dark</option>
            </select>
          </div>
        </label>
      </div></div>
    </div>
  )
}

function ToolbarButton(props: {
  active?: boolean
  disabled?: boolean
  className?: string
  hotkey?: string
  title?: string
  onClick: () => void
  children: React.ReactNode
}) {
  const classes = [
    'scratchpad__toolbar__item',
    'scratchpad__toolbar__button',
    props.active ? 'activated' : '',
    props.disabled ? 'disabled' : 'enabled',
    props.className ?? '',
  ].filter(Boolean).join(' ')
  return (
    <button
      type="button"
      tabIndex={-1}
      className={classes}
      disabled={props.disabled}
      data-global-hotkey={props.hotkey}
      data-tooltip={props.title}
      onClick={() => { if (!props.disabled) props.onClick() }}
    >
      <div>{props.children}</div>
    </button>
  )
}

export function Scratchpad({ problem, user, contestId = '', canViewRecord, onExit, onSubmitted }: ScratchpadProps) {
  const [code, setCode] = useState(user.codeTemplate ?? '')
  const [language, setLanguage] = useState(user.codeLang || 'cpp')
  const [languages, setLanguages] = useState<LanguageItem[]>(languageOptions)
  const [input, setInput] = useState('')
  const [output, setOutput] = useState('')
  const [pretestVisible, setPretestVisible] = useState(localStorage.getItem('scratchpad/pretest') === 'true')
  const [recordsVisible, setRecordsVisible] = useState(canViewRecord && localStorage.getItem('scratchpad/records') === 'true')
  const [activePage, setActivePage] = useState<PanelName | null>('problem')
  const [posting, setPosting] = useState(false)
  const [pretestBusy, setPretestBusy] = useState(false)
  const [records, setRecords] = useState<Submission[]>([])
  const [recordsLoading, setRecordsLoading] = useState(false)
  const savedEditorConfig = useMemo(() => {
    try {
      return JSON.parse(localStorage.getItem('editor.config') || '{}') as {
        fontSize?: number
        tabSize?: number
        theme?: 'vs-light' | 'vs-dark'
      }
    } catch {
      return {}
    }
  }, [])
  const [fontSize, setFontSize] = useState(savedEditorConfig.fontSize ?? 14)
  const [tabSize, setTabSize] = useState(savedEditorConfig.tabSize ?? 4)
  const [theme, setTheme] = useState<'vs-light' | 'vs-dark'>(
    savedEditorConfig.theme
      ?? (document.documentElement.dataset.mantineColorScheme === 'dark' ? 'vs-dark' : 'vs-light'),
  )
  const editorHost = useRef<HTMLDivElement>(null)
  const editorRef = useRef<monaco.editor.IStandaloneCodeEditor | null>(null)
  const modelRef = useRef<monaco.editor.ITextModel | null>(null)
  const codeRef = useRef(code)
  const draftReady = useRef(false)
  const pollCancelled = useRef(false)
  const cacheKey = `${user.id}/${problem.contest?.docId ?? 'system'}/${problem.docId}${contestId ? `@${contestId}` : ''}`

  codeRef.current = code

  useEffect(() => {
    void loadLanguages().then((items) => {
      if (!items.length) return
      setLanguages(items)
      if (!items.some((item) => item._id === language)) {
        const next = user.codeLang && items.some((item) => item._id === user.codeLang) ? user.codeLang : items[0]._id
        setLanguage(next)
      }
    })
  }, [language, user.codeLang])

  useEffect(() => {
    void getScratchpadDraft(cacheKey).then((draft) => {
      if (draft) {
        setCode(draft.code)
        setLanguage(draft.lang)
      }
      draftReady.current = true
    })
  }, [cacheKey])

  useEffect(() => {
    if (!draftReady.current) return
    const timer = window.setTimeout(() => {
      void putScratchpadDraft({ id: cacheKey, code, lang: language })
    }, 400)
    return () => window.clearTimeout(timer)
  }, [cacheKey, code, language])

  useEffect(() => {
    const host = editorHost.current
    if (!host) return
    const model = monaco.editor.createModel(codeRef.current, language, monaco.Uri.parse(`syuct-oj://scratchpad/${cacheKey}.${language}`))
    const editor = monaco.editor.create(host, {
      model,
      theme,
      fontFamily: globalThis.UserContext?.codeFontFamily || 'var(--code-font-family, "Source Code Pro", Consolas, monospace)',
      fontSize,
      tabSize,
      automaticLayout: true,
      minimap: { enabled: false },
      scrollBeyondLastLine: false,
      lineNumbers: 'on',
      glyphMargin: true,
      fontLigatures: '',
    })
    const subscription = editor.onDidChangeModelContent(() => {
      if (editor.getValue() !== codeRef.current) setCode(editor.getValue())
    })
    editorRef.current = editor
    modelRef.current = model
    return () => {
      subscription.dispose()
      editor.dispose()
      model.dispose()
      editorRef.current = null
      modelRef.current = null
    }
  }, [cacheKey])

  useEffect(() => {
    const editor = editorRef.current
    if (editor && editor.getValue() !== code) editor.setValue(code)
  }, [code])

  useEffect(() => {
    const model = modelRef.current
    if (model && model.getLanguageId() !== language) monaco.editor.setModelLanguage(model, language)
  }, [language])

  useEffect(() => {
    editorRef.current?.updateOptions({ fontSize, tabSize, theme })
  }, [fontSize, tabSize, theme])

  useEffect(() => {
    localStorage.setItem('editor.config', JSON.stringify({ fontSize, tabSize, theme }))
  }, [fontSize, tabSize, theme])

  useEffect(() => {
    const listener = (event: Event) => {
      const detail = (event as CustomEvent<{ fontSize?: number; tabSize?: number; theme?: 'vs-light' | 'vs-dark' }>).detail
      if (detail.fontSize) setFontSize(detail.fontSize)
      if (detail.tabSize) setTabSize(detail.tabSize)
      if (detail.theme) setTheme(detail.theme)
    }
    window.addEventListener('scratchpad-settings', listener)
    return () => window.removeEventListener('scratchpad-settings', listener)
  }, [])

  useEffect(() => subscribeScratchpadPretest((value) => {
    setInput(value)
    setPretestVisible(true)
    localStorage.setItem('scratchpad/pretest', 'true')
  }), [])

  useEffect(() => () => { pollCancelled.current = true }, [])

  useEffect(() => {
    if (!recordsVisible) return
    setRecordsLoading(true)
    getSubmissions({
      pid: String(problem.docId),
      uidOrName: String(user.id),
      tid: contestId || undefined,
      limit: 20,
    })
      .then((page) => setRecords(page.items))
      .catch((err: unknown) => notifyError(err instanceof Error ? err.message : '记录加载失败'))
      .finally(() => setRecordsLoading(false))
  }, [contestId, problem.docId, recordsVisible, user.id])

  const runPretest = useCallback(async () => {
    if (posting || pretestBusy) return
    setPretestBusy(true)
    setPretestVisible(true)
    localStorage.setItem('scratchpad/pretest', 'true')
    setOutput('提交预测试...')
    pollCancelled.current = false
    try {
      let record = await createPretest(problem.pid, language, code, [input], contestId)
      const deadline = Date.now() + 60_000
      while (!terminalStatuses.has(record.status) && Date.now() < deadline && !pollCancelled.current) {
        await new Promise((resolve) => window.setTimeout(resolve, 500))
        record = await getSubmission(record.id)
      }
      if (!terminalStatuses.has(record.status)) throw new Error('预测试超时')
      setOutput(scratchpadStatus(record))
    } catch (err) {
      const message = err instanceof Error ? err.message : '预测试失败'
      setOutput(message)
      notifyError(message)
    } finally {
      setPretestBusy(false)
    }
  }, [code, contestId, input, language, posting, pretestBusy, problem.pid])

  const submit = useCallback(async () => {
    if (posting) return
    setPosting(true)
    try {
      const submission = await createSubmission(problem.pid, language, code, contestId)
      notifySuccess('已提交。')
      onSubmitted(submission)
    } catch (err) {
      notifyError(err instanceof Error ? err.message : '提交失败')
    } finally {
      setPosting(false)
    }
  }, [code, contestId, language, onSubmitted, posting, problem.pid])

  const togglePanel = (panel: 'pretest' | 'records') => {
    if (panel === 'pretest') {
      setPretestVisible((value) => {
        localStorage.setItem('scratchpad/pretest', String(!value))
        return !value
      })
    } else {
      setRecordsVisible((value) => {
        localStorage.setItem('scratchpad/records', String(!value))
        return !value
      })
    }
  }

  const canPretest = (problem.problemType || 'default') === 'default'
  const visibleLanguages = useMemo(() => (
    languages.length ? languages : languageOptions
  ), [languages])

  return (
    <Allotment onChange={() => editorRef.current?.layout()}>
      <Allotment.Pane visible minSize={50} maxSize={50}>
        <div className="scratchpad__tablist">
          <div
            className={`scratchpad__tabicon-problem${activePage === 'problem' ? ' scratchpad__tab-active' : ''}`}
            onClick={() => setActivePage((value) => value === 'problem' ? null : 'problem')}
          >
            <i className="icon icon-file" />
          </div>
          <div
            className={`scratchpad__tabicon-settings${activePage === 'settings' ? ' scratchpad__tab-active' : ''}`}
            onClick={() => setActivePage((value) => value === 'settings' ? null : 'settings')}
          >
            <i className="icon icon-wrench" />
          </div>
        </div>
      </Allotment.Pane>
      <Allotment.Pane visible={activePage !== null}>
        <div className="scratchpad__problem">
          <div style={{ display: activePage === 'problem' ? 'block' : 'none', height: '100%' }}>
            <ProblemPanel />
          </div>
          <div style={{ display: activePage === 'settings' ? 'block' : 'none', height: '100%' }}>
            <SettingsPanel fontSize={fontSize} tabSize={tabSize} theme={theme} />
          </div>
        </div>
      </Allotment.Pane>
      <Allotment vertical onChange={() => editorRef.current?.layout()}>
        <div className="flex-col splitpane-fill">
          <div className="scratchpad__toolbar flex-row flex-cross-center">
            {canPretest ? (
              <ToolbarButton
                disabled={posting || pretestBusy}
                className="scratchpad__toolbar__pretest"
                hotkey="f9"
                title="预测试代码 (F9)"
                onClick={() => void runPretest()}
              >
                <i className="icon icon-debug" /> {pretestBusy ? '运行中...' : '运行预测试'} (F9)
              </ToolbarButton>
            ) : null}
            <ToolbarButton
              disabled={posting}
              className="scratchpad__toolbar__submit"
              hotkey="f10"
              title="提交代码 (F10)"
              onClick={() => void submit()}
            >
              <i className="icon icon-play" /> {posting ? '提交中...' : '提交'} (F10)
            </ToolbarButton>
            <ToolbarButton hotkey="alt+q" title="退出草稿纸 (Alt+Q)" onClick={onExit}>
              <i className="icon icon-close" /> 退出 (Alt+Q)
            </ToolbarButton>
            <div className="scratchpad__toolbar__item">
              <select
                className="select"
                disabled={posting}
                value={language}
                onChange={(event) => setLanguage(event.target.value)}
              >
                {visibleLanguages.map((item) => <option key={item._id} value={item._id}>{item.name}</option>)}
              </select>
            </div>
            <div className="scratchpad__toolbar__item scratchpad__toolbar__split" />
            {canPretest ? (
              <ToolbarButton
                active={pretestVisible}
                hotkey="alt+p"
                title="切换预测试面板 (Alt+P)"
                onClick={() => togglePanel('pretest')}
              >
                <i className="icon icon-edit" /> 预测试
              </ToolbarButton>
            ) : null}
            {canViewRecord ? (
              <ToolbarButton
                active={recordsVisible}
                hotkey="alt+r"
                title="切换记录面板 (Alt+R)"
                onClick={() => togglePanel('records')}
              >
                <i className="icon icon-flag" /> 记录
              </ToolbarButton>
            ) : null}
          </div>
          <div ref={editorHost} className="ScratchpadMonacoEditor" style={{ height: '100%', width: '100%' }} />
        </div>
        <Allotment.Pane visible={pretestVisible}>
          <div className="flex-col splitpane-fill">
            <div className="scratchpad__panel-title"><i className="icon icon-edit" /> 预测试</div>
            <div className="flex-col flex-fill">
              <Allotment>
                <textarea
                  className="scratchpad__data-input"
                  wrap="off"
                  spellCheck={false}
                  value={input}
                  onChange={(event) => setInput(event.target.value)}
                  placeholder="输入"
                />
                <pre className="scratchpad__data-input" style={{ margin: 0, overflow: 'auto' }}>{output}</pre>
              </Allotment>
            </div>
          </div>
        </Allotment.Pane>
        <Allotment.Pane visible={recordsVisible}>
          <div className="flex-col splitpane-fill">
            <div className="scratchpad__panel-title"><i className="icon icon-flag" /> 记录</div>
            <table className={`data-table is--full-row scratchpad__records__table${recordsLoading ? ' loading' : ''}`}>
              <colgroup>
                <col className="col--detail" />
                <col className="col--memory" />
                <col className="col--time" />
                <col className="col--at" />
              </colgroup>
              <tbody>
                {records.map((record) => (
                  <tr
                    key={record.id}
                    onClick={() => {
                      const query = contestId ? `?tid=${encodeURIComponent(contestId)}` : ''
                      window.location.href = `/record/${record.id}${query}`
                    }}
                  >
                    <td className={`col--detail record-status--border ${statusClass(record.status)}`}>
                      <span className={`icon record-status--icon ${statusClass(record.status)}`} />{' '}
                      <span className={`record-status--text ${statusClass(record.status)}`}>
                        {statusLabels[record.status] ?? record.status}
                      </span>
                    </td>
                    <td className="col--memory">{record.memory ? `${Math.ceil(record.memory / 1000)} MB` : '-'}</td>
                    <td className="col--time">{record.time ? `${(record.time / 1000).toFixed(1)}s` : '-'}</td>
                    <td className="col--at"><time>{new Date(record.createdAt).toLocaleString('zh-CN', { hour12: false })}</time></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Allotment.Pane>
      </Allotment>
    </Allotment>
  )
}
