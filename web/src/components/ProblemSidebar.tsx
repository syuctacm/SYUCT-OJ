import { Link } from 'react-router-dom'
import type { ProblemDetail } from '../api.ts'
import { formatMemoryLimit, formatTimeLimit } from '../problem-format.ts'

type ProblemSidebarProps = {
  problem: ProblemDetail
  pageName: 'problem_detail' | 'problem_submit'
}

export function ProblemSidebar({ problem, pageName }: ProblemSidebarProps) {
  const problemPath = `/p/${encodeURIComponent(problem.pid || String(problem.docId))}`
  return (
    <>
      <div className="section side section--problem-sidebar">
        {pageName !== 'problem_detail' ? (
          <div className="section__header"><h1 className="section__title">{problem.title}</h1></div>
        ) : null}
        <div>
          <ol className="menu">
            {pageName === 'problem_submit' ? (
              <li className="menu__item">
                <Link className="menu__link" to={problemPath}><span className="icon icon-flag" /> 查看题目</Link>
              </li>
            ) : null}
            <li className="menu__item scratchpad--hide">
              <Link className={`menu__link${pageName === 'problem_submit' ? ' active' : ''}`} to={`${problemPath}/submit`}>
                <span className="icon icon-send" /> 提交
              </Link>
            </li>
          </ol>
        </div>
      </div>
      {pageName === 'problem_submit' ? (
        <div className="section side">
          <div className="section__header"><h1 className="section__title">信息</h1></div>
          <div className="section__body typo" data-fragment-id="problem-sidebar-information">
            <dl className="large horizontal">
              <dt>ID</dt><dd>{problem.docId}</dd>
              <dt>时间</dt><dd>{formatTimeLimit(problem.timeLimit) || '-'}</dd>
              <dt>内存</dt><dd>{formatMemoryLimit(problem.memoryLimit) || '-'}</dd>
              <dt>难度</dt><dd>{problem.difficulty || '(None)'}</dd>
              <dt>标签</dt>
              <dd>
                {(problem.tags?.length ?? 0) > 0 ? (
                  <ul className="problem__tags">
                    {problem.tags?.map((tag) => <li className="problem__tag" key={tag}><span className="problem__tag-link">{tag}</span></li>)}
                  </ul>
                ) : '(None)'}
              </dd>
              <dt>提交数</dt><dd>{problem.nSubmit}</dd>
              <dt>通过数</dt><dd>{problem.nAccept}</dd>
              <dt>上传者</dt><dd>{problem.owner ? <Link to={`/user/${problem.owner}`}>UID {problem.owner}</Link> : '-'}</dd>
            </dl>
          </div>
        </div>
      ) : null}
    </>
  )
}
