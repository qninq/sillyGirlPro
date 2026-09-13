# SillyGirl

[![Version](https://img.shields.io/badge/version-v1.2.9-1677ff)](VERSION)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](go.mod)

SillyGirl（傻妞）是一个多平台聊天机器人框架：一套核心接入微信（ClawBot）、QQ 官方机器人、QQ 频道、Telegram、钉钉、Web 等平台，支持 JavaScript / Python / gRPC 插件、跨平台消息转发、回复规则与定时任务，自带 Web 管理后台与用户服务入口（QQ 邮箱注册、QQ 号即账号）。

## 快速开始

### 源码构建（Go 1.26+）

```bash
git clone https://github.com/qninq/sillyGirlPro.git
cd sillyGirlPro
go build -o sillyGirl .
./sillyGirl
```

### Linux 一键安装

```bash
bash <(curl -sL https://raw.githubusercontent.com/qninq/sillyGirlPro/main/install.sh)
```

国内网络可设置代理：`GITHUB_PROXY=https://gh-proxy.org bash <(curl -sL ...)`。安装到 `/usr/local/sillyGirl`，运行 `./sillyGirl -t` 进入交互模式。

### Docker

```bash
docker run -d --name sillygirl --restart unless-stopped \
  -p 8080:8080 \
  -e SILLYGIRL_DATA_PATH=/data \
  -v "$PWD/data:/data" \
  qninq/sillygirl:latest
```

### 管理后台

启动后访问 `http://localhost:8080/admin`，首次访问需设置管理员账号密码。内置适配器默认关闭，在「BOT 对接管理」页逐一开启后才接入平台；其余插件安装、规则配置与用户服务入口（`/`）均在后台完成配置。

## 文档导航

> SillyGirl 文档导航。功能说明、配置方法和示例统一维护在 `docs/` 的四个板块中。

| 板块 | 内容 | 入口 |
|---|---|---|
| 界面截图 | 管理后台概览、插件市场、存储管理和适配器页面 | [查看界面截图](docs/screenshots.md) |
| API 与存储 | GET/POST REST API、认证、响应格式、gRPC、BoltDB 与 Redis | [查看 API 与存储文档](docs/api-storage.md) |
| 插件编写 | JavaScript/Python 插件、元数据、规则、SDK、配置表单、调试与插件开发页面 | [查看插件开发指南](docs/plugin-development.md) |
| 适配器 | 微信 ClawBot、QQ、Telegram、钉钉、QQ 频道、Web 与 Pagermaid | [查看适配器指南](docs/adapters.md) |

## 项目导航

- [版本记录](CHANGELOG.md)
- [Releases](https://github.com/qninq/sillyGirlPro/releases)
- [源代码](https://github.com/qninq/sillyGirlPro)

---

本项目 fork 自 [smallfawn/sillyGirl](https://github.com/smallfawn/sillyGirl)，感谢原作者的开源贡献。
