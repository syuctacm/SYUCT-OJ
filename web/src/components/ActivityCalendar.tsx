import { useEffect, useState, type ReactNode } from 'react'
import { getUserActivity, type UserActivity } from '../api.ts'

const colors = ['#edf2f7', '#b8dcf9', '#70b4f4', '#288bf5', '#0960d6']
const cellWidth = 11.5
const cellGap = 3.2
const offsetX = 24
const offsetY = 24

function colorFor(count: number) {
  if (count <= 0) return colors[0]
  if (count <= 2) return colors[1]
  if (count <= 5) return colors[2]
  if (count <= 9) return colors[3]
  return colors[4]
}

export function ActivityCalendar({ uid }: { uid: number }) {
  const [data, setData] = useState<UserActivity | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    getUserActivity(String(uid)).then(setData).catch((err: unknown) => setError(err instanceof Error ? err.message : '活跃数据加载失败'))
  }, [uid])

  if (error) return <div className="nothing-placeholder compact">{error}</div>
  if (!data) return <div className="nothing-placeholder compact">加载中...</div>

  const now = new Date()
  const beijingNow = new Date(now.getTime() + (now.getTimezoneOffset() + 480) * 60000)
  const totalDays = 52 * 7 + beijingNow.getDay() + 1
  const startDate = new Date(beijingNow.getTime() - (totalDays - 1) * 86400000)
  const weekdayNames = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']
  const monthNames = ['1月', '2月', '3月', '4月', '5月', '6月', '7月', '8月', '9月', '10月', '11月', '12月']
  const cells: ReactNode[] = []
  const monthLabels: ReactNode[] = []
  let lastMonth = -1

  for (let week = 0; week < 53; week++) {
    for (let day = 0; day < 7; day++) {
      const dayOffset = week * 7 + day
      if (dayOffset >= totalDays) continue
      const date = new Date(startDate.getTime() + dayOffset * 86400000)
      const y = date.getFullYear()
      const m = String(date.getMonth() + 1).padStart(2, '0')
      const d = String(date.getDate()).padStart(2, '0')
      const dateText = `${y}-${m}-${d}`
      if (day === 0 && date.getMonth() !== lastMonth && week < 52) {
        monthLabels.push(<text x={offsetX + week * (cellWidth + cellGap)} y="14" key={`month-${week}`}>{monthNames[date.getMonth()]}</text>)
        lastMonth = date.getMonth()
      }
      const stats = data.dailyStats[dateText] ?? { submissions: 0, ac: 0 }
      const x = (offsetX + week * (cellWidth + cellGap)).toFixed(1)
      const cy = (offsetY + day * (cellWidth + cellGap)).toFixed(1)
      cells.push(
        <rect className="ac-day-cell" x={x} y={cy} width={cellWidth} height={cellWidth} fill={colorFor(stats.submissions)} key={dateText}>
          <title>{`${dateText} (${weekdayNames[date.getDay()]})\n${stats.submissions ? `${stats.submissions} 次提交 · ${stats.ac} 题通过` : '没有提交记录'}`}</title>
        </rect>,
      )
    }
  }

  const svgWidth = (offsetX + 53 * (cellWidth + cellGap) + 12).toFixed(1)
  const svgHeight = (offsetY + 7 * (cellWidth + cellGap) + 6).toFixed(1)
  return (
    <div className="activity-calendar-wrapper">
      <div className="ac-header">
        <div className="ac-header-left">
          <div className="ac-title">Activity Calendar</div>
          <div className="ac-subtitle">最近一年累计提交 <strong>{data.totalSubmissions}</strong> 次 · 活跃打卡 <strong>{data.activeDays}</strong> 天 · 最长连续 <strong>{data.maxStreak}</strong> 天</div>
        </div>
        <div className="ac-header-right">已通过 <span className="ac-stat-num">{data.totalSolved}</span> 题</div>
      </div>
      <div className="ac-graph-container">
        <svg className="ac-svg" viewBox={`0 0 ${svgWidth} ${svgHeight}`}>
          {monthLabels}
          <text x="6" y={(offsetY + 1 * (cellWidth + cellGap) + 9.5).toFixed(1)}>一</text>
          <text x="6" y={(offsetY + 3 * (cellWidth + cellGap) + 9.5).toFixed(1)}>三</text>
          <text x="6" y={(offsetY + 5 * (cellWidth + cellGap) + 9.5).toFixed(1)}>五</text>
          {cells}
        </svg>
      </div>
      <div className="ac-footer">
        <span>统计周期：过去 365 天</span>
        <div className="ac-legend"><span>少</span>{colors.map((color) => <span className="ac-legend-box" style={{ background: color }} key={color} />)}<span>多</span></div>
      </div>
    </div>
  )
}
