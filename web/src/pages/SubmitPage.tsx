import { useEffect, useState, type FormEvent } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { createSubmission, getProblem, type ProblemDetail } from '../api.ts'
import { useAuth } from '../auth.tsx'

const defaultCode = `#include <bits/stdc++.h>
using namespace std;

int main() {
    ios::sync_with_stdio(false);
    cin.tie(nullptr);
    return 0;
}
`

export function SubmitPage() {
  const { pid = '' } = useParams()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const contestId = searchParams.get('contest') ?? ''
  const { user, loading } = useAuth()
  const [problem, setProblem] = useState<ProblemDetail | null>(null)
  const [code, setCode] = useState(defaultCode)
  const [lang, setLang] = useState('cpp')
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => {
    getProblem(pid).then(setProblem).catch(() => setProblem(null))
  }, [pid])

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!user) return
    setSubmitting(true)
    setError('')
    try {
      const submission = await createSubmission(pid, lang, code, contestId)
      navigate(`/record/${submission.id}`)
    } catch (err) {
      setError(err instanceof Error ? err.message : '提交失败')
    } finally {
      setSubmitting(false)
    }
  }

  if (loading) return <section className="section"><div className="empty-state">加载中...</div></section>
  if (!user) return <section className="section"><div className="empty-state">请先 <Link to="/login">登录</Link> 后提交。</div></section>

  return (
    <section className="section">
      <div className="section__header">
        <div>
          <p className="eyebrow">SUBMIT</p>
          <h1 className="section__title">{problem?.title || pid}</h1>
          {contestId ? <p className="supplementary">比赛提交 · {contestId}</p> : null}
        </div>
        <Link className="rounded button" to={`/p/${encodeURIComponent(pid)}`}>返回题目</Link>
      </div>
      <form className="section__body form-stack" onSubmit={handleSubmit}>
        <label className="field field-inline">
          <span>语言</span>
          <select className="select" value={lang} onChange={(event) => setLang(event.target.value)}>
            <option value="cpp">C++17</option>
            <option value="python3">Python 3</option>
          </select>
        </label>
        <label className="field">
          <span>代码</span>
          <textarea className="textbox code-editor" value={code} onChange={(event) => setCode(event.target.value)} spellCheck={false} />
        </label>
        {error ? <p className="form-error">{error}</p> : null}
        <button className="rounded primary button" type="submit" disabled={submitting}>{submitting ? '提交中...' : '提交评测'}</button>
      </form>
    </section>
  )
}
