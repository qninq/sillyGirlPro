# 适配器指南

[返回项目导航](../README.md) · [界面截图](screenshots.md) · [API 与存储](api-storage.md) · [插件编写](plugin-development.md)

适配器把平台事件转换为 SillyGirl 的统一消息模型，再把插件回复发送回原平台。当前内置 7 个适配器。

![适配器管理页面](images/adapter-management.png)

## 支持矩阵

| 平台标识 | 接入方式 | 收消息 | 发消息 | 主要配置 |
|---|---|---|---|---|
| `clawbot` | 微信 iLink HTTP 长轮询 | `getupdates` | `sendmessage` | `token`、`api_base` |
| `qq` | OneBot 反向 WebSocket | `/qq/receive` | WebSocket action | `token` |
| `telegram` | Telegram Bot API 长轮询 | `getUpdates` | `sendMessage` | `token`、`api_base` |
| `dingtalk` | 钉钉 Stream | Stream callback | `sessionWebhook` | `client_id`、`client_secret` |
| `qqguild` | BotGo Webhook/WebSocket | `/qqguild/webhook` 或 Gateway | OpenAPI | `app_id`、`app_secret`、`mode` |
| `web` | 浏览器长轮询 | `/api/web-chat/messages` | 内置消息队列 | `web_chat_public` |
| `pagermaid` | WebSocket 桥接 | `/pagermaid/receive` | WebSocket action | `token` |

## 通用配置

推荐在管理后台 `/admin/bots` 完成配置。底层配置分别保存在同名 Bucket 中，常用键如下：

| 键 | 含义 |
|---|---|
| `enable` | 设为 `false` 时停用适配器；未配置时按各适配器默认值处理 |
| `debug` | 输出该适配器的收发调试日志 |
| `token` / `client_secret` / `app_secret` | 平台认证凭据，后台以密码输入框维护 |
| `api_base` | 可选兼容 API 或反向代理基址 |

配置变化会触发适配器刷新或重启。状态页出现稳定的 Bot ID，表示 `core.Factory` 已注册成功。

## 微信 ClawBot

ClawBot 使用腾讯 OpenClaw 微信通道同款 iLink API。后台点击“扫码获取”后生成二维码；确认成功会保存 `clawbot.token` 并启动长轮询。

| Bucket | Key | 说明 |
|---|---|---|
| `clawbot` | `token` | iLink bot token |
| `clawbot` | `enable` | 可选开关 |
| `clawbot` | `api_base` | 默认 `https://ilinkai.weixin.qq.com` |
| `clawbot` | `cdn_base_url` | 可选媒体 CDN 基址 |
| `clawbot` | `debug` | 调试日志 |

回复依赖上游消息的 `context_token`。图片消息会在有效期内下载并转存为本地临时资源，插件应及时处理收到的媒体地址。

主动推送（转发、`pushAdmin` 等）会复用该联系人**最近一次收到消息**的 `context_token`（30 分钟内有效）；机器人从未收到过该联系人消息时无法主动推送，会记录警告日志。

## QQ / OneBot

在 NapCat、Lagrange.OneBot 等兼容端配置反向 WebSocket：

```text
ws://HOST:8080/qq/receive
wss://HOST/qq/receive
```

```json
{
  "enable": true,
  "url": "ws://HOST:8080/qq/receive",
  "accessToken": "TOKEN"
}
```

SillyGirl 的 `qq.token` 必须与 OneBot 客户端的 `accessToken` 一致。连接建立后，Bot ID 从 `X-Self-ID` 或 OneBot 事件中确定。

## Telegram Bot

1. 从 BotFather 获取 Bot Token。
2. 在 `/admin/bots` 填写 Token。
3. 需要代理时填写兼容 `api_base`。
4. 保存后检查日志与 BOT 状态。

| Bucket | Key | 说明 |
|---|---|---|
| `telegram` | `token` | Bot Token |
| `telegram` | `enable` | 可选开关 |
| `telegram` | `api_base` | 默认 `https://api.telegram.org` |
| `telegram` | `debug` | 调试日志 |

适配器启动时清理旧 webhook，然后使用长轮询接收更新。

## 钉钉机器人

钉钉适配器使用 Stream 模式，不需要公网回调地址。文本回复复用事件中的 `sessionWebhook`。

| Bucket | Key | 说明 |
|---|---|---|
| `dingtalk` | `client_id` | Client ID / AppKey |
| `dingtalk` | `client_secret` | Client Secret / AppSecret |
| `dingtalk` | `enable` | 可选开关 |
| `dingtalk` | `debug` | 调试日志 |

在钉钉开放平台创建机器人并启用 Stream 模式，保存配置后检查 `Stream 已连接` 日志。

## QQ 官方频道机器人

平台标识为 `qqguild`，与 `qq` OneBot 独立。支持 Webhook 和 WebSocket 两种模式。

```text
https://HOST/qqguild/webhook
```

| Bucket | Key | 说明 |
|---|---|---|
| `qqguild` | `app_id` | 机器人 AppID |
| `qqguild` | `app_secret` | 机器人 AppSecret |
| `qqguild` | `mode` | `webhook` 或 `websocket` |
| `qqguild` | `sandbox` | 是否使用沙箱 OpenAPI |
| `qqguild` | `public_bot` | 公域机器人开关，开启后注册 `MESSAGE_CREATE` intent 接收频道全量消息（仅公域机器人可用，默认关闭） |
| `qqguild` | `markdown` | 群聊/C2C 回复改用 Markdown 格式发送，失败自动回退纯文本（默认关闭） |
| `qqguild` | `at` | 群聊回复自动 @ 发送者，仅在 `markdown` 开启时生效（默认开启） |
| `qqguild` | `enable` | 可选开关 |
| `qqguild` | `debug` | 调试日志 |

