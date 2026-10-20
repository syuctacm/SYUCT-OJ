# 题目 API

## 接口

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/v1/problems` | 题目列表、分页、搜索和筛选 |
| GET | `/api/v1/problems/:pid` | 题目详情，支持 `pid`、`docId` 或 ObjectId |
| POST | `/api/v1/problems` | 创建题目 |
| GET | `/api/v1/problems/:pid/edit` | 编辑数据和原始 YAML 配置 |
| PATCH | `/api/v1/problems/:pid` | 更新题目和判题配置 |
| GET | `/api/v1/problems/:pid/assets` | 测试数据和附加文件列表 |
| POST | `/api/v1/problems/:pid/assets` | multipart 上传文件 |
| DELETE | `/api/v1/problems/:pid/assets` | 批量删除文件 |
| GET | `/api/v1/problems/:pid/assets/download` | 下载文件 |

## 列表参数

- `page`：默认 `1`
- `limit`：默认 `20`，最大 `100`
- `q`：按标题或题号模糊搜索
- `tag`：按标签筛选
- `difficulty`：按难度筛选
- `sort`：`default` 或 `recent`

登录请求会为每题附带 `userStatus`，包含当前用户最近一次提交的 `status` 和 `rid`。

只返回 `domainId=system`、`docType=10` 且未隐藏的题目。

## 数据来源

直接读取 Hydro MongoDB 的 `document` 集合，不复制题目到新表。用户提交状态读取同库的 `record` 集合。

## 前端

题库页结构和类名对应 Hydro `ui-default`：

- `problem_main.html`：搜索栏、排序、题库表格和侧栏
- `partials/problem_list.html`：分页、题目标题、标签、状态、AC/Tried 和难度列
- `components/problem.html`：题目链接和标签 DOM

搜索、排序、分页、标签筛选和用户提交状态均通过 URL 与 API 参数保持。

题目详情和提交页对应：

- `problem_detail.html`：题目标题、标签信息、Markdown 内容
- `problem_submit.html`：语言选择、代码输入和提交按钮
- `partials/problem_sidebar.html`：提交入口和题目信息侧栏

详情 API 额外返回 `problemType`、`timeLimit` 和 `memoryLimit`，由 Hydro 题目 `config` 生成展示信息。

题目创建和编辑页由 `HydroTemplate` 直接渲染上游 `problem_edit.html`：

- `/p/new`
- `/p/:pid/edit`
- PID、标题、标签、难度、可见性、Markdown 题面和原版分类/文件侧栏
- Markdown 编辑器上传写入题目附加文件，并刷新原版附加文件侧栏
- 管理员可创建，管理员或题目所有者可编辑
- React 只负责数据、提交和上传接入，不复制主表单 DOM

判题配置页对应 `problem_config.html`：

- `/p/:pid/config`
- 直接编辑 YAML 配置
- 保存前校验 YAML 对象

题目文件页对应 `problem_files.html` / `partials/problem_files.html`：

- `/p/:pid/files`
- 测试数据与附加文件分栏
- 多文件上传、下载和批量删除
- 仅管理员或题目所有者可访问

## 文件存储

存储布局与 Hydro 对齐：

```text
problem/system/<docId>/testdata/<name>
problem/system/<docId>/additional_file/<name>
```

上传 `config.yaml` 或 `config.yml` 时，内容会同步写入题目的 `config` 字段，供判题机读取。编辑接口会将 Hydro 的 `- key: ... / value: ...` 配置列表规范化为 YAML 对象，避免旧数据编辑后无法保存。

## 已知差异

- 题目删除 API 尚未实现，编辑页暂不展示原模板删除按钮
- 题目分类配置尚未从 Go 设置接口读取，分类侧栏目前为空
- 题目文件重命名和批量选择脚本尚未接入

## 响应

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "items": [],
    "total": 0,
    "page": 1,
    "limit": 20
  }
}
```
