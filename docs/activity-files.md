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

- `/contest/:tid/files`
- `/homework/:tid/files`
- `/training/:tid/files`

文件页复用 Hydro `partials/files.html` 的 `.data-table` 结构，支持多文件上传、下载、批量选择和删除。比赛/作业/训练详情页在所有者或管理员可见时显示文件入口。

## 尚未完成

- 比赛私有文件 `privateFiles`
- Markdown 编辑器图片上传接入这些文件接口
- 文件拖拽上传和旧站批量选择脚本
