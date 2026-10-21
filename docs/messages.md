# 站内消息

## 接口

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/v1/messages` | 按会话分组返回最近的私信，并把当前用户的未读计数清零 |
| POST | `/api/v1/messages` | 发送私信，body 为 `{uid, content}` |
| DELETE | `/api/v1/messages/:mid` | 删除自己发出的消息 |

未登录访问返回 `401`；给不存在的用户发送返回 `404`。

## 数据来源

直接读写 Hydro 的 `message` 集合，不复制数据：

```js
{ _id, from, to: [uid], content, flag }
```

`flag` 沿用上游语义：`1` 未读、`2` 警告、`4` 富文本、`8` 信息、`16` 需要翻译。发送 `flag=1` 的消息会给接收者 `unreadMsg` 加一，打开消息页后清零，与上游 `MessageModel` 一致。

## 前端

`/home/messages` 由 `HydroTemplate` 渲染上游 `home_messages.html`（账号侧栏 + `#messagePad` 挂载点），messagepad 本身是原生 React 复刻：

- `web/src/components/MessagePad.tsx`：会话列表、消息流、输入框
- DOM 结构与类名与上游 `components/messagepad/*` 保持一致（`.messagepad__list-item`、`.messagepad__message.side--self/--other` 等），因此直接复用 ui-default 样式
- 支持 `?target=<uid>` 直接打开某个会话
- “新消息”按钮打开用户搜索对话框（复用 `/api/v1/users?search=`）
- 系统消息（`from=1`）的 `{message, params}` 占位符会被替换后展示

## 已知差异

- 上游通过 WebSocket 推送新消息并弹通知；重写版本改为页面打开时每 10 秒轮询，没有实时推送和弹窗
- 上游消息删除入口在当前 UI 中并未暴露（只有接口），重写保持一致，只提供 API
- 发送权限：上游要求 `PRIV_SEND_MESSAGE`，重写沿用项目现有的“登录即可”策略
