// @ts-nocheck -- copied from Hydro ui-default; upstream aliases are resolved by its build.
import { useId } from 'react'
import Anchor from 'markdown-it-anchor'
import Footnote from 'markdown-it-footnote'
import Mark from 'markdown-it-mark'
import MergeCells from 'markdown-it-merge-cells'
import TOC from 'markdown-it-table-of-contents'
import { config, MdEditor } from 'md-editor-rt'
import Imsize from '../legacy/markdown-it-imsize.ts'
import katex from '../legacy/markdown-it-katex.ts'
import { Media } from '../legacy/markdown-it-media.ts'
import { xssProtector } from '../legacy/markdown-it-xss.ts'

config({
  markdownItConfig(mdit) {
    mdit.options.html = true
    mdit.options.linkify = true
    mdit.linkify.tlds('.py', false)
    mdit.linkify.tlds('.zip', false)
    mdit.linkify.tlds('.mov', false)
    mdit.use(Media)
    mdit.use(Footnote)
    mdit.use(Mark)
    mdit.use(Imsize)
    mdit.use(Anchor)
    mdit.use(TOC)
    mdit.use(MergeCells)
    mdit.use(xssProtector)
    mdit.use(katex)
  },
  markdownItPlugins() {
    return []
  },
  codeMirrorExtensions(extensions) {
    return extensions.filter((item) => item.type !== 'linkShortener')
  },
})

type Props = {
  value: string
  placeholder?: string
  disabled?: boolean
  autoFocus?: boolean
  onChange: (value: string) => void
}

export function LegacyMarkdownEditor({ value, placeholder, disabled, autoFocus, onChange }: Props) {
  const id = useId()
  return (
    <MdEditor
      className="textbox"
      id={`md-editor-${id.replace(/:/g, '')}`}
      value={value}
      placeholder={placeholder}
      disabled={disabled}
      autoFocus={autoFocus}
      codeTheme="github"
      codeStyleReverse={false}
      language="zh-CN"
      noMermaid
      noPrettier
      noKatex
      noHighlight
      autoDetectCode
      toolbarsExclude={[
        'sub',
        'sup',
        'mermaid',
        'save',
        'pageFullscreen',
        'fullscreen',
        'previewOnly',
        'htmlPreview',
        'github',
      ]}
      onChange={onChange}
    />
  )
}
