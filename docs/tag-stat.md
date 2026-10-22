# 标签统计与标签榜单

## 接口

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/v1/problem-tags` | 标签题目数统计 |
| GET | `/api/v1/problem-tags/:tag/ranking` | 标签过题榜单、搜索和分页 |
| GET | `/api/v1/problem-tags/:tag/ranking/export` | 导出完整榜单 CSV（支持 `q` 过滤） |

标签统计聚合 Hydro `document` 集合中未隐藏题目的 `tag` 字段。来源标签排在算法标签之后。

标签榜单按用户在该标签题目上的 Accepted 数排序，提交数从 `record` 集合统计。

## 前端

- 题库侧栏 `/p`：标签统计、展开/收起和榜单入口
- `/p/tag/:tag`：标签榜单、标签切换、账号搜索、分页和 CSV 导出

页面直接渲染生产 `tag-stat` 插件的原始模板：

- `web/src/upstream/templates/problem_main.html`：题库页标签统计侧栏
- `web/src/upstream/templates/partials/category.html`：标签统计、展开/收起和问题标签显示切换
- `web/src/upstream/templates/problem_tag_stat.html`：标签榜单、搜索、分页和导出入口
- `web/src/upstream/render.ts` 将这三份模板注册到本地 Nunjucks 环境
- React 只负责 API 取数、URL 映射、搜索提交和交互事件，不复制模板 DOM

CSV 导出沿用插件的列定义和 UTF-8 BOM，并输出排名、UID、账号名、过题数、提交数、通过率和标签。

## 验证

本地 Docker MongoDB 集成测试覆盖：

- 标签聚合计数
- 标签题目数和上榜人数
- 用户过题数排序
- 用户名搜索
- 无匹配用户时返回空榜单而不是 500
- CSV BOM、表头、过滤结果和通过率
- 超出末页时回退到最后一页
