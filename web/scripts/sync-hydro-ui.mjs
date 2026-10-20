import { readFile, writeFile } from 'node:fs/promises'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))
const webRoot = resolve(here, '..')
const source = resolve(webRoot, 'node_modules/@hydrooj/ui-default/components/editor/mdeditor.ts')
const target = resolve(webRoot, 'src/upstream/hydro-editor/mdeditor.ts')

let code = await readFile(source, 'utf8')
code = code
  .replace("from '../../backendlib/markdown-it-imsize'", "from '../../legacy/markdown-it-imsize.ts'")
  .replace("from '../../backendlib/markdown-it-katex'", "from '../../legacy/markdown-it-katex.ts'")
  .replace("from '../../backendlib/markdown-it-media'", "from '../../legacy/markdown-it-media.ts'")
  .replace("from '../../backendlib/markdown-it-xss'", "from '../../legacy/markdown-it-xss.ts'")

const header = '// @ts-nocheck -- vendored from @hydrooj/ui-default 4.58.5; only import paths are rewritten.\n'
await writeFile(target, header + code)
console.log(`synced ${target}`)
