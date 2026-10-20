# 讨论 API 与页面

## 接口

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/v1/discussions` | 讨论列表 |
| GET | `/api/v1/discussions/:did` | 讨论详情 |

数据读取 Hydro `document` 集合中的 `docType=21` 文档。

## 前端

- `/discuss`：讨论列表
- `/discuss/:did`：讨论详情

列表支持置顶排序，详情展示 Markdown 内容。

## 尚未完成

- 回复列表和回复编辑
- 发帖和权限控制
- 点赞、收藏、历史版本
- 题解和讨论节点分类
