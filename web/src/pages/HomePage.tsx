import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import {
  getContests,
  getDiscussions,
  getProblems,
  getTrainings,
  type Contest,
  type Discussion,
  type ProblemSummary,
  type Training,
} from '../api.ts'

const ruleNames: Record<string, string> = {
  acm: 'ACM',
  oi: 'OI',
  ioi: 'IOI',
  ledo: 'Ledo',
  strictioi: 'Strict IOI',
}

function formatTime(value: string) {
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}

function durationOf(contest: Contest) {
  if (contest.duration) return contest.duration
  return Math.max(0, Math.round((new Date(contest.endAt).getTime() - new Date(contest.beginAt).getTime()) / 3600000))
}

export function HomePage() {
  const [contests, setContests] = useState<Contest[]>([])
  const [discussions, setDiscussions] = useState<Discussion[]>([])
  const [trainings, setTrainings] = useState<Training[]>([])
  const [problems, setProblems] = useState<ProblemSummary[]>([])
  const [error, setError] = useState('')

  useEffect(() => {
    Promise.all([getContests(), getDiscussions(), getTrainings(), getProblems('', 1, 'recent')])
      .then(([contestItems, discussionItems, trainingItems, problemPage]) => {
        setContests(contestItems.slice(0, 5))
        setDiscussions(discussionItems.slice(0, 5))
        setTrainings(trainingItems.slice(0, 3))
        setProblems(problemPage.items.slice(0, 8))
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '首页加载失败'))
  }, [])

  return (
    <div className="row">
      <div className="large-9 columns">
        {error ? <div className="section"><div className="section__body error-state">{error}</div></div> : null}
        {contests.length ? (
          <div className="section page--contest_main">
            <div className="section__header"><h1 className="section__title">比赛</h1></div>
            <ol className="section__list contest__list">
              {contests.map((contest) => (
                <li className={`section__list__item contest__item contest-type--${contest.rule}`} key={contest.id}>
                  <div className="media">
                    <div className="media__left medium">
                      <div className="contest__date numbox">
                        <div className="numbox__num large">{new Date(contest.beginAt).getDate()}</div>
                        <div className="numbox__text">{new Date(contest.beginAt).getFullYear()}-{new Date(contest.beginAt).getMonth() + 1}</div>
                      </div>
                    </div>
                    <div className="media__body medium">
                      <h1 className="contest__title"><Link to={`/contest/${contest.docId}`}>{contest.title}</Link></h1>
                      <ul className="supplementary list">
                        <li><Link to={`/contest?rule=${encodeURIComponent(contest.rule)}`} className="contest-type-tag"><span className="icon icon-award" />{ruleNames[contest.rule] || contest.rule}</Link></li>
                        {contest.rated ? <li className="contest-tag-rated"><span className="icon icon-star" />Rated</li> : null}
                        <li><span className="icon icon-schedule--fill text-blue" /> {durationOf(contest)} 小时</li>
                        <li><span className="icon icon-user--multiple" /> {contest.attend}</li>
                      </ul>
                    </div>
                  </div>
                </li>
              ))}
            </ol>
            <ul className="pager pager-bottom"><li><Link className="pager__item next link" to="/contest">更多 &gt;</Link></li></ul>
          </div>
        ) : null}

        {discussions.length ? (
          <div className="section page--discussion_main">
            <div className="section__header"><h1 className="section__title">讨论</h1></div>
            <ol className="section__list discussion__list">
              {discussions.map((discussion) => (
                <li className={`section__list__item discussion__item${discussion.highlight ? ' highlight' : ''}`} key={discussion.id}>
                  <div className="media">
                    <div className="media__left">
                      <div className="discussion__replies numbox">
                        <div className="numbox__num large">{discussion.nReply}</div>
                        <div className="numbox__text">回复</div>
                      </div>
                    </div>
                    <div className="media__body top">
                      <h1 className="discussion__title"><Link to={`/discuss/${discussion.docId}`}>{discussion.title}</Link></h1>
                      <ul className="supplementary list">
                        <li>{discussion.parentTitle || '讨论'}</li>
                        <li>{discussion.views} 浏览</li>
                        <li><Link to={`/user/${discussion.owner}`}>UID {discussion.owner}</Link> @ {formatTime(discussion.updateAt)}</li>
                      </ul>
                    </div>
                  </div>
                </li>
              ))}
            </ol>
            <ul className="pager pager-bottom"><li><Link className="pager__item next link" to="/discuss">更多 &gt;</Link></li></ul>
          </div>
        ) : null}

        {trainings.length ? (
          <div className="section page--training_main">
            <div className="section__header"><h1 className="section__title">训练</h1></div>
            <ol className="section__list all primary training__list">
              {trainings.map((training) => (
                <li className="section__list__item training__item" key={training.id}>
                  <div className="media">
                    <div className="media__left middle">
                      <div className="training__participants numbox">
                        <div className="numbox__num large">{training.attend}</div>
                        <div className="numbox__text">报名</div>
                      </div>
                    </div>
                    <div className="media__body middle">
                      <h1 className="training__title"><Link to={`/training/${training.docId}`}>{training.title}</Link></h1>
                      <div className="training__intro typo"><p>{training.description || ''}</p></div>
                      <ul className="supplementary list training__progress">
                        <li><span className="icon icon-flag text-blue" /> {training.sectionCount} 个章节, {training.problemCount} 道题</li>
                      </ul>
                    </div>
                  </div>
                </li>
              ))}
            </ol>
            <ul className="pager pager-bottom"><li><Link className="pager__item next link" to="/training">更多 &gt;</Link></li></ul>
          </div>
        ) : null}
      </div>
      <div className="large-3 columns">
        <div className="section side page--problem_main">
          <div className="section__header"><h1 className="section__title">最近题目</h1></div>
          <div className="section__body typo no-media col--problem-name">
            {problems.map((problem) => (
              <p key={problem.id}><Link to={`/p/${encodeURIComponent(problem.pid || String(problem.docId))}`}><b>{problem.pid}</b>&nbsp;&nbsp;{problem.title}</Link></p>
            ))}
          </div>
        </div>
        <div className="section side">
          <div className="section__header"><h1 className="section__title">推荐</h1></div>
          <div className="section__body">
            <ul className="group-list">
              <li className="group-list__item">
                <h2 className="section__title">中文</h2>
                <ol className="chip-list">
                  <li className="chip-list__item"><a className="typo-a" href="https://loj.ac/" target="_blank" rel="noreferrer">LibreOJ</a></li>
                  <li className="chip-list__item"><a className="typo-a" href="https://www.luogu.com.cn/" target="_blank" rel="noreferrer">洛谷</a></li>
                  <li className="chip-list__item"><a className="typo-a" href="https://uoj.ac/" target="_blank" rel="noreferrer">UOJ</a></li>
                </ol>
              </li>
              <li className="group-list__item">
                <h2 className="section__title">英文</h2>
                <ol className="chip-list">
                  <li className="chip-list__item"><a className="typo-a" href="https://codeforces.com/" target="_blank" rel="noreferrer">Codeforces</a></li>
                  <li className="chip-list__item"><a className="typo-a" href="https://atcoder.jp/" target="_blank" rel="noreferrer">AtCoder</a></li>
                  <li className="chip-list__item"><a className="typo-a" href="https://www.spoj.com/" target="_blank" rel="noreferrer">SPOJ</a></li>
                </ol>
              </li>
            </ul>
          </div>
        </div>
      </div>
    </div>
  )
}
