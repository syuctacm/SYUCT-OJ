// Ported from Hydro ui-default components/discussion/history.page.tsx
// (AGPL-3.0). The revision list keeps the upstream menu markup and the revision
// body is rendered with the same markdown pipeline as the page itself.
import { MantineProvider, Popover } from '@mantine/core'
import { hydroMantineTheme } from '../upstream/mantine.ts'
import { useState } from 'react'
import { clearRendered, renderInto } from '../upstream/roots.ts'
import { getDiscussionHistory, getDiscussionRevision, type DiscussionHistoryEntry } from '../api.ts'
import { renderMarkdown } from '../upstream/markdown.ts'
import { Dialog } from './Dialog.tsx'

function History({ payload }: { payload: string }) {
  const [opened, setOpened] = useState(false)
  const [loaded, setLoaded] = useState(false)
  const [entries, setEntries] = useState<DiscussionHistoryEntry[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [revision, setRevision] = useState<{ entry: DiscussionHistoryEntry; content: string } | null>(null)

  function parsePayload() {
    const url = new URL(payload, window.location.origin)
    const match = /\/discussions\/([^/]+)\/raw/.exec(url.pathname)
    return {
      did: match?.[1] ?? '',
      docId: url.searchParams.get('docId') ?? undefined,
    }
  }

  async function load() {
    if (loaded) return
    setLoaded(true)
    setLoading(true)
    setError('')
    try {
      const { did, docId } = parsePayload()
      setEntries(await getDiscussionHistory(did, docId))
    } catch (err) {
      setError(err instanceof Error ? err.message : '历史版本加载失败')
    } finally {
      setLoading(false)
    }
  }

  async function openRevision(entry: DiscussionHistoryEntry) {
    try {
      const { did, docId } = parsePayload()
      const content = await getDiscussionRevision(did, docId, entry.time)
      setRevision({ entry, content })
    } catch (err) {
      setError(err instanceof Error ? err.message : '版本加载失败')
    }
  }

  return (
    <>
      <MantineProvider theme={hydroMantineTheme}>
        <Popover opened={opened} onChange={setOpened} onOpen={load}>
          <Popover.Target>
            <span
              onClick={() => setOpened((value) => !value)}
              onMouseEnter={() => setOpened(true)}
              onMouseLeave={() => {
                setTimeout(() => setOpened(false), 300)
              }}
            >
              编辑于 <span className="icon icon-expand_more" />
            </span>
          </Popover.Target>
          <Popover.Dropdown
            onMouseEnter={() => setOpened(true)}
            onMouseLeave={() => setOpened(false)}
          >
            <ol className="menu">
              {loading || error ? (
                <li className="menu__item">
                  <a className="menu__link">{loading ? '加载中…' : error}</a>
                </li>
              ) : null}
              {entries.map((entry) => (
                <li className="menu__item" key={entry.time}>
                  <a className="menu__link" onClick={() => void openRevision(entry)}>
                    UID {entry.uid} · {new Date(entry.time).toLocaleString('zh-CN')}
                  </a>
                </li>
              ))}
            </ol>
          </Popover.Dropdown>
        </Popover>
      </MantineProvider>
      {revision ? (
        <Dialog
          onOk={() => setRevision(null)}
          onCancel={() => setRevision(null)}
          okLabel="关闭"
          cancelLabel="取消"
          width="800px"
        >
          <div className="typo richmedia">
            <p>
              <span data-user>{revision.entry.uid}</span> 编辑于{' '}
              <span className="time relative">{new Date(revision.entry.time).toLocaleString('zh-CN')}</span>
            </p>
            <div
              style={{ scrollBehavior: 'smooth', maxHeight: '60vh', overflow: 'auto' }}
              dangerouslySetInnerHTML={{ __html: renderMarkdown(revision.content) }}
            />
          </div>
        </Dialog>
      ) : null}
    </>
  )
}

export function mountDiscussionHistory(root: HTMLElement) {
  const containers: HTMLElement[] = []
  root.querySelectorAll<HTMLElement>('[data-discussion-history]').forEach((element) => {
    const payload = element.getAttribute('data-raw-url')
    if (!payload) return
    renderInto(element, <History payload={payload} />)
    containers.push(element)
  })
  return () => containers.forEach(clearRendered)
}
