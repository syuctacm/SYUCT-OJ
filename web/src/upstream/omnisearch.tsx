// Ported from @hydrooj/ui-default components/omnisearch/index.page.tsx (AGPL-3.0).
// Request helpers are mapped to the Go API instead of Hydro's internal API.
import $ from 'jquery'
import { debounce } from 'lodash'
import { useState } from 'react'
import { createRoot } from 'react-dom/client'

import { getProblems, searchUsers, type ProblemSummary, type UserChoice } from '../api.ts'
import { statusClass, statusLabels } from '../status.ts'
import { avatarUrl } from './avatar.ts'

let installed = false

function SearchResult() {
  const [searching, setSearching] = useState(false)
  const [problems, setProblems] = useState<ProblemSummary[]>([])
  const [users, setUsers] = useState<UserChoice[]>([])

  const search = debounce(async (query: string) => {
    if (!query.trim()) {
      setProblems([])
      setUsers([])
      setSearching(false)
      return
    }
    setSearching(true)
    try {
      const [problemPage, nextUsers] = await Promise.all([
        getProblems(query, 1, 'default'),
        searchUsers(query),
      ])
      setProblems(problemPage.items.slice(0, 10))
      setUsers(nextUsers.slice(0, 10))
    } finally {
      setSearching(false)
    }
  }, 1000)

  return (
    <>
      <input
        placeholder="Search"
        onChange={(event) => { void search(event.currentTarget.value) }}
      />
      <span className="icon icon-search" />
      <div id="omnibar-content">
        {searching ? <div>Searching...</div> : null}
        {problems.length ? <div className="omnibar-content-title">题目</div> : null}
        {problems.map((problem) => (
          <a key={problem.docId} className="omnibar-content-section omnibar-problem" href={`/p/${encodeURIComponent(problem.pid || String(problem.docId))}`}>
            <div>
              {problem.userStatus ? <p className={`record-status--text ${statusClass(problem.userStatus.status)}`}>
                <span className={`icon record-status--icon ${statusClass(problem.userStatus.status)}`} />
                <span className="supplementary">{statusLabels[problem.userStatus.status]}</span>
              </p> : null}
              <div>{problem.title}</div>
            </div>
            <div>
              <span className="icon icon-book" />{problem.pid || problem.docId}
              <span className="icon icon-pie-chart" />{problem.nAccept}/{problem.nSubmit}
            </div>
          </a>
        ))}
        {users.length ? <div className="omnibar-content-title">用户</div> : null}
        {users.map((user) => (
          <a key={user.id} className="omnibar-content-section omnibar-user" href={`/user/${user.id}`}>
            <div>
              <img src={avatarUrl(user.avatar, 64)} alt={user.username} />
              <div className="omnibar-user-info">
                <span>UID {user.id}</span><br />
                <div><span style={{ lineHeight: '20px' }}>{user.displayName || user.username}</span></div>
              </div>
            </div>
          </a>
        ))}
      </div>
    </>
  )
}

export function installHydroOmnisearch() {
  if (installed) return
  installed = true
  const $search = $(`
    <div class="omnibar" data-hotkey="esc:click" style="opacity:0;display:none;">
      <div class="omnibar-main"></div>
    </div>
  `)
  const $entry = $(`
    <button class="omnibar-toggle" data-global-hotkey="ctrl+k:click">
      <span class="icon icon-search"></span>
    </button>
  `)
  $('body').append($search).append($entry)
  const root = createRoot($search.find('.omnibar-main').get(0)!)
  root.render(<SearchResult />)

  let open = false
  const show = () => {
    $search.show()
    window.setTimeout(() => {
      $search.css('opacity', 1)
      $search.find('input').trigger('focus')
    }, 20)
    open = true
  }
  const hide = () => {
    $search.css('opacity', 0)
    window.setTimeout(() => $search.hide(), 200)
    open = false
  }
  $entry.on('click.hydroOmnisearch', () => { if (open) hide(); else show() })
  $search.on('click.hydroOmnisearch', hide)
  $search.find('.omnibar-main').on('click.hydroOmnisearch', (event) => event.stopPropagation())

  const updateVisibility = () => {
    $entry.toggle(document.documentElement.dataset.layout === 'basic')
    if (document.documentElement.dataset.layout !== 'basic') hide()
  }
  $(document).on('vjPageFullyInitialized.hydroOmnisearch', updateVisibility)
  updateVisibility()
}
