// The upstream Lucky button redirects to a random visible problem; this page
// resolves the target through the API and replaces the history entry.
import { useEffect, useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { getRandomProblem } from '../api.ts'

export function RandomProblemPage() {
  const location = useLocation()
  const navigate = useNavigate()
  const [error, setError] = useState('')

  useEffect(() => {
    const query = new URLSearchParams(location.search).get('q') ?? ''
    getRandomProblem(query)
      .then((pid) => navigate(`/p/${encodeURIComponent(pid)}`, { replace: true }))
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '没有符合条件的题目'))
  }, [location.search, navigate])

  return (
    <div className="row"><div className="columns">
      <div className="nothing-placeholder">{error || '正在随机选题...'}</div>
    </div></div>
  )
}
