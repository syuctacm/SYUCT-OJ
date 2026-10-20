import type { ChangeEventHandler, CSSProperties, ReactNode } from 'react'

type FormFieldProps = {
  columns?: number
  row?: boolean
  lastColumn?: boolean
  label: ReactNode
  helpText?: ReactNode
  children: ReactNode
}

function FormField({
  columns,
  row = true,
  lastColumn = true,
  label,
  helpText,
  children,
}: FormFieldProps) {
  const field = (
    <div className={`${columns ? `medium-${columns} ` : ''}columns form__item${lastColumn ? ' end' : ''}`}>
      <label>
        {label}
        {children}
      </label>
      {helpText ? <p className="help-text">{helpText}</p> : null}
    </div>
  )
  return row ? <div className="row">{field}</div> : field
}

type CommonFieldProps = Omit<FormFieldProps, 'children'> & {
  name: string
}

type FormTextProps = CommonFieldProps & {
  value: string | number
  placeholder?: string
  disabled?: boolean
  required?: boolean
  autoFocus?: boolean
  type?: string
  date?: boolean
  time?: boolean
  style?: CSSProperties
  onChange?: ChangeEventHandler<HTMLInputElement>
}

export function FormText({
  name,
  value,
  placeholder,
  disabled,
  required,
  autoFocus,
  type = 'text',
  date,
  time,
  style,
  onChange,
  ...field
}: FormTextProps) {
  const legacyName = { name: `form_item_${name}` }
  const pickerAttrs = {
    ...(date ? { 'data-pick-date': '' } : {}),
    ...(time ? { 'data-pick-time': '' } : {}),
  }
  return (
    <FormField {...field}>
      <div className="textbox-container" {...legacyName}>
        <input
          type={type}
          name={name}
          value={value}
          placeholder={placeholder}
          className="textbox"
          disabled={disabled}
          required={required}
          autoFocus={autoFocus}
          data-autofocus={autoFocus ? '' : undefined}
          style={style}
          onChange={onChange}
          {...pickerAttrs}
        />
      </div>
    </FormField>
  )
}

type FormTextareaProps = CommonFieldProps & {
  value: string
  placeholder?: string
  disabled?: boolean
  required?: boolean
  markdown?: boolean
  style?: CSSProperties
  onChange?: ChangeEventHandler<HTMLTextAreaElement>
}

export function FormTextarea({
  name,
  value,
  placeholder,
  disabled,
  required,
  markdown,
  style,
  onChange,
  ...field
}: FormTextareaProps) {
  const legacyName = { name: `form_item_${name}` }
  return (
    <FormField {...field}>
      <div className="textarea-container" {...legacyName}>
        <textarea
          name={name}
          placeholder={placeholder}
          className="textbox"
          disabled={disabled}
          required={required}
          style={style}
          data-markdown={markdown ? '' : undefined}
          value={value}
          onChange={onChange}
        />
      </div>
    </FormField>
  )
}

type FormSelectProps = CommonFieldProps & {
  value: string
  disabled?: boolean
  onChange?: ChangeEventHandler<HTMLSelectElement>
  children: ReactNode
}

export function FormSelect({ name, value, disabled, onChange, children, ...field }: FormSelectProps) {
  const legacyName = { name: `form_item_${name}` }
  return (
    <FormField {...field}>
      <div className="select-container" {...legacyName}>
        <select className="select" name={name} value={value} disabled={disabled} onChange={onChange}>
          {children}
        </select>
      </div>
    </FormField>
  )
}

type FormCheckboxProps = Omit<CommonFieldProps, 'children'> & {
  checked: boolean
  placeholder?: ReactNode
  disabled?: boolean
  onChange?: ChangeEventHandler<HTMLInputElement>
}

export function FormCheckbox({
  name,
  checked,
  placeholder,
  disabled,
  onChange,
  ...field
}: FormCheckboxProps) {
  const legacyName = { name: `form_item_${name}` }
  return (
    <FormField {...field}>
      <div className="checkbox-container" {...legacyName}>
        <label className="checkbox">
          <input
            type="checkbox"
            name={name}
            className="checkbox"
            checked={checked}
            disabled={disabled}
            onChange={onChange}
          />
          {placeholder}
        </label>
      </div>
    </FormField>
  )
}

export function MarkdownHint() {
  return (
    <div className="section side">
      <div className="section__body typo">
        <dl>
          <dt>行内代码</dt>
          <dd><pre>`std::cout()`</pre></dd>
          <dt>代码区块</dt>
          <dd><pre>{'```c++\nint main() {\n  return 0;\n}\n```'}</pre></dd>
          <dt>行内公式</dt>
          <dd><pre>$E=mc^2$</pre></dd>
          <dt>公式区块</dt>
          <dd><pre>$$E=mc^2$$</pre></dd>
          <dt>链接</dt>
          <dd><pre>[文本](https://...)</pre></dd>
          <dt>高亮</dt>
          <dd><pre>==文本==</pre></dd>
          <dt>图片</dt>
          <dd><pre>![说明](https://...)</pre></dd>
          <dt>指定大小的图片</dt>
          <dd><pre>![说明](url =100x100)</pre></dd>
          <dt>脚注</dt>
          <dd><pre>{'Here is a ref[^1].\n\n[^1]: And the footnote.'}</pre></dd>
          <dt>视频</dt>
          <dd><pre>@[video](https://.../a.mp4)</pre></dd>
          <dt>提及</dt>
          <dd><pre>@[](/user/uid)</pre></dd>
        </dl>
        <h2>了解更多</h2>
        <ul>
          <li><a href="http://commonmark.org/help/" target="_blank" rel="noreferrer"><span className="icon icon-link--external" /> CommonMark 语法</a></li>
          <li><a href="http://www.onemathematicalcat.org/MathJaxDocumentation/TeXSyntax.htm" target="_blank" rel="noreferrer"><span className="icon icon-link--external" /> LaTeX 表达式</a></li>
        </ul>
      </div>
    </div>
  )
}
