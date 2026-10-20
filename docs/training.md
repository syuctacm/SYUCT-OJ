# 训练 API 与页面

## 接口

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/v1/trainings` | 训练列表 |
| GET | `/api/v1/trainings/:tid` | 训练详情 |

数据直接读取 Hydro `document` 集合中的 `docType=40` 文档。

## 前端

- `/training`：训练列表
- `/training/:tid`：训练详情

列表支持置顶排序，详情展示训练说明。

## 尚未完成

- 训练 DAG 和题目依赖展示
- 训练进度和通过状态
- 训练题目跳转和训练内提交
- 训练管理页面