Webhook 模式要求可访问的 HTTPS 地址；WebSocket 模式由 SillyGirl 主动连接 Gateway，断线重连采用指数退避（2s 起步、30s 封顶，连接成功后重置）。

支持扫码快捷绑定：BOT 设置里点击「扫码添加机器人」，使用手机 QQ 扫描二维码并在手机上确认后，自动保存 AppID/AppSecret 并启动机器人（机器人需已在 q.qq.com 注册；密钥通过 QQ 官方绑定服务 AES-256-GCM 加密传输，服务端本地解密后落库）。

消息能力：

- **接收**：支持频道 @ 消息（`AT_MESSAGE_CREATE`）、频道全量消息（公域开关开启后）、群聊全量消息（`GROUP_MESSAGE_CREATE`）、群 @ 消息、C2C 私聊和频道私信；按消息 ID 做入站去重（10 秒窗口），网关重连重推不会导致重复回复。
- **发送**：默认纯文本。回复内容中的 `[CQ:image,url=...]`、`[CQ:video,url=...]`、`[CQ:record,url=...]`、`[CQ:file,url=...]` 会被解析，群聊和 C2C 通过 QQ v2 富媒体接口上传后以 `msg_type=7` 发送，文字随首条媒体一起发送；频道场景支持单张图片 URL 直发。本地路径的媒体链接无法上传，会被忽略并记录日志。
- **Markdown**：开启 `markdown` 开关后，群聊和 C2C 的纯文本回复改用 `msg_type=2` Markdown 发送；机器人无 Markdown 权限时自动回退纯文本。频道消息的 `content` 本身支持 Markdown，无需开关。
- **回复自动@**：开启 `at` 开关后（默认开），群聊 Markdown 回复会自动在正文前插入 `<qqbot-at-user>` 标签 @ 发送者；仅在 `markdown` 开启时生效。
- **主动推送**：只带 ID 的主动推送（转发、`pushAdmin` 等）按以下顺序路由：核心显式携带的 `chat_type`（群聊→群接口 `/v2/groups/{id}/messages`，私聊→C2C 接口）→ 从收到的群聊/C2C 消息学习的「openid → 场景」映射 → 未知目标按频道接口处理。群主动消息受平台月度配额限制；C2C 主动推送需要对方先私聊过机器人。
- **撤回**：接收过的消息支持通过核心 Action（`type: delete_message`）按场景调用对应撤回接口，30 分钟内的消息有效。

## Web Bot

Web Bot 随主程序注册为 `web/default`，后台右下角可直接打开聊天窗口。

主动推送（转发、`pushAdmin` 等）目标为 **Web 会话 rid**（浏览器登录会话 ID，私聊类型按 `user_id` 投递，仅带 `chat_id` 时回退到 `chat_id`）；会话需保持在线，长时间无访问的会话会被清理，队列中的消息随之丢弃。

| 配置 | 说明 |
|---|---|
| `sillyGirl.web_chat_public=false` | 仅已登录管理员可发送消息，推荐默认值 |
| `sillyGirl.web_chat_public=true` | 允许匿名调用聊天接口 |

```bash
curl 'http://HOST:8080/api/web-chat/messages?rid=SESSION'
curl -X POST 'http://HOST:8080/api/web-chat/messages' \
  -H 'Content-Type: application/json' \
  -d '{"rid":"SESSION","ctt":"你好"}'
```

每个 `rid` 对应独立的有界消息队列，长时间不活跃会自动过期。

## Pagermaid

仓库中的桥接插件位于 [`adapters/pagermaid/sillyplus.py`](../adapters/pagermaid/sillyplus.py)。

```text
ws://HOST:8080/pagermaid/receive?token=TOKEN
wss://HOST/pagermaid/receive?token=TOKEN
```

| Bucket | Key | 说明 |
|---|---|---|
| `pagermaid` | `token` | WebSocket 连接密钥 |
| `pagermaid` | `enable` | 可选开关 |
| `pagermaid` | `debug` | 调试日志 |

桥接脚本只负责转发消息和执行发消息动作；群监听、屏蔽用户、管理员判断和插件规则均由核心处理。

## 统一消息模型

适配器调用 `Factory.Receive` 时至少应提供：

```go
params := map[string]interface{}{
    core.USER_ID:    "ACCOUNT",
    core.CHAT_ID:    "GROUP_OR_CHANNEL",
    core.CONETNT:    "消息正文",
    core.MESSAGE_ID: "MESSAGE_ID",
    "user_name":    "昵称",
    "chat_name":    "会话名称",
}
adapter.Receive(params)
```

平台 ID 统一转成字符串；私聊的 `chat_id` 可为空。发送端通过 `SetReplyHandler` 读取同一组标准字段并返回平台消息 ID。

## 排查顺序

1. 在 BOT 页面确认适配器已启用。
2. 检查必需凭据和接入 URL。
3. 确认日志中不存在认证、超时或重复连接错误。
4. 检查 BOT 页面是否出现稳定 Bot ID。
5. 用私聊和群聊各触发一次最小插件规则。
6. 确认 `user_id`、`chat_id`、`message_id` 和回复目标正确。
7. 临时打开 `debug` 定位问题，完成后关闭，避免日志包含过量上下文。
