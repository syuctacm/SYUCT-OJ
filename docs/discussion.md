# 讨论 API 与页面

## 接口

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/v1/discussions` | 讨论列表 |
| GET | `/api/v1/discussions/:did` | 讨论详情 |
| GET | `/api/v1/discussions/:did/replies` | 讨论回复列表 |
| POST | `/api/v1/discussions/:did/replies` | 发布讨论回复 |

讨论和回复分别读取 Hydro `document` 集合中的 `docType=21`、`docType=22` 文档。回复通过 `parentType=21`、`parentId=<讨论内部 ID>` 关联到主题；发布回复时会同步递增主题的 `nReply` 并更新 `updateAt`。

发布回复需要登录 Session，请求体格式：

```json
{"content": "回复内容，支持 Markdown"}
```

回复正文不能为空，最大 64 KiB。`lock=true` 的讨论返回 HTTP 409，未登录用户返回 HTTP 401。

## 前端

- `/discuss`：讨论列表
- `/discuss/:did`：讨论详情、主回复和楼中楼回复
- 登录用户可在详情页回复，支持 `Ctrl+Enter` 提交

列表支持置顶排序，详情展示 Markdown 内容。当前回复按写入时间倒序展示。

## 尚未完成

- 发帖和讨论节点选择
- 编辑、删除回复
- 点赞、收藏、历史版本
- 回复通知和提及
- 题解和讨论节点分类

## 验证

本地 Docker MongoDB 集成测试覆盖：

- 未登录回复被拒绝
- 登录用户发布回复
- 回复列表读取
- 回复计数和更新时间同步
- 空回复被拒绝
- 锁定讨论拒绝回复
