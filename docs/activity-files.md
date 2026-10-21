# 比赛 / 作业 / 训练文件

## 接口

比赛：

- `GET /api/v1/contests/:tid/files`
- `POST /api/v1/contests/:tid/files`
- `DELETE /api/v1/contests/:tid/files`
- `GET /api/v1/contests/:tid/files/:name`

作业：

- `GET /api/v1/homeworks/:tid/files`
- `POST /api/v1/homeworks/:tid/files`
- `DELETE /api/v1/homeworks/:tid/files`
- `GET /api/v1/homeworks/:tid/files/:name`

训练：

- `GET /api/v1/trainings/:tid/files`
- `POST /api/v1/trainings/:tid/files`
- `DELETE /api/v1/trainings/:tid/files`
- `GET /api/v1/trainings/:tid/files/:name`

上传使用 multipart，字段名为 `files`，兼容单文件字段 `file`。列表和下载公开；上传、删除要求资源所有者或管理员。

## 存储

```text
contest/system/<tid>/public/<name>
training/system/<tid>/<name>
```

比赛和作业共用 `contest` 路径。文件元数据分别写入比赛/作业文档或训练文档的 `files` 数组，格式与 Hydro `FileInfo` 对齐。

限制：

- 最多 100 个文件
- 单次上传请求最大 256 MiB
- multipart 内存缓冲 32 MiB
- 文件名不允许路径穿越或空文件名

## 前端

- `/contest/:tid/files`（仍是手写页，后续迁移到 `contest_manage.html`）
- `/homework/:tid/file`（旧路径 `/homework/:tid/files` 保留为别名）
- `/training/:tid/file`（旧路径 `/training/:tid/files` 保留为别名）

作业文件页直接渲染上游 `homework_files.html`，训练文件页直接渲染 `training_files.html`，两页都包含 `partials/files.html`，因此上传、下载、批量选择和删除的 DOM 与旧站一致；React 只负责事件适配。比赛文件页仍是手写实现，等私有资料接口就绪后与 `contest_manage.html` 一起迁移。

比赛、作业、训练的编辑页已把 Markdown 编辑器上传按钮接到对应文件接口：上传成功后插入文件下载 URL，并刷新比赛编辑页的原版文件侧栏。

## 尚未完成

- 比赛私有文件 `privateFiles`
- 文件拖拽上传和旧站批量选择脚本
