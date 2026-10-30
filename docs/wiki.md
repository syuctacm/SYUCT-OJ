# 关于与帮助页面

- `/about`：复用 Hydro `about.html` 和 `wiki_base.html`，展示单站点介绍、联系方式和后台说明
- `/wiki/help`：使用同一 wiki 布局的单站点帮助内容，覆盖提交、评测状态和账号使用
- 页脚“关于 / 帮助 / 联系我们”分别指向 `/about`、`/wiki/help`、`/about#contact`

这些页面是静态内容，不读取用户数据或敏感配置。
