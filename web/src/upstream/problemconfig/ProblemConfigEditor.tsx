// @ts-nocheck
import { diffLines } from 'diff'
import 'monaco-editor/min/vs/editor/editor.main.css'
import * as yaml from 'js-yaml'
import { isEqual } from 'lodash'
import { createWebWorker } from 'monaco-editor/internal/common/workers.js'
import { configureMonacoYaml } from 'monaco-yaml'
import { useEffect, useRef } from 'react'
import { useDispatch, useSelector } from 'react-redux'
import { monaco } from '../monaco/base.ts'
import schema from '../monaco/schema/problemconfig.ts'
import type { RootState } from './reducer/index.ts'
import type { ProblemConfigFile, TestCaseConfig } from './types.ts'


let yamlConfigured = false

function configureYaml() {
  if (yamlConfigured) return
  configureMonacoYaml({
    ...monaco,
    editor: { ...monaco.editor, createWebWorker },
  } as typeof monaco, {
    enableSchemaRequest: false,
    schemas: [{
      uri: 'hydro://problem/config.schema.json',
      fileMatch: ['*'],
      schema: schema as Record<string, unknown>,
    }],
  })
  yamlConfigured = true
}

const configKey = [
  'type', 'subType', 'target', 'score', 'time',
  'memory', 'filename', 'checker_type', 'checker', 'interactor',
  'manager', 'num_processes', 'multi_pass', 'validator', 'user_extra_files',
  'judge_extra_files', 'detail', 'outputs', 'redirect', 'cases',
  'subtasks', 'langs', 'key', 'time_limit_rate', 'memory_limit_rate',
]

const subtasksKey = [
  'time', 'memory', 'score', 'if', 'id',
  'type', 'cases',
]

const casesKey = ['time', 'memory', 'input', 'output']

export function configYamlFormat(config: ProblemConfigFile) {
  const formatConfig: ProblemConfigFile = {}
  configKey.forEach((key) => {
    if (config[key] !== undefined) {
      if (key === 'subType' && ['single', 'multi'].includes(String(config[key])) && config.type !== 'submit_answer') return
      if (key === 'checker_type' && config.type !== 'default') return
      if (key === 'checker'
        && (['default', 'strict'].includes(String(formatConfig.checker_type)) || !formatConfig.checker_type)) return
      if (key === 'interactor' && config.type !== 'interactive') return
      if (key === 'multi_pass' && (!Number.isInteger(config.multi_pass) || Number(config.multi_pass) <= 1 || Number(config.multi_pass) > 20)) return
      if (key === 'subtasks') {
        formatConfig[key] = []
        ;(config[key] as Array<Record<string, unknown>>).forEach((subtask) => {
          const formatSubtask: Record<string, unknown> = {}
          subtasksKey.forEach((subtaskKey) => {
            if (subtask && subtask[subtaskKey] !== undefined) {
              formatSubtask[subtaskKey] = subtask[subtaskKey]
            }
          })
          ;(formatConfig[key] as Array<Record<string, unknown>>).push(formatSubtask)
        })
      } else if (key === 'cases') {
        formatConfig[key] = []
        ;(config[key] as Array<Record<string, unknown>>).forEach((caseItem) => {
          const formatCase: TestCaseConfig = {
            time: '1000ms', memory: '256MB', input: '', output: '',
          }
          casesKey.forEach((caseKey) => {
            if (caseItem[caseKey] !== undefined) formatCase[caseKey] = String(caseItem[caseKey])
            else delete formatCase[caseKey]
          })
          ;(formatConfig[key] as TestCaseConfig[]).push(formatCase)
        })
      } else formatConfig[key] = config[key]
    }
  })
  if (formatConfig.type === 'objective') {
    Object.keys(formatConfig).filter((i) => !['type', 'answers'].includes(i)).forEach((i) => delete formatConfig[i])
    formatConfig.answers = config.answers || {}
  }
  Object.keys(formatConfig).filter((i) => i.startsWith('__')).forEach((i) => delete formatConfig[i])
  return formatConfig
}

function dumpConfig(config: ProblemConfigFile) {
  return yaml.dump(configYamlFormat(config), { noRefs: true, lineWidth: -1 })
}

export default function ProblemConfigEditor() {
  const config = useSelector((state: RootState) => state.config)
  const dispatch = useDispatch()
  const containerRef = useRef<HTMLDivElement>(null)
  const editorRef = useRef<monaco.editor.IStandaloneCodeEditor | null>(null)
  const modelRef = useRef<monaco.editor.ITextModel | null>(null)

  useEffect(() => {
    if (!containerRef.current) return
    configureYaml()
    const model = monaco.editor.createModel(
      dumpConfig(config),
      'yaml',
      monaco.Uri.parse(`hydro://problem/config-${Date.now()}.yaml`),
    )
    const editor = monaco.editor.create(containerRef.current, {
      model,
      theme: document.documentElement.dataset.mantineColorScheme === 'dark' ? 'vs-dark' : 'vs-light',
      automaticLayout: true,
      minimap: { enabled: false },
      lineNumbers: 'off',
      fontFamily: globalThis.UserContext?.codeFontFamily || 'var(--code-font-family, "Source Code Pro", Consolas, monospace)',
      scrollBeyondLastLine: false,
      fixedOverflowWidgets: true,
    })
    modelRef.current = model
    editorRef.current = editor
    const subscription = editor.onDidChangeModelContent(() => {
      dispatch({ type: 'CONFIG_CODE_UPDATE', payload: editor.getValue() })
    })
    return () => {
      subscription.dispose()
      editor.dispose()
      model.dispose()
      editorRef.current = null
      modelRef.current = null
    }
    // The initial editor is intentionally constructed once. Later config
    // changes are reconciled by the effect below.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [dispatch])

  useEffect(() => {
    const editor = editorRef.current
    const model = modelRef.current
    if (!editor || !model || config.__valid === false) return
    const pending = dumpConfig(config)
    const current = model.getValue()
    try {
      if (isEqual(yaml.load(current), configYamlFormat(config))) return
    } catch {
      // Invalid YAML is handled by CONFIG_CODE_UPDATE and the Errors tab.
    }
    const operations: monaco.editor.IIdentifiedSingleEditOperation[] = []
    let cursor = 1
    for (const line of diffLines(current, pending)) {
      if (line.added) {
        const range = model.getFullModelRange().setStartPosition(cursor, 0).setEndPosition(cursor, 0)
        operations.push({ range, text: line.value })
      } else if (line.removed) {
        const range = model.getFullModelRange().setStartPosition(cursor, 0)
        cursor += line.count
        operations.push({ range: range.setEndPosition(cursor, 0), text: '' })
      } else {
        cursor += line.count
      }
    }
    model.pushEditOperations([], operations, undefined)
    editor.layout()
  }, [config])

  return <div ref={containerRef} className="ConfigMonacoEditor" style={{ minHeight: 500, height: '100%', width: '100%' }} />
}
