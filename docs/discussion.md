# 讨论 API 与页面

## 接口

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/v1/discussions` | 讨论列表 |
| GET | `/api/v1/discussion-nodes` | Hydro 讨论节点 |
| POST | `/api/v1/discussions` | 创建讨论主题 |
| GET | `/api/v1/discussions/:did` | 讨论详情 |
| GET | `/api/v1/discussions/:did/replies` | 讨论回复列表 |
| POST | `/api/v1/discussions/:did/replies` | 发布讨论回复 |

讨论节点、主题和回复分别读取 Hydro `document` 集合中的 `docType=20`、`docType=21`、`docType=22` 文档。节点使用字符串 `docId`，主题通过 `parentType=20`、`parentId=<节点 docId>` 关联。

创建主题需要登录 Session，请求体格式：

```json
{
  "nodeId": "qa",
  "title": "讨论标题",
  "content": "支持 Markdown 的正文",
  "pin": false,
  "highlight": false
}
```

标题最大 200 字符，正文最大 64 KiB。置顶和高亮仅允许管理员设置。

发布回复需要登录 Session，请求体格式：

```json
{"content": "回复内容，支持 Markdown"}
```

回复正文不能为空，最大 64 KiB。`lock=true` 的讨论返回 HTTP 409，未登录用户返回 HTTP 401。

## 前端

页面结构和类名直接对应 Hydro `ui-default` 模板：

- `discussion_main_or_node.html`：列表、创建入口和节点侧栏
- `discussion_create.html`：创建页
- `partials/discussion_edit_form.html`：标题、置顶、高亮和正文表单
- `components/md_hint.html`：Markdown 提示侧栏
- `discussion_detail.html`：详情和回复列表

路由：

- `/discuss`：讨论列表和节点筛选
- `/discuss/new`：创建主题
- `/discuss/:did`：讨论详情、主回复和楼中楼回复

登录用户可在详情页回复，支持 `Ctrl+Enter` 提交。

## 尚未完成

- 编辑、删除主题和回复
- 点赞、收藏、历史版本
- 回复通知和提及
- 节点管理

## 验证

本地 Docker MongoDB 集成测试覆盖：

- 节点读取和分组所需字段
- 未登录发帖被拒绝
- 无效节点、空标题被拒绝
- 创建主题并写入 `docType=21`
- 未登录回复被拒绝
- 登录用户发布回复
- 回复列表读取
- 回复计数和更新时间同步
- 空回复被拒绝
- 锁定讨论拒绝回复
