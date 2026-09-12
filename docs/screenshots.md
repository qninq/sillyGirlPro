# 界面截图

[返回项目导航](../README.md) · [API 与存储](api-storage.md) · [插件编写](plugin-development.md) · [适配器](adapters.md)

以下截图由 **Chrome DevTools MCP** 在 SillyGirl `v1.0.8` 的隔离本地 fixture 上生成，页面数据仅用于文档展示。

> 截图早于当前版本（`v1.2.8`）：侧边栏菜单、概览页容器卡片与存储管理页界面此后均已改版，各段说明中标注了与当前版本的差异；界面细节以实际运行版本为准。

## 页面总览

| 页面 | 路由 | 用途 | 截图 |
|---|---|---|---|
| 管理概览 | `/admin` | 查看版本、脚本、用户与容器摘要 | [有](#管理概览) |
| 插件市场 | `/admin/plugins` | 搜索、安装、编辑和配置插件 | [有](#插件市场) |
| 插件开发 | `/admin/scripts` | 按语言分类管理本地应用脚本，在线编辑、配参与调试 | 暂无 |
| 存储管理 | `/admin/storage` | 查看 Bucket、Key 与 Value | [有](#存储管理) |
| BOT 管理 | `/admin/bots` | 配置适配器并查看连接状态 | [有](#适配器管理) |

## 管理概览

![SillyGirl 管理概览](images/admin-overview.png)

概览页集中显示当前版本、远端版本、脚本数量与用户数量。图中该版本还带有 `smallcat` 容器卡片；SmallCat 面板已在 `v1.2.3` 移除，当前版本只展示青龙与呆呆两类容器。

## 插件市场

![SillyGirl 插件市场](images/plugin-market.png)

插件卡片提供源码编辑、详情、安装、卸载和配置入口；筛选、搜索与编辑器按需加载。

## 存储管理

![SillyGirl 存储管理](images/storage-management.png)

图中为旧版存储页：顶部桶选择器加扁平键值表格（序号 / Bucket / Key / Value），支持查询、刷新、新建桶、删除桶与分页。当前版本已改为「数据桶目录 + 键值表格」布局：左侧桶名按点号分层折叠分组，右侧展示所选桶的键值表格并支持按 KEY / VALUE 搜索，VALUE 编辑器对 JSON 提供语法高亮、自动展开排版与「格式化 JSON」按钮，保存时压缩回单行保持存储紧凑。截图使用 `demo` Bucket，不含生产配置。

## 适配器管理

![SillyGirl 适配器管理](images/adapter-management.png)

BOT 页面统一展示微信 ClawBot、钉钉、Pagermaid、QQ、QQ 官方频道、Web Bot 与 Telegram 的启用和连接状态。
