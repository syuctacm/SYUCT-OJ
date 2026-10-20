import { useEffect, useState, type FormEvent } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { createSubmission, getProblem, type ProblemDetail } from '../api.ts'
import { useAuth } from '../auth.tsx'
import { ProblemSidebar } from '../components/ProblemSidebar.tsx'

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

  if (!problem) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>

  return (
    <>
      <div className="medium-9 columns">
        <div className="section">
          <div className="section__header"><h1 className="section__title">提交评测</h1></div>
          <div className="section__body typo">
            {loading ? <div className="nothing-placeholder compact">正在确认登录状态...</div> : !user ? (
              <blockquote className="note"><p>请先 <Link to="/login">登录</Link> 后提交。</p></blockquote>
            ) : (
              <form method="post" onSubmit={handleSubmit}>
                <div className="row">
                  <div className="medium-12 columns form__item">
                    <label>
                      代码语言
                      <div className="select-container">
                        <select name="lang" className="select" value={lang} onChange={(event) => setLang(event.target.value)}>
                          <option value="cpp">C++17</option>
                          <option value="python3">Python 3</option>
                        </select>
                      </div>
                    </label>
                  </div>
                </div>
                <div className="row">
                  <div className="medium-12 columns form__item end" data-hotkey="ctrl+enter:submit">
                    <label>
                      代码
                      <div className="textarea-container">
                        <textarea
                          name="code"
                          className="textbox monospace"
                          value={code}
                          onChange={(event) => setCode(event.target.value)}
                          onKeyDown={(event) => {
                            if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') {
                              event.preventDefault()
                              event.currentTarget.form?.requestSubmit()
                            }
                          }}
                          spellCheck={false}
                          autoFocus
                          required
                        />
                      </div>
                    </label>
                  </div>
                </div>
                {contestId ? <blockquote className="note"><p>比赛提交 · {contestId}</p></blockquote> : null}
                {error ? <p className="form-error">{error}</p> : null}
                <div className="row"><div className="columns">
                  <input type="submit" className="rounded primary button" value={submitting ? '提交中...' : '提交'} disabled={submitting} />
                </div></div>
              </form>
            )}
          </div>
        </div>
      </div>
      <div className="medium-3 columns">
        <ProblemSidebar problem={problem} pageName="problem_submit" />
      </div>
    </>
  )
}
