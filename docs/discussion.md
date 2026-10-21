# 讨论 API 与页面

## 接口

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/v1/discussions` | 讨论列表（支持 `parentType`/`parentId` 过滤） |
| GET | `/api/v1/discussion-nodes` | Hydro 讨论节点 |
| POST | `/api/v1/discussions` | 创建讨论主题 |
| PATCH | `/api/v1/discussions/:did` | 编辑讨论主题 |
| DELETE | `/api/v1/discussions/:did` | 删除讨论主题及回复 |
| PATCH | `/api/v1/discussions/:did/lock` | 锁定/解锁讨论 |
| POST | `/api/v1/discussions/:did/star` | 收藏/取消收藏主题 |
| GET | `/api/v1/discussions/:did` | 讨论详情 |
| GET | `/api/v1/discussions/:did/replies` | 讨论回复列表 |
| POST | `/api/v1/discussions/:did/replies` | 发布讨论回复（带 `replyTo` 时为楼中楼回复） |
| PATCH | `/api/v1/discussions/:did/replies/:rid` | 编辑主回复或楼中楼回复 |
| DELETE | `/api/v1/discussions/:did/replies/:rid` | 删除主回复或楼中楼回复 |

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

编辑主题请求体：

```json
{"title": "新标题", "content": "新正文", "pin": false, "highlight": false}
```

所有者可以编辑/删除自己的主题，管理员可以编辑/删除任意主题。置顶、高亮和锁定仅管理员可操作。删除主题会同时删除其回复。

回复作者可以编辑/删除自己的回复，管理员可以编辑/删除任意回复；主题所有者也可以删除主题下的回复。删除主回复会递减主题的 `nReply`，删除楼中楼回复只会移除对应嵌套项。

## 前端

页面结构和类名直接对应 Hydro `ui-default` 模板：

- `discussion_main_or_node.html`：由 `HydroTemplate` 直接渲染列表、创建入口和节点侧栏
- `discussion_create.html`：创建页
- `partials/discussion_edit_form.html`：标题、置顶、高亮和正文表单
- `components/md_hint.html`：Markdown 提示侧栏
- `discussion_detail.html`：由 `HydroTemplate` 直接渲染话题头部、正文、回复区和作者侧栏

路由：

- `/discuss`：全部讨论列表
- `/discuss/node/:name`：讨论节点列表（上游路由）
- `/discuss/node/:name/create`：在指定讨论节点创建主题（上游路由）
- `/discuss?node=<name>`：兼容旧链接并跳转到节点路由
- `/discuss/new?node=<name>`：兼容旧链接并跳转到上游路由
- `/discuss/:did/edit`：编辑主题
- `/discuss/:did`：讨论详情、主回复和楼中楼回复

讨论列表由 `HydroTemplate` 直接渲染上游 `discussion_main_or_node.html`：列表项显示回复数、节点标签、浏览量和作者，右侧是创建入口和节点分组侧栏。节点页只显示该节点下的讨论，节点标签和创建入口都指向上游路由。

讨论详情由 `HydroTemplate` 直接渲染上游 `discussion_detail.html` 和 `components/comments_discussion.html`：话题头部（节点标签、标题、作者、时间）、正文、操作区（编辑/锁定/举报/查看数）、回复列表和作者侧栏都来自上游模板。适配层接管主回复、楼中楼回复（`replyTo`）、回复编辑/删除、锁定切换和主题收藏（`document.status` docType=21）；楼中楼回复只更新话题时间，不增加 `nReply`，与上游一致。

登录用户可在详情页回复，支持 `Ctrl+Enter` 提交。讨论创建/编辑页由 `HydroTemplate` 直接渲染上游 `discussion_create.html`、`discussion_edit.html` 和 `partials/discussion_edit_form.html`，React 只负责加载数据、提交/删除和快捷键挂载；原版删除按钮会调用现有 DELETE API。创建入口只在该讨论节点页面出现，与原站一致。

## 尚未完成

- 问题/比赛/训练维度的讨论节点路由尚未接入
- 反应（emoji）、历史版本和行内 raw 查看
- 浏览量暂时不会随访问自增
- 回复通知和提及
- 节点管理
- 讨论编辑器暂不提供图片上传，只能插入外链图片

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
- 所有者/管理员编辑与删除
- 非所有者编辑被拒绝
- 置顶/高亮/锁定仅管理员可操作
- 主回复和楼中楼回复编辑/删除
- 回复删除后的 nReply 与嵌套结构校验
