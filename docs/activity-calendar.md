# 活跃日历

## 接口

```text
GET /api/v1/users/:uid/activity
```

返回最近 365 天数据：

- `dailyStats`：按日期记录提交数和通过题目数
- `totalSubmissions`
- `activeDays`
- `maxStreak`
- `totalSolved`

日期按 `Asia/Shanghai` 计算，通过题目按 PID 去重。

## 前端

用户主页的“活跃日历”标签使用原 Activity Calendar 插件的 53 周 SVG 布局、颜色分级和悬停提示，组件位于 `web/src/components/ActivityCalendar.tsx`。原插件 CSS 从旧站用户主页内嵌脚本原样提取到 `web/src/legacy/activity-calendar.css`，不手工改写。

## 验证

本地 Docker MongoDB 集成测试覆盖提交数、活跃天数、连续天数和去重通过题目数。
