// Ported from Hydro ui-default components/discussion/reaction.page.tsx
// (AGPL-3.0). The vendored ui-default theme ships the Mantine stylesheet, so the
// popover keeps the exact upstream markup and classes.
import { MantineProvider, Popover } from '@mantine/core'
import { useState } from 'react'
import { clearRendered, renderInto } from '../upstream/roots.ts'
import { reactToDiscussion, type DiscussionReactionNode } from '../api.ts'

const DEFAULT_EMOJI_LIST = '👍 👎 😄 😕 ❤️ 🤔 🤣 🌿 🍋 🕊️ 👀 🤡'

function getRow(count: number) {
  if (count <= 2) return 2
  if (count <= 3) return 3
  if (count <= 4) return 4
  if (count <= 6) return 6
  return 12
}

function escapeHtml(value: string) {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}

export function renderReactionList(
  reactions: Record<string, number> = {},
  self: Record<string, number> = {},
  root: Element,
) {
  root.innerHTML = Object.entries(reactions)
    .sort(([, left], [, right]) => Number(right) - Number(left))
    .filter(([, count]) => Number(count) > 0)
    .map(([emoji, count]) => `<div class="reaction${self?.[emoji] ? ' active' : ''}"><span class="emoji">${escapeHtml(emoji)}</span> ${Number(count)}</div>`)
    .join('\n')
}

type PickerProps = {
  did: string
  id: string
  nodeType: DiscussionReactionNode
  listRoot: Element
  onError?: (message: string) => void
}

function ReactionPicker({ did, id, nodeType, listRoot, onError }: PickerProps) {
  const emojiList = DEFAULT_EMOJI_LIST.split(' ')
  const elesPerRow = getRow(Math.sqrt(emojiList.length))
  const [focus, setFocus] = useState(false)
  const [open, setOpen] = useState(false)
  const [trigger, setTrigger] = useState(false)

  async function pick(emoji: string) {
    try {
      const result = await reactToDiscussion(did, { id, nodeType, emoji, reverse: false })
      renderReactionList(result.doc.react, result.sdoc.react, listRoot)
      setOpen(false)
      setTrigger(false)
    } catch (err) {
      onError?.(err instanceof Error ? err.message : '反应失败')
    }
  }

  const rows: string[][] = []
  for (let index = 0; index < emojiList.length; index += elesPerRow) {
    rows.push(emojiList.slice(index, index + elesPerRow))
  }

  return (
    <MantineProvider>
      <Popover opened={trigger || open || focus}>
        <Popover.Target>
          <span
            className="icon icon-emoji"
            onMouseEnter={() => setTrigger(true)}
            onMouseLeave={() => {
              setTimeout(() => setTrigger(false), 300)
            }}
          />
        </Popover.Target>
        <Popover.Dropdown onMouseEnter={() => setOpen(true)} onMouseLeave={() => setOpen(false)}>
          {rows.map((line, index) => (
            <div className="row" key={index} style={{ paddingBottom: 4, paddingTop: 4 }}>
              {line.map((emoji) => (
                <div
                  key={emoji}
                  className={`medium-${12 / elesPerRow} small-${12 / elesPerRow} columns popover-reaction-item`}
                  onClick={() => void pick(emoji)}
                >
                  {emoji}
                </div>
              ))}
            </div>
          ))}
          <div className="row" style={{ paddingTop: 7, paddingBottom: 4 }}>
            <div className="medium-12 columns">
              <input
                name="emojiSuggest"
                onFocus={() => setFocus(true)}
                onBlur={() => setFocus(false)}
              />
            </div>
          </div>
        </Popover.Dropdown>
      </Popover>
    </MantineProvider>
  )
}

type MountOptions = {
  did: string
  canReact: boolean
  onError?: (message: string) => void
  onRequireLogin: () => void
}

export function mountDiscussionReactions(root: HTMLElement, options: MountOptions) {
  const containers: HTMLElement[] = []
  root.querySelectorAll<HTMLElement>('[data-op="react"]').forEach((element) => {
    const raw = element.dataset.form
    if (!raw) return
    let payload: { nodeType?: DiscussionReactionNode; id?: string }
    try {
      payload = JSON.parse(raw)
    } catch {
      return
    }
    if (!payload.id || !payload.nodeType) return
    const list = root.querySelector(`.reactions[data-${payload.nodeType}='${payload.id}']`)
    if (!list) return
    renderInto(
      element,
      <ReactionPicker
        did={options.did}
        id={payload.id}
        nodeType={payload.nodeType}
        listRoot={list}
        onError={options.onError}
      />,
    )
    containers.push(element)
  })

  const onClick = async (event: Event) => {
    const target = (event.target as HTMLElement).closest('.reaction')
    if (!target) return
    const list = target.parentElement
    if (!list) return
    if (!options.canReact) {
      options.onRequireLogin()
      return
    }
    const nodeType = (list.getAttribute('data-type') ?? 'did') as DiscussionReactionNode
    const id = list.getAttribute(`data-${nodeType}`) ?? ''
    const emoji = target.querySelector('.emoji')?.textContent?.trim() ?? ''
    if (!id || !emoji) return
    event.preventDefault()
    try {
      const result = await reactToDiscussion(options.did, {
        id,
        nodeType,
        emoji,
        reverse: target.classList.contains('active'),
      })
      renderReactionList(result.doc.react, result.sdoc.react, list)
    } catch (err) {
      options.onError?.(err instanceof Error ? err.message : '反应失败')
    }
  }
  root.addEventListener('click', onClick)
  return () => {
    root.removeEventListener('click', onClick)
    containers.forEach(clearRendered)
  }
}
