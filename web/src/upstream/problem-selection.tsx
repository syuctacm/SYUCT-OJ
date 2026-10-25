// Ported from @hydrooj/ui-default pages/problem_main.page.tsx (AGPL-3.0).
// The selection UI and dialog structure stay upstream; clipboard and API shims are local.
import { useEffect, useRef, useState } from 'react'

import { notifyError, notifySuccess } from '../components/Notification.tsx'
import { ProblemSelectAutoComplete } from '../components/autocomplete/ProblemSelectAutoComplete.tsx'

type Props = {
  initialPids?: string[]
  onChange: (pids: string[]) => void
  onClear: (handler: () => void) => void
}

function allPids() {
  return Array.from(document.querySelectorAll<HTMLInputElement>('.main [data-checkbox-group="problem"]'))
    .map((checkbox) => checkbox.closest<HTMLElement>('tr')?.dataset.pid ?? '')
    .filter(Boolean)
}

function checkedPids() {
  return Array.from(document.querySelectorAll<HTMLInputElement>('.main [data-checkbox-group="problem"]:checked'))
    .map((checkbox) => checkbox.closest<HTMLElement>('tr')?.dataset.pid ?? '')
    .filter(Boolean)
}

async function copyText(text: string) {
  try {
    await navigator.clipboard.writeText(text)
  } catch {
    const input = document.createElement('textarea')
    input.value = text
    input.style.position = 'fixed'
    input.style.opacity = '0'
    document.body.append(input)
    input.select()
    document.execCommand('copy')
    input.remove()
  }
}

export function ProblemSelection({ initialPids = [], onChange, onClear }: Props) {
  const [pids, setPids] = useState<string[]>(initialPids)
  const [dialogOpen, setDialogOpen] = useState(false)
  const selectionRef = useRef<{
    getSelectedItems: () => Array<{ docId: number; pid?: string }>
  } | null>(null)

  useEffect(() => {
    onClear(() => setPids([]))
  }, [onClear])

  useEffect(() => {
    const update = () => queueMicrotask(() => {
      const all = allPids()
      const selected = checkedPids()
      setPids((current) => {
        const next = current.filter((pid) => !all.includes(pid) || selected.includes(pid))
        for (const pid of selected) if (!next.includes(pid)) next.push(pid)
        return next
      })
    })
    document.addEventListener('click', update)
    return () => document.removeEventListener('click', update)
  }, [])

  useEffect(() => {
    onChange(pids)
    for (const checkbox of document.querySelectorAll<HTMLInputElement>('.main [data-checkbox-group="problem"]')) {
      const pid = checkbox.closest<HTMLElement>('tr')?.dataset.pid ?? ''
      checkbox.checked = pids.includes(pid)
    }
  }, [onChange, pids])

  async function copy(kind: 'ids' | 'pids') {
    const items = selectionRef.current?.getSelectedItems() ?? []
    const values = kind === 'ids'
      ? (items.length ? items.map((item) => item.docId) : pids)
      : items.map((item) => item.pid || item.docId)
    if (!values.length) {
      notifyError('请先选择题目。')
      return
    }
    try {
      await copyText(values.join(','))
      notifySuccess('题目编号已复制。')
    } catch (error) {
      notifyError(error instanceof Error ? error.message : '复制失败')
    }
  }

  return (
    <>
      <a className="menu__link display-mode-hide" onClick={() => setDialogOpen(true)}>
        <span className="icon icon-stack"></span>
        {' '}{`已选择 ${pids.length} 道题目`}
      </a>
      <div
        className="dialog withBg"
        style={{ display: dialogOpen ? 'flex' : 'none', zIndex: 1000, opacity: 1 }}
        onClick={() => setDialogOpen(false)}
      >
        <div className="dialog__content" style={{ transform: 'scale(1, 1)' }} onClick={(event) => event.stopPropagation()}>
          <div className="dialog__body" style={{ height: 'calc(100% - 45px)' }}>
            <div className="row"><div className="columns"><h1>选择题目</h1></div></div>
            <div className="row"><div className="columns">
              <ProblemSelectAutoComplete
                ref={selectionRef as never}
                multi
                onChange={(value) => setPids(value.split(',').filter((item) => item.trim()))}
                selectedKeys={pids}
              />
              <style>{'.autocomplete-wrapper { max-height: 50vh; }'}</style>
            </div></div>
          </div>
          <div className="row"><div className="columns clearfix">
            <div className="float-right dialog__action">
              <button className="rounded button" onClick={() => void copy('ids')}>复制 ID</button>{' '}
              <button className="rounded button" onClick={() => void copy('pids')}>复制 Pid</button>{' '}
              <button className="rounded button" onClick={() => setPids([])}>清空</button>{' '}
              <button className="primary rounded button" onClick={() => setDialogOpen(false)}>确定</button>
            </div>
          </div></div>
        </div>
      </div>
    </>
  )
}
