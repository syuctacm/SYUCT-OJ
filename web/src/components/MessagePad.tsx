// Port of the upstream ui-default messagepad React app
// (components/messagepad/*) without Redux. The DOM keeps the upstream class
// names so the pinned ui-default stylesheet renders it unchanged.
import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import {
  getMessages, getPublicUser, searchUsers, sendMessage,
  type MessageDialogue, type MessageDoc, type User, type UserChoice,
} from '../api.ts'
import { Dialog } from './Dialog.tsx'
import { Icon } from './Icon.tsx'
import { avatarUrl } from '../upstream/avatar.ts'

const POLL_INTERVAL = 10000

function timeAgo(iso: string) {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return ''
  const diff = (Date.now() - date.getTime()) / 1000
  if (diff < 60) return '刚刚'
  if (diff < 3600) return `${Math.floor(diff / 60)} 分钟前`
  if (diff < 86400) return `${Math.floor(diff / 3600)} 小时前`
  if (diff < 86400 * 30) return `${Math.floor(diff / 86400)} 天前`
  return date.toLocaleDateString('zh-CN')
}

function absoluteTime(iso: string) {
  const date = new Date(iso)
  return Number.isNaN(date.getTime()) ? '' : date.toLocaleString('zh-CN', { hour12: false })
}

// System messages (from uid 1) may carry {message, params} payloads, matching
// MessagePadDialogueContentContainer.renderContent upstream.
function messageText(msg: MessageDoc) {
  if (msg.from === 1) {
    try {
      const data = JSON.parse(msg.content) as { message?: string; params?: Record<string, unknown> }
      if (typeof data.message === 'string') {
        return data.message.replace(/\{([^{}]+)\}/g, (match, key: string) => {
          const value = data.params?.[key.split(':link')[0]]
          return value === undefined ? match : String(value)
        })
      }
    } catch {
      // Plain text system message.
    }
  }
  return msg.content
}

const SYSTEM_MESSAGE_TRANSLATIONS: Record<string, string> = {
  'User {0} mentioned you in {1:link}': '用户 {0} 在 {1:link} 中提到了你',
}

// renderMessageContent keeps the system-message placeholder semantics from
// ui-default's MessagePadDialogueContentContainer. In particular, params with
// a :link suffix become real links instead of literal text.
function renderMessageContent(msg: MessageDoc): ReactNode {
  if (msg.from !== 1) return msg.content
  let data: { message?: string; params?: Record<string, unknown> | unknown[] }
  try {
    data = JSON.parse(msg.content)
  } catch {
    return msg.content
  }
  if (typeof data.message !== 'string') return msg.content
  const params = data.params ?? {}
  const template = SYSTEM_MESSAGE_TRANSLATIONS[data.message] ?? data.message
  const parts = template.split(/(\{[^{}]+\})/g)
  return parts.map((part, index) => {
    const match = /^\{([^{}]+)\}$/.exec(part)
    if (!match) return part
    const key = match[1]
    const value = Array.isArray(params) ? params[Number(key.split(':link')[0])] : params[key.split(':link')[0]]
    if (value === undefined || value === null) return part
    const text = String(value)
    if (key.endsWith(':link')) {
      return <a key={index} style={{ color: 'wheat' }} href={text} target="_blank" rel="noreferrer">{text}</a>
    }
    return <span key={index} style={{ color: 'wheat' }}>{text}</span>
  })
}

function dialogueSummary(dialogue: MessageDialogue) {
  const last = dialogue.messages[dialogue.messages.length - 1]
  if (!last) return ''
  if ((last.flag & 4) !== 0) return '[富文本消息]'
  return messageText(last)
}

type Props = {
  // The pad is mounted into its own React root, so the signed-in user is
  // passed in instead of read from the AuthProvider context.
  user: User | null
  target?: number | null
  onError?: (message: string) => void
}

