import { normalizeSubtasks, readSubtasksFromFiles } from '../upstream/problemconfig/subtask.ts'
import { MantineProvider } from '@mantine/core'
import { hydroMantineTheme } from '../upstream/mantine.ts'
import * as yaml from 'js-yaml'
import { useEffect, useRef, useState } from 'react'
import { Provider } from 'react-redux'
import { legacy_createStore as createStore, type Store } from 'redux'
import { updateProblem, type ProblemAssetList, type ProblemEditInput } from '../api.ts'
import ProblemConfig from '../upstream/problemconfig/index.tsx'
import { configYamlFormat } from '../upstream/problemconfig/ProblemConfigEditor.tsx'
import reducer from '../upstream/problemconfig/reducer/index.ts'

type ProblemConfigState = ReturnType<typeof reducer>
type ProblemConfigStore = Store<ProblemConfigState>

type Props = {
  pid: string
  assets: ProblemAssetList
  config: string
  onSaved: () => Promise<void> | void
}

function testdataForEditor(assets: ProblemAssetList) {
  return assets.testdata.map((file) => ({ ...file, _id: file.name }))
}

function initializeStore(assets: ProblemAssetList, config: string): ProblemConfigStore {
  const store = createStore(reducer)
  store.dispatch({
    type: 'CONFIG_LOAD_FULFILLED',
    payload: {
      config: config || '{}',
      testdata: testdataForEditor(assets),
    },
  })
  const state = store.getState()
  if (state.config.cases) {
    const cases = state.config.cases as Array<{ input: string; output: string }>
    const score = Number(state.config.score || 0) * cases.length
    store.dispatch({
      type: 'CONFIG_FORM_UPDATE',
      key: 'subtasks',
      value: [{
        type: 'sum',
        score: score && score < 100 ? score : 100,
        cases,
        id: 1,
      }],
    })
    store.dispatch({ type: 'CONFIG_FORM_UPDATE', key: 'cases', value: undefined })
    store.dispatch({ type: 'CONFIG_FORM_UPDATE', key: 'score', value: undefined })
  }
  const current = store.getState().config
  if (current.__valid !== false && !current.subtasks?.length) {
    const subtasks = readSubtasksFromFiles(
      testdataForEditor(assets).map((file) => file.name),
      current,
    )
    if (subtasks.length) {
      store.dispatch({
        type: 'CONFIG_AUTOCASES_UPDATE',
        subtasks: normalizeSubtasks(subtasks, (name) => name, current.time, current.memory, true),
      })
    }
  }
  return store
}

export function ProblemConfigPanel({ pid, assets, config, onSaved }: Props) {
  const storeRef = useRef<ProblemConfigStore | null>(null)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)

  if (!storeRef.current) {
    storeRef.current = initializeStore(assets, config)
  }
  const store = storeRef.current

  useEffect(() => {
    store.dispatch({ type: 'CONFIG_TESTDATA_REPLACE', value: testdataForEditor(assets) })
  }, [assets, store])

  async function save() {
    const state = store.getState().config
    if (state.__valid === false) {
      setError('配置存在错误，请先在 Errors 中修复后再保存')
      return
    }
    setSaving(true)
    setError('')
    const problem = assets.problem
    const input: ProblemEditInput = {
      pid: problem.pid,
      title: problem.title,
      content: problem.content || '',
      tags: problem.tags ?? [],
      difficulty: problem.difficulty ?? 0,
      hidden: Boolean(problem.hidden),
      config: yaml.dump(configYamlFormat(state), { noRefs: true, lineWidth: -1 }),
    }
    try {
      await updateProblem(pid, input)
      await onSaved()
    } catch (err) {
      setError(err instanceof Error ? err.message : '判题配置保存失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <MantineProvider theme={hydroMantineTheme}>
      {error ? <blockquote className="warn"><p>{error}</p></blockquote> : null}
      {saving ? <blockquote className="note"><p>正在保存判题配置...</p></blockquote> : null}
      <Provider store={store}>
        <ProblemConfig onSave={() => void save()} />
      </Provider>
    </MantineProvider>
  )
}
