import { useEffect, useMemo, useState } from 'react'
import { getStatusPage, type StatusPageData } from '../api.ts'
import { HydroTemplate } from '../components/HydroTemplate.tsx'

export function StatusPage() {
  const [data, setData] = useState<StatusPageData | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    getStatusPage()
      .then(setData)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '状态加载失败'))
  }, [])

  const context = useMemo(() => ({
    page_name: 'status',
    stats: (data?.stats ?? []).map((stat) => ({
      ...stat,
      _id: { toHexString: () => stat.id },
      osinfo: stat.osinfo,
      cpu: stat.cpu,
      memory: stat.memory,
    })),
    languages: data?.languages ?? {},
    compilers: data?.compilers ?? [],
    handler: { user: { _id: 0, viewLang: 'zh', hasPriv: () => false, hasPerm: () => false } },
  }), [data])

  if (error) return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  if (!data) return <div className="row"><div className="columns"><div className="nothing-placeholder">加载中...</div></div></div>
  return <HydroTemplate template="status.html" context={context} />
}