export function MessagePad({ user, target = null, onError }: Props) {
  const [dialogues, setDialogues] = useState<MessageDialogue[]>([])
  const [activeId, setActiveId] = useState<number | null>(target)
  const [pending, setPending] = useState<MessageDialogue | null>(null)
  const [inputs, setInputs] = useState<Record<number, string>>({})
  const [posting, setPosting] = useState(false)
  const [error, setError] = useState('')
  const [picking, setPicking] = useState(false)
  const [query, setQuery] = useState('')
  const [choices, setChoices] = useState<UserChoice[]>([])
  const contentRef = useRef<HTMLOListElement>(null)

  const load = useCallback(async () => {
    try {
      const data = await getMessages()
      setDialogues(data.items)
      setError('')
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : '消息加载失败'
      setError(message)
      onError?.(message)
    }
  }, [onError])

  useEffect(() => {
    void load()
    const timer = window.setInterval(() => void load(), POLL_INTERVAL)
    return () => window.clearInterval(timer)
  }, [load])

  // `?target=<uid>` opens (or starts) a conversation with that user.
  useEffect(() => {
    if (!target) return
    setActiveId(target)
    getPublicUser(String(target))
      .then((profile) => {
        setPending((current) => current ?? {
          id: target,
          user: {
            id: profile.id,
            username: profile.username,
            displayName: profile.displayName,
            avatar: profile.avatar,
          },
          messages: [],
        })
      })
      .catch(() => undefined)
  }, [target])

  const active = useMemo(() => {
    if (activeId === null) return null
    return dialogues.find((dialogue) => dialogue.id === activeId)
      ?? (pending && pending.id === activeId ? pending : null)
  }, [activeId, dialogues, pending])

  useEffect(() => {
    const node = contentRef.current
    if (node) node.scrollTop = node.scrollHeight
  }, [active, activeId])

  useEffect(() => {
    if (!picking) return
    const timer = window.setTimeout(() => {
      searchUsers(query.trim())
        .then(setChoices)
        .catch(() => setChoices([]))
    }, 250)
    return () => window.clearTimeout(timer)
  }, [picking, query])

  function pick(userChoice: UserChoice) {
    setPending({ id: userChoice.id, user: userChoice, messages: [] })
    setActiveId(userChoice.id)
    setPicking(false)
    setQuery('')
    setChoices([])
  }

  async function send() {
    if (activeId === null || posting) return
    const content = (inputs[activeId] ?? '').trim()
    if (!content) return
    setPosting(true)
    setError('')
    try {
      await sendMessage(activeId, content)
      setInputs((current) => ({ ...current, [activeId]: '' }))
      setPending((current) => (current && current.id === activeId ? { ...current, messages: [] } : current))
      await load()
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : '发送失败'
      setError(message)
      onError?.(message)
    } finally {
      setPosting(false)
    }
  }

  const value = activeId === null ? '' : inputs[activeId] ?? ''

  return (
    <div className="messagepad clearfix">
      <div className="messagepad__sidebar">
        <div className="section__header">
          <button className="primary rounded button" onClick={() => setPicking(true)}>
            <Icon name="add" /> 新消息
          </button>
        </div>
        <ol className="messagepad__list" style={{ overscrollBehavior: 'contain' }}>
          {dialogues.map((dialogue) => (
            <li key={dialogue.id}>
              <a
                className={`messagepad__list-item media${dialogue.id === activeId ? ' active' : ''}`}
                onClick={() => setActiveId(dialogue.id)}
              >
                <div className="media__left middle">
                  <img
                    src={avatarUrl(dialogue.user.avatar ?? '', 50)}
                    alt="avatar"
                    width={50}
                    height={50}
                    className="medium user-profile-avatar"
                  />
                </div>
                <div className="media__body middle">
                  <h3 className="messagepad__username">{dialogue.user.username}</h3>
                  <div className="messagepad__desc">{dialogueSummary(dialogue)}</div>
                </div>
              </a>
            </li>
          ))}
        </ol>
      </div>
      <div className="messagepad__header">
        {active ? (
          <a className="messagepad__content__header__title" href={`/user/${active.user.id}`}>
            {`${active.user.username}(UID: ${active.user.id})`}
          </a>
        ) : null}
      </div>
      <ol className="messagepad__content" style={{ overscrollBehavior: 'contain' }} ref={contentRef}>
        {active?.messages.map((msg) => (
          <li
            key={msg.id}
            className={`messagepad__message ${msg.from === user?.id ? 'side--self' : 'side--other'}`}
          >
            <div className="messagepad__message__avatar">
              <img
                src={avatarUrl((msg.from === user?.id ? user?.avatar : active.user.avatar) ?? '', 50)}
                alt="avatar"
                width={50}
                height={50}
                className="medium user-profile-avatar"
              />
            </div>
            <div className="messagepad__message__body">
              <div>{renderMessageContent(msg)}</div>
              <time data-tooltip={absoluteTime(msg.createdAt)}>{timeAgo(msg.createdAt)}</time>
            </div>
          </li>
        ))}
      </ol>
      <div className={`messagepad__input${activeId !== null ? ' visible' : ''}`}>
        <div className="messagepad__textarea-container">
          <textarea
            data-markdown
            disabled={posting}
            value={value}
            onChange={(event) => {
              if (activeId === null) return
              const next = event.target.value
              setInputs((current) => ({ ...current, [activeId]: next }))
            }}
            onKeyDown={(event) => {
              if (event.key === 'Enter' && (event.ctrlKey || event.metaKey)) {
                event.preventDefault()
                void send()
              }
            }}
          />
        </div>
        <button disabled={!value.trim() || posting} onClick={() => void send()}>
          <Icon name="send" />
        </button>
      </div>
      {error ? <div className="messagepad__error"><blockquote className="warn"><p>{error}</p></blockquote></div> : null}
      {picking ? (
        <Dialog okLabel="关闭" cancelLabel="" onOk={() => setPicking(false)} onCancel={() => setPicking(false)}>
          <div className="section">
            <div className="section__header"><h1 className="section__title">新消息</h1></div>
            <div className="section__body">
              <input
                className="textbox"
                autoFocus
                placeholder="搜索用户名"
                value={query}
                onChange={(event) => setQuery(event.target.value)}
              />
              <ul className="menu">
                {choices.map((choice) => (
                  <li key={choice.id} className="menu__item">
                    <a className="menu__link" onClick={() => pick(choice)}>
                      {choice.username} (UID: {choice.id})
                    </a>
                  </li>
                ))}
              </ul>
            </div>
          </div>
        </Dialog>
      ) : null}
    </div>
  )
}
