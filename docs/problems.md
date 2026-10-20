# 题目 API

## 接口

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/v1/problems` | 题目列表、分页、搜索和筛选 |
| GET | `/api/v1/problems/:pid` | 题目详情，支持 `pid`、`docId` 或 ObjectId |

## 列表参数

- `page`：默认 `1`
- `limit`：默认 `20`，最大 `100`
- `q`：按标题或题号模糊搜索
- `tag`：按标签筛选
- `difficulty`：按难度筛选

只返回 `domainId=system`、`docType=10` 且未隐藏的题目。

## 数据来源

直接读取 Hydro MongoDB 的 `document` 集合，不复制题目到新表。

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
