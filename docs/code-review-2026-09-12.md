# 代码审查报告 · 假死 / 资源占用 / 泄漏与整体健康度

- **审查日期**：2026-09-12
- **审查版本**：工作区 `VERSION = 1.2.7`（含未提交改动）
- **审查性质**：只读静态审查 + 构建/测试实测；**未修改任何生产代码**
- **报告状态**：待办。第 6 条与文档项已完成，其余条目保留待后续修复

---

## 一、审查范围与边界

| 覆盖程度 | 范围 |
|---|---|
| **逐行细读** | `core/` 运行主干：adapter、base_sender、function、grpc_sender、grpc_plugins、grpc_runtime、plugin_core、plugin_message、plugin_impl、queue、task、bucket、web、init、nickname、node_temp、storage/*、logs/file、python_runtime、node_runtime_preload |
| **模式级检索** | `adapters/**`、`frontend/src/**`、`proto3/**`、`skills/**` |
| **未逐行审读** | `core/node_dependency.go`（2571 行）、`core/plugin_subscribe.go`（1468 行）、`core/system_update.go`、`adapters/*` 其余部分、大部分 Vue 组件 |

未执行：`go test -race`（测试当时无法编译）、`govulncheck`（故本报告不列任何 CVE）。

---

## 二、实测验证证据

| 命令 | 结果 |
|---|---|
| `go build ./...` | 通过 |
| `go vet ./...` | 通过（修复第 6 条前为失败） |
| `go test ./...` | 全部 ok，无 FAIL（修复前 core 为 `[build failed]`） |
| `npm run check:components` | 通过：`lazy_views: 14`、`domain_composables: 10`、`controller_lines: 3059` |
| `npm run check:responsive` | 通过，`failed: []` |
| `python skills/sillygirl-plugin-writer/scripts/validate_plugin.py <templates>` | 2/2 PASS |
| `run.log` | 末行 `fatal: morestack on gsignal`（v1.2.6 启动后即崩，根因未定位，见第八节） |

---

## 三、已完成项

- **第 6 条 · core 测试集无法编译**：`createNormalUser` 新增 `email` 参数后，`core/auth_header_integration_test.go:171`、`:249` 与 `core/user_admin_test.go:13` 仍按 3 参调用，导致 `go vet` / `go test` 在 core 包整体 `build failed`，约 40 个用例长期无法执行。已补齐参数。
- **配置表单兜底语义的测试契约**：`TestFinishPythonDependencyInstallRetriesPluginConfigSchema` 原断言与 v1.2.7 的占位导入兜底行为相悖，已改为验证兜底语义。
- **文档一致性**：CHANGELOG、`docs/adapters.md`、`docs/screenshots.md`、`frontend/ARCHITECTURE.md`、`skills/sillygirl-plugin-writer/*`、`core/python_runtime.go` 注释均已按代码实测校正，详见 CHANGELOG「未发布 → 测试与验证 / 文档」。

---

## 四、高严重度：可导致崩溃或整体假死

### H1 · `s.listen()` 监听器永久滞留，且第 5 层 listen 触发 nil panic 使进程退出

- **证据**：`core/base_sender.go:315` 默认 `timeout := time.Hour * 999999`（≈114 年）；`core/base_sender.go:348-350` 用 `waits[4-s.level]` 注册/注销；`core/grpc_sender.go:198-200` 每次监听把 level 加 1；`waits` 仅含 1..5 五个键（`core/base_sender.go:253-269`）。
- **触发**：未显式传 timeout 的 `s.listen()`；或对同一 Sender 连续/嵌套 listen 使 level 累计到 4。
- **影响**：① Carry + goroutine 永久滞留，内存与 goroutine 无界增长（`utils/init.go:296-311` 的 `MonitorGoroutine` 在 >800 时重启进程，正属掩盖该泄漏的治标手段）；② level=4 时 `waits[0]` 为 nil → 方法调用 nil 指针 panic，且发生在 `go s.Await(...)` 独立 goroutine 中，`HandleMessage` 的 recover 拦不住 → **整个进程退出**。
- **建议**：给 `waits` 的索引做边界校验或改为 map 兜底；为 listen 设合理默认上限并保证退出路径一定反注册；补一个「未匹配的 listen 必须被回收」的失败探针测试。

### H2 · `CancelPluginlistening` 持读锁做阻塞发送 → 全局死锁

- **证据**：`core/plugin_core.go:52-62` 在 `Foreach` 内执行 `c.Chan <- errors.New("uinstall")`；`Foreach` 持有 `cs.RLock()`（`core/base_sender.go:245-251`）；`c.Chan` 容量仅 1（`core/base_sender.go:345`）。
- **触发**：卸载/重载插件时，目标 Carry 的 channel 已有排队消息。
- **影响**：发送阻塞 → RLock 永久持有；此后 `waits[n].Add/Remove`（写锁）与 `HandleMessage` 的 `Foreach`（读锁）全部阻塞 → 消息分发彻底停摆（假死）。
- **建议**：改为非阻塞发送（`select` + `default`）或先摘除再在锁外通知；不要在持锁期间做任何可能阻塞的操作。

### H3 · 插件 stderr 无界累积 + 单行 64KB 上限导致插件挂死

- **证据**：`core/grpc_plugins.go:319-329` 非 `on_start` 插件把 stderr 全部 append 到 `lines`，仅在进程退出时输出；`core/grpc_plugins.go:301`、`:314` 的 `bufio.NewScanner` 未调 `Buffer`，默认单行上限 64KB（对照 `core/app_scripts.go:621-622`、`:673-674` 已显式提到 1MB）。
- **触发**：长期运行的插件持续写 stderr；或插件输出单行超过 64KB（大 JSON、base64、超长堆栈）。
- **影响**：① 核心内存无界增长；② `Scan()` 返回 false 后读端停止，子进程写满管道即阻塞在写操作上，`cmd.Wait()` 永不返回 → 插件假死 + 处理协程泄漏。
- **建议**：stderr 改为有界环形缓冲或即时上报；与 `app_scripts.go` 对齐设置 `scanner.Buffer`；检查 `scanner.Err()`。

### H4 · 每次 `listen` 在 `senderRegisters` 留下永久条目

- **证据**：`core/grpc_sender.go:206` 调用 `register(s)`；实现见 `core/grpc_runtime.go:84-92`；仅在插件进程退出时由 `deleteSenderRegister` 整体清理（`core/grpc_plugins.go:344`、`:381`）。
- **影响**：长驻插件的每次 listen 都泄漏一个 Sender（持有 stream 与回调引用）。
- **建议**：listen 结束时删除对应 uuid 条目。

### H5 · `WritePluginMessage` 每次日志全量读改写 + 超线性比较

- **证据**：`core/plugin_message.go:28-55`（读整桶 → 逐条 `Similarity` → 整体回写并 fsync）；`Console.Warn/Error` 每次触发（`core/plugin_impl.go:85`、`:95`）；`Similarity` 内部含嵌套 `strings.Contains` 扫描（`core/node_strings.go:41-76`）。
- **影响**：插件错误刷屏时 CPU、内存、磁盘 IO 同时放大；`pmsgs` 数组仅按 >0.9 相似度去重，无界增长。
- **建议**：改增量写入 + 定长淘汰；相似度比较改为先按 class/前缀分桶再比较；对写入做限流与合并。

---

## 五、中严重度：资源与并发

| 编号 | 问题 | 证据 | 建议 |
|---|---|---|---|
| M1 | `storage.Listens` 无锁并发读写（append vs 遍历 vs 改写元素），插件热加载时 race | `core/storage/main.go:52-66`、`:14-22`、`core/storage/boltdb/init.go:167-171`、`core/storage/redis/init.go:140` | 加 RWMutex 或改 copy-on-write 快照 |
| M2 | 端口切换竞态与不可恢复的绑定失败；`Daemon()` 无单实例锁，`bolt.Open(...,nil)` Timeout=0 使第二实例静默永久阻塞 | `core/web.go:314-369`、`utils/init.go:245-251`、`core/storage/boltdb/init.go:72` | 端口变更串行化 + 加锁；绑定失败重试并告警；加 pidfile/文件锁保证单实例 |
| M3 | `node_temp.go` 每次 Set 起 goroutine 全量序列化并覆写整个文件（非原子、无 temp+rename）；`Get` 对过期键返回 nil 但不删除 | `core/node_temp.go:43-53`、`:57-65` | 改惰性/批量落盘 + `原子替换`；Get 命中过期即删除；加定期清理 |
| M4 | 多处 `http.DefaultClient` 无超时 | `core/qinglong.go:219`、`core/daidai.go:231`、`core/clawbot_login.go:27`、`adapters/qqguild/onboard.go:90`、`adapters/flowbot/main.go:507`、`core/logs/alils/request.go:56`、`adapters/dingtalk/main.go:311` | 统一带 Timeout 的 client |
| M5 | Redis 订阅重连前未关闭旧 subscriber | `core/storage/redis/init.go:126-150` | 重连前 `subscriber.Close()`，并加退避上限 |
| M6 | `f.gmsgChan` 只增不减 | `core/adapter.go:439-443`、`:663-667`（全仓库无 `Delete`） | 空闲即回收，或加 TTL/LRU |
| M7 | `Destroy()` 关 `msgChan` 与 `Reply()` 发送竞态 → `send on closed channel` panic | `core/adapter.go:340` vs `:682-685`（判断与发送之间无锁） | 用 `f.Lock()` 保护，或改 `sync.Once` + 取消信号代替 close |
| M8 | 每条消息在唯一分发协程内做一次 bolt 写（fsync），且 `Messages` 为无缓冲通道 | `core/function.go:202-209`、`:305-313`、`:188`、`:324`、`core/storage/boltdb/init.go:342-369` | 昵称写入异步化/合并；`Messages` 加缓冲；评估关闭 bolt 同步写 |

---

## 六、低严重度与补充

| 编号 | 问题 | 证据 |
|---|---|---|
| L1 | 前端 `pollExit` 每 500ms 轮询，`stopScriptDebug()` 不清除；SSE `onerror` 路径会让 interval 一直跑 | `frontend/src/composables/admin/useScriptsAdmin.ts:790-796`、`:737-743`、`:783-789` |
| L2 | 第三方远程脚本注入后台/用户页，含 `http://` 端点（仅 HTTPS 页时跳过）→ 明文 HTTP 下面板可被中间人替换 | `frontend/src/watchdog.ts:3-7` |
| L3 | 全局表只增不减：`mutexMap`、`queues`；`Queue.Size/IsEmpty` 无锁（依赖调用方持锁） | `core/plugin_core.go:20-34`、`core/queue.go:8`、`:58-68` |
| L4 | 首次 `MakeBucket` 静默删除所有「无键」桶，无日志无确认 | `core/bucket.go:105-111` |
| L5 | 打开数据库失败 `os.Exit(0)`，退出码 0 使进程管理器不重启 | `core/storage/boltdb/init.go:64-76` |
| L6 | 依赖滞后/上游归档：`Dreamacro/clash v1.17.0`、`boltdb/bolt v1.3.1`（上游已归档）、`go-elasticsearch/v6`（EOL 大版本）、`go-redis/v8`、`fsnotify v1.4.9`、`botgo v0.2.1`；`replace` 将 gorm 降级钉死 | `go.mod` |

---

## 七、仓库卫生（已查实）

- **`run.log` 未被忽略**：`.gitignore` 含 `.runlogs/`、`.tmp-*.log`、`*.db`，但无 `run.log`；该文件含 `fatal: morestack on gsignal` 崩溃日志，存在误提交风险。
- **40 个前端构建产物纳入版本控制**（`core/admin/assets/*.js`），每次构建都会 churn（工作区表现为 16 删 + 16 增）。
- 报告生成时工作区共有 **55 项未提交改动**，横跨 adapters / core / docs / frontend / assets。

---

## 八、未定位 / 未执行

- **`morestack on gsignal` 根因未定位**：已核查 `storage.Set` 写回路径（`core/storage/boltdb/init.go:172-196`）存在 `old == new` 去重保护，未发现 `Set→Watch→Set` 递归。属存疑项，建议用带符号构建 + `GOTRACEBACK=crash` 在 v1.2.7 复现。
- **未跑**：`go test -race ./core/...`（测试现已可编译，可直接执行，预期能暴露 M1/M2/M7 类竞态）、`govulncheck`。
- **待定**：`docs/images/*.png` 仍为 v1.0.8 时期截图（文内已如实标注差异），是否重拍待定；`join_strategy_whitelist` 的「≤10000 个」无法从代码证实。

---

## 九、建议修复顺序

1. **H1 / H2 / H3** —— 唯一会导致进程崩溃或整体假死的三条，且都集中在 listen 链路，可用测试覆盖。
2. **`go test -race ./core/...`** —— 一次性暴露 M1 / M2 / M7 类并发问题，成本低收益高。
3. **H4 / H5 + M3** —— 泄漏与写放大，属长期运行稳定性问题。
4. **M4 / M5 / M6 / M8** —— 外部依赖与吞吐相关的加固。
5. **L 系列与仓库卫生** —— 随手可清的项，建议与上面同批提交以减少 churn。

每条修复都应附带非回归验证：`go vet ./... && go test ./... && go test -race ./core/...`。
