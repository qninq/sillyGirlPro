# API 接口与存储引擎

[返回项目导航](../README.md) · [界面截图](screenshots.md) · [插件编写](plugin-development.md) · [适配器](adapters.md)

本文档集中说明 SillyGirl 的 REST/gRPC 接口、认证与错误契约，以及 BoltDB/Redis 存储引擎。插件运行时 API 见[插件开发指南](plugin-development.md)。

## 目录

- [REST API](#rest-api)
- [存储引擎](#存储引擎)
- [gRPC API](#grpc-api)
- [接口验证](#接口验证)

## REST API

Base URL: `http://host:port/api`

### 约定

- 资源名使用小写复数名词和 kebab-case；资源标识放在路径参数中。
- API 业务路由只使用 `GET` 和 `POST`：`GET` 安全读取；`POST` 创建资源、提交资源表示或创建处理任务。
- 对现有资源的变更使用 `POST /resources/:id`；删除被建模为 deletion 子资源，使用 `POST /resources/:id/deletions`，不使用动作式 URL。
- 分页、筛选和可选关联数据使用查询参数；不使用 `/list`、动词路径、扩展名路由或 `?id=` 定位单资源。
- 被替换的旧路由不保留兼容别名。
- 创建成功返回 `201 Created` 和新资源的 `Location`；异步任务返回 `202 Accepted`；删除和注销成功返回 `200 OK`，`data` 为 `null`。
- 不存在、冲突、语义校验失败和服务端异常分别返回 `404`、`409`、`422`、`500`，不再用 `200` 包装错误。

除文件下载、图片、静态页面及机器人协议端点等原始内容接口外，所有 REST JSON 响应都严格使用统一 envelope，顶层只包含 `status`、`message`、`data`：

```json
{
  "status": true,
  "message": "成功",
  "data": {}
}
```

错误响应保留对应的 HTTP 状态码，`status` 固定为 `false`；字段级错误等附加信息统一放入 `data`：

```json
{
  "status": false,
  "message": "字段校验失败",
  "data": {
    "errors": { "schedule": "Cron 表达式格式错误" }
  }
}
```

需要登录的 Admin 和 User 资源通过请求头 `token: <JWT>` 认证。登录和注册接口响应体返回 JWT，前端保存后在后续请求中写入该请求头。`EventSource` 等无法自定义请求头的客户端可将 JWT 放在 `token` 查询参数中（如实时日志流 `/api/admin/logs/stream?token=<JWT>`），服务端在校验请求头为空时回退读取该参数。

### Admin 认证与系统资源

| Method | Resource | 说明 |
|---|---|---|
| `GET`, `POST` | `/api/admin/setup` | 查询或完成首次初始化 |
| `POST` | `/api/admin/sessions` | 创建管理会话 |
| `GET` | `/api/admin/sessions/current` | 读取当前管理会话 |
| `POST` | `/api/admin/sessions/current/deletions` | 创建当前管理会话的删除记录并结束会话 |
| `GET`, `POST` | `/api/admin/settings` | 读取或提交系统设置单例 |
| `POST` | `/api/admin/system-update-jobs` | 创建更新任务 |
| `GET` | `/api/admin/system-update-jobs/:id` | 读取更新任务 |
| `POST` | `/api/admin/system-restart-jobs` | 创建重启任务 |
| `GET` | `/api/admin/system-restart-jobs/:id` | 读取重启任务 |
| `GET` | `/api/admin/system-backups/current` | 下载当前系统备份 |
| `POST` | `/api/admin/clawbot-login-sessions` | 创建 ClawBot 登录会话 |
| `GET` | `/api/admin/clawbot-login-sessions/:session` | 读取登录状态 |
| `POST` | `/api/admin/clawbot-login-sessions/:session/verification-attempts` | 提交验证码尝试 |

### Admin 业务资源

| Method | Resource |
|---|---|
| `GET`, `POST` | `/api/admin/users` |
| `POST` | `/api/admin/users/:username` |
| `POST` | `/api/admin/users/:username/deletions` |
| `GET`, `POST` | `/api/admin/tasks` |
| `POST` | `/api/admin/tasks/:task_id` |
| `POST` | `/api/admin/tasks/:task_id/deletions` |
| `GET` | `/api/admin/task-options?task_id=:task_id` |
| `POST` | `/api/admin/tasks/:task_id/executions` |
| `GET`, `POST` | `/api/admin/replies` |
| `POST` | `/api/admin/replies/:id` |
| `POST` | `/api/admin/replies/:id/deletions` |
| `GET`, `POST` | `/api/admin/carry-groups` |
| `POST` | `/api/admin/carry-groups/:chat_id` |
| `POST` | `/api/admin/carry-groups/:chat_id/deletions` |
| `GET` | `/api/admin/carry-group-options` |
| `GET`, `POST` | `/api/admin/masters` |
| `POST` | `/api/admin/masters/:platform/:number/deletions` |
| `GET`, `POST` | `/api/admin/panels` |
| `POST` | `/api/admin/panels/:id` |
| `POST` | `/api/admin/panels/:id/deletions` |
| `GET` | `/api/admin/panels/:id/accounts` |
| `POST` | `/api/admin/panel-connection-tests` |
| `POST` | `/api/admin/panel-status-checks` |
| `GET`, `POST` | `/api/admin/plugin-settings/:uuid` |
| `POST` | `/api/admin/plugin-settings/:uuid/deletions` |
| `GET` | `/api/admin/plugin-settings` |
| `POST` | `/api/admin/local-plugins` |
| `GET`, `POST` | `/api/admin/local-plugins/:id` |
| `GET` | `/api/admin/local-plugins/:id/dependents` |
| `POST` | `/api/admin/local-plugins/:id/status` |
| `POST` | `/api/admin/local-plugins/:id/deletions` |
| `GET`, `POST` | `/api/admin/app-scripts` |
| `GET`, `POST` | `/api/admin/app-scripts/:id` |
| `POST` | `/api/admin/app-scripts/:id/deletions` |
| `GET` | `/api/admin/app-scripts/:id/executions` |
| `GET` | `/api/admin/dependencies?runtime=:runtime&plugin=:plugin` |
| `POST` | `/api/admin/dependencies` |
| `POST` | `/api/admin/dependency-deletions` |
| `POST` | `/api/admin/dependency-deletions/:runtime/:plugin/*package` |
| `GET`, `POST` | `/api/admin/dependency-registries/:runtime` |
| `POST` | `/api/admin/dependency-registries/:runtime/options` |
| `POST` | `/api/admin/dependency-registries/:runtime/option-deletions/*registry` |
| `POST` | `/api/admin/scripts` |
| `GET`, `POST` | `/api/admin/scripts/:id` |
| `POST` | `/api/admin/scripts/:id/deletions` |
| `GET`, `POST` | `/api/admin/plugin-market/sources` |
| `POST` | `/api/admin/plugin-market/source-status/*address` |
| `POST` | `/api/admin/plugin-market/source-deletions/*address` |
| `GET` | `/api/admin/plugin-market/plugins` |
| `POST` | `/api/admin/plugin-market-snapshots` |
| `GET`, `POST` | `/api/admin/plugin-market/github-proxy` |
| `POST` | `/api/admin/plugin-market/github-proxy-options` |
| `POST` | `/api/admin/plugin-market/github-proxy-option-deletions/*proxy` |
| `GET`, `POST` | `/api/admin/storage/values` |
| `GET` | `/api/admin/storage/entries` |
| `GET`, `POST` | `/api/admin/storage/bucket-entries` |
| `GET`, `POST` | `/api/admin/storage/buckets` |
| `POST` | `/api/admin/storage/buckets/:bucket/deletions` |
| `POST` | `/api/admin/storage/buckets/:bucket/renames` |
| `GET` | `/api/admin/message-rules/:kind` |
| `POST` | `/api/admin/message-rules/:kind/:key` |
| `POST` | `/api/admin/message-rules/:kind/:key/deletions` |
| `GET` | `/api/admin/bots` |
| `GET` | `/api/admin/command-list` |
| `POST` | `/api/admin/command-list/:key/admin` |
| `POST` | `/api/admin/qqguild-onboard-tasks` |
| `POST` | `/api/admin/qqguild-onboard-tasks/:task_id/polls` |
| `GET` | `/api/admin/logs/stream`（SSE，鉴权支持 `token` 查询参数） |

`GET /api/admin/logs/stream` 为 SSE 实时日志流，鉴权支持 `token` 查询参数回退。

`GET /api/admin/bots` 返回 `settings`（各平台配置值）、`statuses`（平台启停/连接状态/实例列表）与 `events`（各适配器最近事件环形缓冲，含实例注册/销毁与开关变更，每平台保留最近 50 条，内存态重启即清空）。

普通用户账号接口（Admin 用户管理）：

- `GET /api/admin/users` 返回全部普通用户行（含 `email`、`bindings`、`disabled`、`storage_key`），后台页面在此基础上做账号 / 昵称 / 邮箱 / QQ / TGID 的前端过滤搜索。
- `POST /api/admin/users` 创建用户，请求体 `{ "username": "...", "password": "...", "nickname": "...", "email": "...", "qq": "...", "telegram": "...", "disabled": false }`；`email` 非空时仅接受 QQ 邮箱（`QQ号@qq.com`），`qq`/`telegram` 非空时写入对应绑定。
- `POST /api/admin/users/:username` 更新用户，`password` 留空保留原密码，`email` 留空不修改；`qq`/`telegram` 为替换语义（留空解除绑定）。
- `POST /api/admin/users/:username/deletions` 删除用户，同时移除 `users` 桶内 `user:` 与 `bindings:` 两个键。

用户服务（User 资源）说明：

- `POST /api/user/accounts` 为自助注册入口，请求体 `{ "email": "...", "password": "...", "nickname": "...", "code": "..." }`。仅接受 QQ 数字邮箱（规则 `^(\d{5,12})@qq\.com$`），邮箱前缀 QQ 号即登录账号（5-12 位数字），昵称选填（默认使用邮箱）；注册成功自动写入 QQ 绑定，返回 JWT 与用户信息。管理员在基础设置「邮箱验证」配置齐全（服务器地址、端口、授权码、邮箱主体）时，`code` 为必填的邮箱验证码；未配置时保持无验证注册。
- `POST /api/user/email-codes` 请求体 `{ "email": "QQ号@qq.com", "purpose": "register" | "password-reset" }`（`purpose` 缺省为 `register`），向该邮箱发送 6 位验证码（5 分钟有效、验证成功即作废、连续错误 5 次作废）；验证码按用途隔离，注册用途的验证码不能用于重置密码。同 IP 60 秒内只能发送一封（跨用途共享）；`register` 用途要求邮箱未注册，`password-reset` 用途要求邮箱已注册，未配置 SMTP 或格式非法时返回相应错误。
- `POST /api/user/password/resets` 忘记密码：请求体 `{ "email": "QQ号@qq.com", "code": "6 位验证码", "password": "新密码" }`，`code` 为 `password-reset` 用途的邮箱验证码（需先在基础设置配置邮箱验证服务）；校验通过后重置密码并立即作废验证码。
- `POST /api/user/sessions` 使用账号（自助注册用户即 QQ 号）与密码登录，连续失败有频率限制。
- `GET /api/user/profile` 返回当前用户（`user.email` 含注册邮箱）、绑定与用户公告。
- `POST /api/user/bindings/:platform` 与对应 deletions 仅支持 `qq`、`telegram` 两个平台。

`POST /api/admin/qqguild-onboard-tasks` 创建 QQ 官方扫码绑定任务（q.qq.com bind 服务），返回 `task_id` 和 `qr_code_url`；`POST /api/admin/qqguild-onboard-tasks/:task_id/polls` 内部轮询绑定结果（最长 50 秒），确认成功后服务端解密凭据并自动保存 `qqguild.app_id`/`qqguild.app_secret` 且启用适配器，返回 `confirmed` 与凭据。

`GET /api/admin/command-list` 返回全部已注册指令（内置 Go 指令与脚本插件指令），包括消息消费循环中硬编码的内置管理命令（群聊/私聊的 `listen`、`unlisten`/`nolisten`、`reply`、`noreply`/`unreply`，标记 `readonly`），每项含触发正则、用途、来源（`builtin`/`plugin`）和管理员限制，响应 `data.stats` 提供 `builtin`/`plugin`/`adminOnly` 数量统计。`POST /api/admin/command-list/:key/admin` 请求体为 `{ "admin": true|false }`，覆盖脚本头部的 `[admin]` 声明并持久化，立即生效且在重启与插件热重载后保留；`readonly` 的内置命令不可修改。

面板集合使用请求体字段 `type: "qinglong" | "daidai"` 区分具体类型。`GET /api/admin/panels` 一次返回两类面板，不再并发请求 provider 专用接口。

本地插件接口使用文件插件 UUID 作为 `:id`：

- `POST /api/admin/local-plugins/:id/status` 请求体为 `{ "status": true|false }`，只修改源码顶部 `status` 注释并重载插件；`module=true` 的依赖模块没有独立运行开关。
- `GET /api/admin/local-plugins/:id/dependents` 返回同发布者目录内通过 `depe` 引用该模块的插件列表，删除或卸载被引用模块会返回冲突。

插件市场源接口：

- `GET /api/admin/plugin-market/sources` 返回 `[{ "address": "...", "disabled": true|false }]` 对象数组；`POST` 请求体为 `{ "address": "..." }` 新增源。
- `POST /api/admin/plugin-market/source-status/*address` 请求体为 `{ "status": true|false }`（true 为启用），切换源的启用/禁用状态；禁用的源不再参与插件市场加载与展示，禁用列表持久化存储（`plugin_source_disabled` 设置键），删除源时同步清理其禁用记录。

应用脚本接口面向管理后台「插件开发」页面，管理本地应用脚本（消息响应脚本与启动任务/常驻服务/定时任务等独立脚本，以及全部 `.ts`/`.go` 占位文件），页面按语言分类展示全部脚本类型：

- `GET /api/admin/app-scripts` 返回全部应用脚本及语言分类（ES5、Node.js、TypeScript、Python、Golang、Adapter JS、Adapter Python、Adapter Go）；仅 JS/Python 系分类可执行，TypeScript/Golang/Adapter Go 为不可执行占位。
- `POST /api/admin/app-scripts` 请求体为 `{ "name": "...", "language": "node|es5|typescript|python|golang|adapter-js|adapter-python|adapter-go", "content": "..." }`；JS/Python 系走本地插件创建链路（校验元数据并加载），其余仅落盘不加载。
- `:id` 为已加载脚本插件 UUID 或 `file:相对路径` 标识（占位脚本）。
- `GET /api/admin/app-scripts/:id/executions` 为在线调试端点，以 SSE 流式返回脚本单次运行的 stdout/stderr（`data: out ...` / `data: err ...`）与结束事件（`data: exit exit_code=N ...`）；鉴权支持 `token` 查询参数回退（EventSource 无法携带自定义请求头），查询参数 `timeout` 控制超时（默认 120 秒、上限 600 秒），超时或客户端断开自动终止脚本进程。
- 已加载脚本的启停复用 `POST /api/admin/local-plugins/:id/status`（页面「启用/停用」徽章按钮即调用该接口）。
- `POST /api/admin/dependency-deletions` 请求体为 `{ "runtime": "node|python", "plugin": "发布者/插件名|__shared__", "package": "包名" }`。保留带路径参数的旧接口用于兼容；包名或作者路径可能含 `/` 时应使用 JSON 接口。

### User 资源

| Method | Resource |
|---|---|
| `POST` | `/api/user/email-codes` |
| `POST` | `/api/user/password/resets` |
| `POST` | `/api/user/accounts` |
| `POST` | `/api/user/sessions` |
| `POST` | `/api/user/sessions/current/deletions` |
| `GET` | `/api/user/profile` |
| `POST` | `/api/user/bindings/:platform` |
| `POST` | `/api/user/bindings/:platform/deletions` |

### Public 资源

| Method | Resource | 响应 |
|---|---|---|
| `GET` | `/api/health` | 健康状态 |
| `GET` | `/api/public/plugins` | 已开放插件 |
| `GET` | `/api/plugin-market/plugins` | 联邦插件市场数据 |
| `GET` | `/api/plugin-downloads/:uuid` | JavaScript 或 ZIP |
| `GET` | `/api/binary-content/:token` | 临时二进制内容 |
| `GET` | `/api/web-chat/messages?rid=...` | 长轮询接收消息 |
| `POST` | `/api/web-chat/messages` | 创建聊天消息 |

Web Chat 发送示例：

```bash
curl -X POST http://localhost:8080/api/web-chat/messages \
  -H 'Content-Type: application/json' \
  -d '{"rid":"user123","ctt":"你好"}'
```

仅接收消息：

```bash
curl 'http://localhost:8080/api/web-chat/messages?rid=user123'
```

## 存储引擎

![存储管理页面](images/storage-management.png)

### 存储抽象

业务代码只依赖 `core/storage.Bucket` 接口，`MakeBucket(name)` 返回当前后端的命名 Bucket。插件、适配器和管理 API 使用相同的 `Get`、`Set`、`Delete`、`Keys`、`Buckets` 与监听语义。

| 引擎 | 默认 | 数据组织 | 适用场景 |
|---|---|---|---|
| BoltDB | 是 | `${SILLYGIRL_DATA_PATH}/sillyGirl.db`，Bucket 对应 Bolt bucket | 单实例、零依赖、轻量部署 |
| Redis | 否 | Redis Hash，Hash 名对应 Bucket | 外部持久化、多实例共享、集中备份 |

启动时先读取 BoltDB 中 `sillyGirl.storage`。值为 `redis` 时，程序使用 `redis_addr` 和 `redis_password` 连接 Redis；连接失败会回落到 BoltDB 并记录错误。

### 数据目录

优先使用环境变量 `SILLYGIRL_DATA_PATH`。未设置时的默认目录：

| 环境 | 默认目录 |
|---|---|
| Windows | `C:\ProgramData\sillyGirl\` |
| macOS | `程序目录/.sillyGirl/` |
| Linux | `/etc/sillyGirl/` |
| Docker | 推荐显式设置为 `/data` 并挂载持久卷 |

Docker 示例：

```bash
docker run -d --name sillygirl --restart unless-stopped \
  -p 8080:8080 \
  -e SILLYGIRL_DATA_PATH=/data \
  -v "$PWD/data:/data" \
  qninq/sillygirl:latest
```

### 值编码

Bucket 对外返回原始类型，底层字符串使用前缀保存类型：

| 前缀 | 类型 | 示例 |
|---|---|---|
| 无 | string | `hello` |
| `d:` | integer | `d:42` |
| `f:` | float | `f:3.140000` |
| `b:` | boolean | `b:true` |
| `o:` | JSON object/array | `o:{"enabled":true}` |

应用层应通过 Bucket API 写值，不要直接拼接这些前缀。

### 内置数据桶

系统自身数据与插件数据共用同一存储引擎，主要内置 Bucket 如下：

| Bucket | 键 | 值 | 用途 |
|---|---|---|---|
| `sillyGirl` | 设置名 | 标量 / JSON | 系统设置（存储后端、公告、管理员账号等） |
| `users` | `user:<账号>` | JSON | 普通用户账号：`id`、`username`、`nickname`、`email`、`password_hash`（bcrypt）、`created_at`、`updated_at`、`disabled` |
| `users` | `bindings:<账号>` | JSON | 用户绑定：`qq`、`telegram`、`updated_at`；自助注册用户注册时自动写入 `qq` |
| `tasks` | 任务 ID | JSON | 定时任务 |
| `plugins` | 插件 ID | JSON | 已安装插件 |
| `plugin_config_schemas` | 插件 UUID | JSON | 插件配置表单定义 |
| `plugin_config_values` | 插件 UUID | JSON | 插件配置值 |
| `plugin_messages` | 消息 ID | JSON | 插件消息记录 |
| `reply` | 规则 ID | JSON | 回复规则 |
| `CarryGroups` | 分组 ID | JSON | 转发 / 监听分组 |
| `noListenUsers`、`listenOnGroups`、`noReplyGroups` | 平台相关 ID | 标量 | 监听与回复开关 |
| `nickname` | 自动生成 | JSON | 用户昵称 |
| `proxies` | 代理 ID | JSON | 代理配置 |
| `auths` | 授权键 | 标量 | OpenAPI 授权 |
| 平台名（`qq`、`telegram` 等） | `masters` 等 | 标量 | 各平台管理员与适配器配置 |

用户账号数据全部收在 `users` 一个桶内，用 `user:` 与 `bindings:` 前缀区分账号与绑定两类键，删除用户会同时移除这两个键（写空值即删除）。自助注册（QQ 邮箱）用户的账号即邮箱前缀 QQ 号，`user:` 记录的 `email` 保存完整 QQ 邮箱，`bindings.qq` 自动写入同一 QQ 号。

### 管理 REST API

Bucket 名称可包含点号以表达层级（如 `im.wc`，管理页按点号分层显示成树），但不能包含逗号、斜杠或空白字符。

| Method | Resource | 说明 |
|---|---|---|
| `GET` | `/api/admin/storage/buckets` | 列出 Bucket |
| `POST` | `/api/admin/storage/buckets` | 创建 Bucket |
| `POST` | `/api/admin/storage/buckets/:bucket/deletions` | 删除 Bucket |
| `POST` | `/api/admin/storage/buckets/:bucket/renames` | Bucket 改名（请求体 `{"name":"新名称"}`，复制键值后删除旧 Bucket） |
| `GET` | `/api/admin/storage/bucket-entries?bucket=NAME&page=1&page_size=20&search=` | 按精确 Bucket 名分页读取键值（`search` 按 KEY/VALUE 子串过滤） |
| `POST` | `/api/admin/storage/bucket-entries` | 按精确 Bucket 名创建、更新或删除值（请求体 `{"bucket":"...","key":"...","value":"..."}`，`value` 为空字符串时删除该键，Bucket 不存在时自动创建） |
| `GET` | `/api/admin/storage/entries?bucket=NAME&page=1&page_size=20` | 分页读取键值（`bucket` 支持逗号分隔多个，`NAME.KEY` 形式读取单键） |
| `GET` | `/api/admin/storage/values?bucket=NAME&key=KEY` | 读取值（`keys=NAME.KEY` 逗号分隔批量，键值寻址用 `.` 分隔，不适用于含点号 Bucket 名） |
| `POST` | `/api/admin/storage/values` | 创建、更新或删除值（键写为 `bucket.key`，含点号 Bucket 名请改用 `bucket-entries`） |

```bash
curl 'http://HOST:8080/api/admin/storage/bucket-entries?bucket=im.wc&page=1&page_size=20' \
  -H 'token: JWT_TOKEN'

curl -X POST 'http://HOST:8080/api/admin/storage/bucket-entries' \
  -H 'token: JWT_TOKEN' \
  -H 'Content-Type: application/json' \
  -d '{"bucket":"im.wc","key":"disable_group","value":"false"}'
```

### 切换、迁移与备份

1. 在后台“基础设置”选择 `boltdb` 或 `redis`。
2. Redis 模式填写 `redis_addr` 和 `redis_password`，保存时会先执行连接测试。
3. 存储后端切换在重启后完整生效。
4. 启动前会自动迁移旧版面板、BOT、插件配置、用户绑定和授权字段。

```bash
# 只检查迁移，不写入
go run ./cmd/storage-migrate -dry-run

# 执行迁移
go run ./cmd/storage-migrate
```

后台备份接口 `GET /api/admin/system-backups/current` 会生成当前数据与插件文件的 ZIP。直接复制 BoltDB 文件前应停止写入；Redis 备份使用 Redis 自身的 RDB/AOF 策略。

## gRPC API

### 服务定义

**Proto 文件**：`proto3/srpc.proto`

**Go 包**：`github.com/qninq/sillyGirlPro/proto3/srpc`

**服务名**：`SillyGirlService`

### 消息类型

#### Empty
```protobuf
message Empty {}
```

#### Default
```protobuf
message Default {
  string value = 1;
}
```

#### BucketSetRequest
```protobuf
message BucketSetRequest {
  string name = 1;
  string key = 2;
  string value = 3;
}
```

#### BucketSetResponse
```protobuf
message BucketSetResponse {
  bool changed = 1;
  string message = 2;
}
```

#### BucketKeyRequest
```protobuf
message BucketKeyRequest {
  string name = 1;
  string key = 2;
}
```

#### BucketRequest
```protobuf
message BucketRequest {
  string name = 1;
}
```

#### BucketKeysResponse
```protobuf
message BucketKeysResponse {
  repeated string keys = 1;
}
```

#### LenResponse
```protobuf
message LenResponse {
  int32 length = 1;
}
```

#### BoolResponse
```protobuf
message BoolResponse {
  bool value = 1;
}
```

#### BucketsResponse
```protobuf
message BucketsResponse {
  repeated string buckets = 1;
}
```

#### SenderRequest
```protobuf
message SenderRequest {
  string uuid = 1;
}
```

#### ReplyRequest
```protobuf
message ReplyRequest {
  string uuid = 1;
  string content = 2;
}
```

#### SenderContentRequest
```protobuf
message SenderContentRequest {
  string uuid = 1;
  string content = 2;
}
```

#### SenderListenRequest
```protobuf
message SenderListenRequest {
  string uuid = 1;
  repeated string rules = 2;
  int32 timeout = 3;
  bool listen_group = 4;
  bool listen_private = 5;
  bool require_admin = 6;
  repeated string allow_platforms = 7;
  repeated string prohibit_platforms = 8;
  repeated string allow_users = 9;
  repeated string prohibit_users = 10;
  repeated string allow_groups = 11;
  repeated string prohibit_groups = 12;
  bool persistent = 13;
  string value = 14;
  string plugin_id = 15;
}
```

#### SenderListenResponse
```protobuf
message SenderListenResponse {
  string echo = 1;
  string uuid = 2;
}
```

#### AdapterRegistRequest
```protobuf
message AdapterRegistRequest {
  string platform = 1;
  string bot_id = 2;
}
```

#### AdapterRequest
```protobuf
message AdapterRequest {
  string platform = 1;
  string bot_id = 2;
  string value = 3;
}
```

#### ConsoleRequest
```protobuf
message ConsoleRequest {
  string type = 1;
  string content = 2;
  string plugin_id = 3;
}
```

### RPC 方法列表

| 方法 | 请求 | 响应 | 说明 |
|------|------|------|------|
| `BucketGet` | `BucketKeyRequest` | `Default` | 获取 Bucket 键值 |
| `BucketSet` | `BucketSetRequest` | `BucketSetResponse` | 设置 Bucket 键值 |
| `BucketDelete` | `BucketRequest` | `Empty` | 删除 Bucket |
| `BucketKeys` | `BucketRequest` | `BucketKeysResponse` | 获取所有键名 |
| `BucketLen` | `BucketRequest` | `LenResponse` | 获取键数量 |
| `BucketGetAll` | `BucketRequest` | `Default` | 获取所有键值（JSON） |
| `BucketBuckets` | `Empty` | `BucketsResponse` | 获取所有 Bucket 名 |
| `BucketWatch` | `stream BucketWatchRequest` | `stream BucketWatchResponse` | 流式监听变更 |
| `SenderGetUserId` | `SenderRequest` | `Default` | 获取用户ID |
| `SenderGetUserName` | `SenderRequest` | `Default` | 获取用户名 |
| `SenderGetChatId` | `SenderRequest` | `Default` | 获取群聊ID |
| `SenderGetChatName` | `SenderRequest` | `Default` | 获取群聊名 |
| `SenderGetMessageId` | `SenderRequest` | `Default` | 获取消息ID |
| `SenderIsAdmin` | `SenderRequest` | `BoolResponse` | 是否管理员 |
| `SenderGetPlatform` | `SenderRequest` | `Default` | 获取平台 |
| `SenderGetBotId` | `SenderRequest` | `Default` | 获取机器人ID |
| `SenderGetContent` | `SenderRequest` | `Default` | 获取消息内容 |
| `SenderSetContent` | `SenderContentRequest` | `Empty` | 设置消息内容 |
| `SenderContinue` | `SenderRequest` | `Empty` | 继续匹配 |
| `SenderListen` | `stream SenderListenRequest` | `stream SenderListenResponse` | 流式消息监听 |
| `SenderEvent` | `SenderRequest` | `Default` | 获取事件数据 |
| `SenderReply` | `ReplyRequest` | `Default` | 发送回复 |
| `SenderParam` | `ReplyRequest` | `Default` | 获取参数 |
| `SenderAction` | `ReplyRequest` | `Default` | 执行动作 |
| `SenderDestroy` | `ReplyRequest` | `Empty` | 销毁 Sender |
| `AdapterRegist` | `stream AdapterRegistRequest` | `stream Default` | 注册适配器 |
| `AdapterReceive` | `AdapterRequest` | `Empty` | 接收消息 |
| `AdapterPush` | `AdapterRequest` | `Default` | 推送消息 |
| `AdapterDestroy` | `AdapterRequest` | `Empty` | 销毁适配器 |
| `AdapterSender` | `AdapterRequest` | `Default` | 获取 Sender |
| `Console` | `ConsoleRequest` | `Empty` | 控制台日志 |

### Python 插件示例

Python 插件运行在 Python 3.12 中，通过 `sillygirl` SDK 调用 Go 主程序能力：

```python
"""
* @title Python 示例
* @rule raw ^py$
"""

import asyncio
from sillygirl import sender as s, Bucket


async def main():
    db = Bucket("demo")
    count = await db.get("count", 0)
    await db.set("count", count + 1)
    await s.reply(f"count={count + 1}")


asyncio.run(main())
```

### Python gRPC 客户端示例

```python
import grpc
from proto3 import srpc_pb2, srpc_pb2_grpc

channel = grpc.insecure_channel('localhost:50051')
stub = srpc_pb2_grpc.SillyGirlServiceStub(channel)

# 获取 Bucket 值
req = srpc_pb2.BucketKeyRequest(name="sillyGirl", key="port")
resp = stub.BucketGet(req)
print(f"Port: {resp.value}")

# 设置 Bucket 值
req = srpc_pb2.BucketSetRequest(name="app", key="test", value="hello")
resp = stub.BucketSet(req)
print(f"Changed: {resp.changed}")
```

## 接口验证

仓库中的 REST 契约测试会检查：

- 业务接口只注册 `GET` 与 `POST`；
- 路由使用资源名、路径参数和 deletion/job 子资源；
- 已删除旧接口保持 `404`；
- `401/404/409/422/500` 使用对应 HTTP 状态；
- 所有 REST JSON 响应顶层严格包含 `status`、`message`、`data`，且 `status` 只能是布尔值；
- `GET /api/admin/panels` 一次返回 `qinglong`、`daidai` 两组集合。

```bash
go test ./core -run 'Test.*(REST|Route|HTTP|AdminResource)'
go test ./...
```
