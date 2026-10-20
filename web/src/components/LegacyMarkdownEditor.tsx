import { MdEditor } from '../upstream/hydro-editor/mdeditor.ts'
import { useId } from 'react'

type Props = {
  value: string
  placeholder?: string
  disabled?: boolean
  autoFocus?: boolean
  onChange: (value: string) => void
  onUploadImg?: (files: File[], callback: (urls: string[]) => void) => void | Promise<void>
  hideImageUpload?: boolean
}

export function LegacyMarkdownEditor({ value, placeholder, disabled, autoFocus, onChange, onUploadImg, hideImageUpload }: Props) {
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
        ...(hideImageUpload ? ['image' as const] : []),
      ]}
      onChange={onChange}
      onUploadImg={onUploadImg}
    />
  )
}
