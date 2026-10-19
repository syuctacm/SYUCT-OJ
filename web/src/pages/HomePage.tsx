import { useEffect, useState } from 'react'

type Health = {
  status: string
  service: string
  database: string
  timestamp: string
}

type Info = {
  name: string
  englishName: string
  ojUrl: string
  githubOrg: string
  version: string
}

export function HomePage() {
  const [health, setHealth] = useState<Health | null>(null)
  const [info, setInfo] = useState<Info | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    Promise.all([
      fetch('/api/v1/health').then((response) => {
        if (!response.ok) throw new Error(`health: ${response.status}`)
        return response.json() as Promise<Health>
      }),
      fetch('/api/v1/info').then((response) => {
        if (!response.ok) throw new Error(`info: ${response.status}`)
        return response.json() as Promise<Info>
      }),
    ])
      .then(([healthData, infoData]) => {
        setHealth(healthData)
        setInfo(infoData)
      })
      .catch((requestError: unknown) => {
        setError(requestError instanceof Error ? requestError.message : 'request failed')
      })
  }, [])

  return (
    <main className="shell">
      <div className="card">
        <img className="logo" src="/logo.png" alt="SYUCT ACM" />
        <p className="eyebrow">DEVELOPMENT WORKSPACE</p>
        <h1>{info?.name ?? 'SYUCT OJ'}</h1>
        <p className="subtitle">{info?.englishName ?? 'Go + React rewrite in progress'}</p>

        <div className="status-row">
          <span className={`dot ${health?.database === 'up' ? 'ok' : ''}`} />
          <span>{health ? `API ${health.status} / DB ${health.database}` : '检查本地服务中...'}</span>
        </div>

        {error ? <p className="error">开发服务尚未连接：{error}</p> : null}

        <div className="links">
          <a href="https://oj.syuctacm.cn" target="_blank" rel="noreferrer">现有 OJ</a>
          <a href="https://github.com/syuctacm" target="_blank" rel="noreferrer">GitHub 组织</a>
        </div>

        <p className="note">本地开发环境，不连接生产数据库。</p>
      </div>
    </main>
  )
}
