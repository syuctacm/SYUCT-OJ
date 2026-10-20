import flatpickr from 'flatpickr'

function padTwo(value: number) {
  return value < 10 ? `0${value}` : `${value}`
}

function setInputValue(input: HTMLInputElement, value: string) {
  const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set
  if (setter) setter.call(input, value)
  else input.value = value
  input.dispatchEvent(new Event('input', { bubbles: true }))
  input.dispatchEvent(new Event('change', { bubbles: true }))
}

export function attachDatePicker(input: HTMLInputElement, onSelect?: (value: string) => void) {
  const picker = flatpickr(input, {
    allowInput: true,
    onChange: (_dates, dateStr) => onSelect?.(dateStr),
  })
  return () => picker.destroy()
}

export function attachTimePicker(input: HTMLInputElement, onSelect?: (value: string) => void) {
  const interval = 15
  const wrapper = document.createElement('div')
  Object.assign(wrapper.style, {
    position: 'relative',
    display: 'inline-block',
    width: '100%',
  })
  input.parentNode?.insertBefore(wrapper, input)
  wrapper.appendChild(input)

  const dropdown = document.createElement('div')
  dropdown.className = 'time-picker-dropdown'
  Object.assign(dropdown.style, {
    position: 'absolute',
    top: `${input.offsetHeight}px`,
    left: '0',
    right: '0',
    maxHeight: '0',
    overflowY: 'auto',
    background: 'var(--bg-color, #fff)',
    border: '1px solid #ccc',
    borderTop: 'none',
    borderRadius: '0 0 4px 4px',
    zIndex: '1000',
    opacity: '0',
    boxShadow: '0 2px 8px rgba(0,0,0,0.15)',
    transition: 'max-height .2s ease, opacity .2s ease',
    pointerEvents: 'none',
  })

  let isOpen = false
  function closeDropdown() {
    if (!isOpen) return
    isOpen = false
    dropdown.style.maxHeight = '0'
    dropdown.style.opacity = '0'
    dropdown.style.pointerEvents = 'none'
  }
  function openDropdown() {
    if (isOpen) return
    isOpen = true
    dropdown.style.maxHeight = '200px'
    dropdown.style.opacity = '1'
    dropdown.style.pointerEvents = 'auto'
    const value = input.value
    if (!value) return
    for (const item of dropdown.children) {
      const element = item as HTMLElement
      if (element.textContent !== value) continue
      dropdown.scrollTop = element.offsetTop - dropdown.offsetTop - 80
      break
    }
  }

  for (let hour = 0; hour < 24; hour++) {
    for (let minute = 0; minute < 60; minute += interval) {
      const time = `${padTwo(hour)}:${padTwo(minute)}`
      const item = document.createElement('div')
      item.textContent = time
      Object.assign(item.style, { padding: '6px 12px', cursor: 'pointer' })
      item.addEventListener('mousedown', (event) => {
        event.preventDefault()
        setInputValue(input, time)
        onSelect?.(time)
        closeDropdown()
      })
      item.addEventListener('mouseenter', () => {
        item.style.background = 'var(--highlight-color, #e8f0fe)'
      })
      item.addEventListener('mouseleave', () => {
        item.style.background = ''
      })
      dropdown.appendChild(item)
    }
  }

  input.addEventListener('focus', openDropdown)
  const onBlur = () => {
    if (document.hasFocus()) closeDropdown()
  }
  input.addEventListener('blur', onBlur)
  wrapper.appendChild(dropdown)

  return () => {
    input.removeEventListener('focus', openDropdown)
    input.removeEventListener('blur', onBlur)
    dropdown.remove()
    if (input.parentNode === wrapper) {
      wrapper.parentNode?.insertBefore(input, wrapper)
      wrapper.remove()
    }
  }
}
