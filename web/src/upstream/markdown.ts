// @ts-nocheck -- the markdown-it ecosystem plugins do not ship compatible types.
// Keep this plugin chain aligned with Hydro ui-default/components/editor/mdeditor.ts.
import MarkdownIt from 'markdown-it'
import Anchor from 'markdown-it-anchor'
import Footnote from 'markdown-it-footnote'
import Mark from 'markdown-it-mark'
import MergeCells from 'markdown-it-merge-cells'
import TOC from 'markdown-it-table-of-contents'
import Imsize from '../legacy/markdown-it-imsize.ts'
import katex from '../legacy/markdown-it-katex.ts'
import { Media } from '../legacy/markdown-it-media.ts'
import { xssProtector } from '../legacy/markdown-it-xss.ts'

const markdown = new MarkdownIt({ html: true, linkify: true })
markdown.linkify.tlds('.py', false)
markdown.linkify.tlds('.zip', false)
markdown.linkify.tlds('.mov', false)
markdown.use(Media)
markdown.use(Footnote)
markdown.use(Mark)
markdown.use(Imsize)
markdown.use(Anchor)
markdown.use(TOC)
markdown.use(MergeCells)
markdown.use(xssProtector)
markdown.use(katex)

export function renderMarkdown(source: string) {
  return markdown.render(source || '')
}
