import * as monaco from 'monaco-editor/editor'
import EditorWorker from 'monaco-editor/editor/editor.worker?worker'
import JsonWorker from 'monaco-editor/language/json/json.worker?worker'
import YamlWorker from '../../workers/yaml.worker?worker'

const workers: Record<string, () => Worker> = {
  yaml: () => new YamlWorker(),
  json: () => new JsonWorker(),
}

;(self as unknown as { MonacoEnvironment: monaco.Environment }).MonacoEnvironment = {
  getWorker(_workerId: string, label: string) {
    return workers[label]?.() ?? new EditorWorker()
  },
}

export { monaco }
