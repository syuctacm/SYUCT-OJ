import { useEffect, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { useParams } from 'react-router-dom'
import { getTraining, type Training } from '../api.ts'

export function TrainingDetailPage() {
  const { tid = '' } = useParams()
  const [training, setTraining] = useState<Training | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    getTraining(tid).then(setTraining).catch((err: unknown) => setError(err instanceof Error ? err.message : '训练加载失败'))
  }, [tid])

  if (error) return <section className="panel"><div className="empty-state error-state">{error}</div></section>
  if (!training) return <section className="panel"><div className="empty-state">加载中...</div></section>

  return (
    <section className="panel">
      <div className="panel-header">
        <div>
          <p className="eyebrow">TRAINING</p>
          <h1 className="panel-title">{training.title}</h1>
          {training.description ? <p className="panel-subtitle">{training.description}</p> : null}
        </div>
        {training.pin ? <span className="tag">置顶</span> : null}
      </div>
      <div className="panel-body problem-content">
        <ReactMarkdown remarkPlugins={[remarkGfm]}>{training.content || '暂无训练内容'}</ReactMarkdown>
      </div>
    </section>
  )
}
