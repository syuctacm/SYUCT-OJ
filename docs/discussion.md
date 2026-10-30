# 讨论 API 与页面

## 接口

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/v1/discussions` | 讨论列表（支持 `parentType`/`parentId` 过滤） |
| GET | `/api/v1/discussion-nodes` | 全局 Hydro 讨论节点 |
| GET | `/api/v1/discussion-nodes/:type/:name` | 全局/比赛/作业/训练/题目虚拟节点 |
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

全局讨论节点、主题和回复分别读取 Hydro `document` 集合中的 `docType=20`、`docType=21`、`docType=22` 文档。比赛、作业和训练讨论使用虚拟节点：`parentType` 分别为 30/30/40，`parentId` 保存对应 ObjectId；全局节点继续使用字符串 `docId`。

创建主题需要登录 Session，请求体格式：

```json
{
  "nodeType": "node",
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
- `/discuss/node/:name`：全局讨论节点列表（兼容路由）
- `/discuss/:type/:name`：`node`、`contest`、`homework`、`training`、`problem` 讨论节点
- `/discuss/:type/:name/create`：在指定节点创建主题
- `/discuss?node=<name>`：兼容旧链接并跳转到节点路由
- `/discuss/new?node=<name>`：兼容旧链接并跳转到上游路由
- `/discuss/:did/edit`：编辑主题
- `/discuss/:did`：讨论详情、主回复和楼中楼回复

讨论列表由 `HydroTemplate` 直接渲染上游 `discussion_main_or_node.html`：列表项显示回复数、节点标签、浏览量和作者，右侧是创建入口和节点分组侧栏。节点页只显示该节点下的讨论，节点标签和创建入口都指向上游路由。

讨论详情由 `HydroTemplate` 直接渲染上游 `discussion_detail.html` 和 `components/comments_discussion.html`：话题头部（节点标签、标题、作者、时间）、正文、操作区（编辑/锁定/举报/查看数）、回复列表和作者侧栏都来自上游模板。适配层接管主回复、楼中楼回复（`replyTo`）、回复编辑/删除、锁定切换和主题收藏（`document.status` docType=21）；楼中楼回复只更新话题时间，不增加 `nReply`，与上游一致。

登录用户首次打开主题时会写入 `document.status.view=true` 并递增主题 `views`；同一用户刷新不会重复计数，匿名访问不计数，与上游 Hydro 的统计规则一致。点击回复按钮会预填被回复作者的 `@[](/user/<uid>)` 提及标记；主回复和楼中楼回复都会解析该标记，并向真实存在且不是发送者本人的用户发送 `message` 集合中的未读富文本系统消息。

话题和每条主回复都带上游 `.reactions` 反应条：点击已有反应可切换，侧栏的 `data-op="react"` 按钮会打开 ui-default 的 Mantine `Popover` emoji 面板（`web/src/components/DiscussionReaction.tsx`，从上游 `components/discussion/reaction.page.tsx` 移植）。未登录点击反应条会跳转登录页。

登录用户可在详情页回复，支持 `Ctrl+Enter` 提交。讨论创建/编辑页由 `HydroTemplate` 直接渲染上游 `discussion_create.html`、`discussion_edit.html` 和 `partials/discussion_edit_form.html`，React 只负责加载数据、提交/删除和快捷键挂载；原版删除按钮会调用现有 DELETE API。创建入口只在该讨论节点页面出现，与原站一致。

### 图片上传

创建主题、编辑主题、主回复以及动态打开的楼中楼回复/编辑框都挂载原版 Markdown 编辑器，并开启图片按钮。上传使用个人文件接口 `POST /api/v1/user/files`，沿用「我的文件」的存储和配额；编辑器会生成随机文件名，避免多次上传同名图片时冲突。

上传成功后编辑器插入个人文件的公开下载地址：

```text
/api/v1/users/<uid>/files/<filename>
```

该地址无需登录即可访问，便于讨论图片对匿名访客展示；上传、列表和删除接口仍然要求本人 Session。旧的 `GET /api/v1/user/files/:name` 保留为登录用户访问自己文件的兼容入口。

## 尚未完成

- 节点管理（创建、编辑和删除讨论节点）

## 历史版本

每次创建/编辑主题、主回复和楼中楼都会往 `discussion.history` 集合写一条 `{docId, content, uid, time}`（尽力而为，写入失败不影响正文更新）。

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/v1/discussions/:did/raw` | 当前正文（原文） |
| GET | `/api/v1/discussions/:did/raw?all=1` | 版本列表（`uid` + `time`） |
| GET | `/api/v1/discussions/:did/raw?time=<ms>` | 指定版本正文 |
| GET | `/api/v1/discussions/:did/raw?docId=<id>` | 指定回复/楼中楼的正文与历史 |

详情页的 `[data-discussion-history]` 由 `web/src/components/DiscussionHistory.tsx` 接管（移植自上游 `components/discussion/history.page.tsx`）：悬停/点击「编辑于」弹出 Mantine Popover 列出各版本，选中后在本站对话框里按 Markdown 渲染当时的内容。

## 反应

| 方法 | 路径 | 用途 |
|---|---|---|
| POST | `/api/v1/discussions/:did/reaction` | 添加或取消主题/主回复的 emoji 反应 |

请求体对齐上游 `operation=reaction`：

```json
{"nodeType": "did", "id": "<docId>", "emoji": "👍", "reverse": false}
```

`nodeType` 为 `did`（主题）或 `drid`（主回复）。计数写入文档的 `react` 映射，用户自身的反应写入 `document.status` 的 `react.<emoji>`，与 Hydro `discussion.react` 语义一致：重复提交不会重复计数，取消后计数减一，最后一个反应移除时清空 `react` 字段。详情接口返回 `reactions`（当前用户自己的反应映射），模板据此给 emoji 加 `active`。

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
- 主题/回复反应的增删、幂等与计数清理
- 未登录、非法 emoji、非法 nodeType 和跨讨论回复的拒绝
- 回复删除后的 nReply 与嵌套结构校验
