# 训练 API 与页面

## 接口

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/v1/trainings` | 训练列表 |
| POST | `/api/v1/trainings` | 创建训练 |
| GET | `/api/v1/trainings/:tid` | 训练详情、DAG 章节、题目和当前用户进度 |
| GET | `/api/v1/trainings/:tid/edit` | 训练编辑数据 |
| PATCH | `/api/v1/trainings/:tid` | 更新训练 |
| POST | `/api/v1/trainings/:tid/enroll` | 报名训练 |

数据读取 Hydro `document` 集合中的 `docType=40` 文档，章节保存在 `dag` 字段。训练兼容数字型和数字字符串形式的 `pid`。

## 进度规则

- 章节按 DAG 顺序计算。
- 前置章节 `requireNids` 未完成时，章节状态为 `invalid`。
- 章节内全部题目有 Accepted 记录时状态为 `done`。
- 有提交记录但尚未全部通过时状态为 `progress`。
- 尚未开始且前置条件满足时状态为 `open`。
- 题目状态从 `record` 集合按当前用户、题目和判题状态实时计算。

## 报名

报名需要登录 Session。首次报名会向 `document.status` 写入：

```text
domainId=system
docType=40
docId=<训练 docId>
uid=<用户 ID>
enroll=1
```

同时递增训练的 `attend`。重复报名是幂等操作，不会重复增加人数。

## 前端

页面结构对应 Hydro `ui-default`：

- `training_main.html`：训练列表、搜索、章节/题目数和报名人数
- `training_detail.html`：训练说明、状态、章节和题目
- `training_edit.html`：训练创建和编辑表单
- `partials/training_detail.html`：DAG 章节状态、前置条件和题目表格

路由：

- `/training`：训练列表和标题搜索
- `/training/new`：创建训练
- `/training/:tid/edit`：编辑训练
- `/training/:tid`：训练说明、总体进度、DAG 章节、题目完成状态和报名入口
- `/training/:tid/files`：训练公开文件

创建和编辑使用管理员权限；编辑还需要训练所有者或管理员。DAG JSON 会校验节点 ID 唯一、前置节点存在、每节至少一道题，并把 PID 或 docId 统一解析为题目 docId。

未报名或章节未解锁时仍可查看内容，但题目入口不可点击。页面直接使用 Hydro 原版 `.training__section`、`.training__problems` 和训练状态类名。训练创建/编辑页由 `HydroTemplate` 直接渲染上游 `training_edit.html`，Markdown 图片上传已接入训练文件接口。

## 尚未完成

- 训练私有文件
- 他人进度对比和完成排行榜
- 训练通知

## 验证

本地 Docker MongoDB 集成测试覆盖：

- DAG 数字/字符串 pid 解析
- 未报名状态和章节解锁状态
- 未登录报名被拒绝
- 首次报名写入状态并递增人数
- 重复报名幂等
- 提交记录驱动 done/progress/open/invalid 状态
- 全部章节完成后进度为 100%
- 管理员创建、所有者/管理员编辑
- DAG 节点、前置条件、PID/docId 和缺失题目校验
