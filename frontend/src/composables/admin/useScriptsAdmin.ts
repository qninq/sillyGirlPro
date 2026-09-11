import { computed, nextTick, reactive, ref } from "vue";
import message from "ant-design-vue/es/message";
import type { Compartment, Extension } from "@codemirror/state";
import type { EditorView } from "@codemirror/view";
import { get, getAuthToken, post } from "../../api";
import type { AppScriptInfo } from "../../types";
import { apiData, type ApiEnvelope } from "./adminApi";

export type AppScriptLanguage =
  | "es5"
  | "node"
  | "typescript"
  | "python"
  | "golang"
  | "adapter-js"
  | "adapter-python"
  | "adapter-go";

export const appScriptLanguageLabels: Record<AppScriptLanguage, string> = {
  es5: "ES5",
  node: "Node.js",
  typescript: "TypeScript",
  python: "Python",
  golang: "Golang",
  "adapter-js": "Adapter JS",
  "adapter-python": "Adapter Python",
  "adapter-go": "Adapter Go",
};

export const appScriptLanguageOrder: AppScriptLanguage[] = [
  "es5",
  "node",
  "python",
  "typescript",
  "golang",
  "adapter-js",
  "adapter-python",
  "adapter-go",
];

export const appScriptStarters: Record<AppScriptLanguage, string> = {
  node: `// =========================================================================
// sillyGirl 插件元数据一览：按需保留/修改，不认识的字段会被自动忽略
// 注释前缀：JS 用 //，Python 用 #；也兼容块注释 @key value 写法
// =========================================================================
// ── 基础信息 ──
// [title: 示例插件]                     插件标题（必填）
// [name: demoPlugin]                    插件标识（必填，建议与文件名一致）
// [desc: 一句话介绍这个插件做什么]      描述，展示在插件市场（也兼容 [description: ...]）
// [version: v1.0.0]                     版本号
// [author: your-name]                   作者
// [icon: https://example.com/icon.png]  图标地址，不填使用默认图标
// [class: 工具]                         分类，可写多个（空格分隔）：工具/查询/娱乐/影音/生活/图片/游戏，也可自定义
// [origin: 自定义]                      来源标识
// ── 触发与执行 ──
// [rule: ^菜单$]                        触发正则，可写多条 [rule: ...]
// [rule: ^查询 (?<关键词>.+)$]          正则具名参数 (?<名称>...) 会自动提取消息参数
// [admin: false]                        true = 仅管理员可触发
// [priority: 0]                         优先级，数字越大越先执行
// [cron: 0 9 * * *]                     定时执行（5 或 6 位 cron），可与 rule 共存
// [on_start: false]                     true = sillyGirl 启动时执行，常驻任务从此开始
// [web: false]                          true = 常驻后台服务（自动附带 on_start 效果）
// [module: false]                       true = 模块插件：供其他插件 [depe] 引用，自身不响应消息
// [carry: false]                        true = 作为消息搬运的处理脚本
// ── 开关与市场 ──
// [status: true]                        插件总开关，false = 停用（兼容 AutMan 的 [disable: false] 反向写法）
// [public: false]                       是否公开到插件市场
// [depe: ["axios"]]                     依赖声明：npm/pip 包名或 ./本地模块，可写多条 [depe: ...]
// ── 配置表单 ──
// 配置统一用 plugin.Form 代码注册（旧 [param: {...}] 头注已废弃）：
// const form = new plugin.Form({
//   apiKey: plugin.Form.string().title("接口密钥").description("在插件设置弹窗里填写"),
// });

const { sender: s, Bucket, plugin, utils } = require('sillygirl');

// 常用 API 速查：
//   s.reply(text)                              回复当前消息
//   s.getMsg() / s.getMsgId()                  原始消息内容 / 消息 ID
//   s.getUserId() / s.getUserName()            发送者 ID / 昵称
//   s.getChatId() / s.getPlatform()            会话 ID / 当前平台
//   s.getBotId() / s.isAdmin()                 机器人 ID / 是否管理员
//   s.pushAdmin(text)                          给管理员推送消息
//   s.listen({ rules, timeout, handle })       连续对话监听，handle 返回值会回复用户
//   s.doAction({ action: "delete_msg", ... })  平台动作（撤回等）
//   new Bucket("demo.data")                    键值存储：get/set/getAll/keys/delete/count
//   utils.sleep(ms)                            异步等待

async function main() {
  await s.reply('插件已就绪，发送「菜单」试试');
}

main().catch((error) => console.error(error));
`,
  python: `# =========================================================================
# sillyGirl 插件元数据一览：按需保留/修改，不认识的字段会被自动忽略
# 注释前缀：Python 用 #；也兼容块注释 @key value 写法
# =========================================================================
# ── 基础信息 ──
# [title: 示例插件]                     插件标题（必填）
# [name: demoPlugin]                    插件标识（必填，建议与文件名一致）
# [desc: 一句话介绍这个插件做什么]      描述，展示在插件市场（也兼容 [description: ...]）
# [version: v1.0.0]                     版本号
# [author: your-name]                   作者
# [icon: https://example.com/icon.png]  图标地址，不填使用默认图标
# [class: 工具]                         分类，可写多个（空格分隔）：工具/查询/娱乐/影音/生活/图片/游戏，也可自定义
# [origin: 自定义]                      来源标识
# ── 触发与执行 ──
# [rule: ^菜单$]                        触发正则，可写多条 [rule: ...]
# [rule: ^查询 (?<关键词>.+)$]          正则具名参数 (?<名称>...) 会自动提取消息参数
# [admin: false]                        true = 仅管理员可触发
# [priority: 0]                         优先级，数字越大越先执行
# [cron: 0 9 * * *]                     定时执行（5 或 6 位 cron），可与 rule 共存
# [on_start: false]                     true = sillyGirl 启动时执行，常驻任务从此开始
# [web: false]                          true = 常驻后台服务（自动附带 on_start 效果）
# [module: false]                       true = 模块插件：供其他插件 [depe] 引用，自身不响应消息
# [carry: false]                        true = 作为消息搬运的处理脚本
# ── 开关与市场 ──
# [status: true]                        插件总开关，false = 停用（兼容 AutMan 的 [disable: false] 反向写法）
# [public: false]                       是否公开到插件市场
# [depe: ["requests"]]                  依赖声明：pip 包名或 ./本地模块，可写多条 [depe: ...]
# ── 配置表单 ──
# 配置统一用 plugin.Form 代码注册（旧 [param: {...}] 头注已废弃）：
# form = plugin.Form({
#     "apiKey": plugin.Form.string().title("接口密钥"),
# })

import asyncio

from sillygirl import Bucket, plugin, sender as s, utils

# 常用 API 速查：
#   await s.reply(text)                        回复当前消息
#   await s.getMsg() / await s.getMsgId()      原始消息内容 / 消息 ID
#   await s.getUserId() / await s.getUserName()  发送者 ID / 昵称
#   await s.getChatId() / await s.getPlatform()  会话 ID / 当前平台
#   await s.getBotId() / await s.isAdmin()     机器人 ID / 是否管理员
#   await s.pushAdmin(text)                    给管理员推送消息
#   await s.listen({ ... })                    连续对话监听
#   bucket = Bucket("demo.data")               键值存储：await get/set/getAll/keys/delete/count
#   await utils.sleep(ms)                      异步等待

async def main():
    await s.reply("插件已就绪，发送「菜单」试试")


asyncio.run(main())
`,
  es5: `// =========================================================================
// sillyGirl 插件元数据一览（ES5）：按需保留/修改，不认识的字段会被自动忽略
// =========================================================================
// ── 基础信息 ──
// [title: ES5 示例插件]                 插件标题（必填）
// [name: es5Demo]                       插件标识（必填）
// [desc: 一句话介绍这个插件做什么]      描述，展示在插件市场
// [version: v1.0.0]                     版本号
// [author: your-name]                   作者
// [icon: https://example.com/icon.png]  图标地址
// [class: 工具]                         分类，可写多个（空格分隔）
// ── 触发与执行 ──
// [rule: ^es5命令$]                     触发正则，可写多条 [rule: ...]
// [admin: false]                        true = 仅管理员可触发
// [priority: 0]                         优先级，数字越大越先执行
// [cron: 0 9 * * *]                     定时执行（5 或 6 位 cron）
// [on_start: false]                     true = sillyGirl 启动时执行
// [module: false]                       true = 模块插件，供其他插件 [depe] 引用
// ── 开关与市场 ──
// [status: true]                        插件总开关，false = 停用
// [public: false]                       是否公开到插件市场
// [depe: []]                            依赖声明：npm 包名或 ./本地模块

// ES5 环境无 require；如运行环境注入了 sender 全局，可直接 sender.reply(...)
var name = "sillyGirl";

function greet(who) {
  console.log("hello " + who);
}

greet(name);
`,
  "adapter-js": `// =========================================================================
// sillyGirl 适配器插件元数据（JS）：适配器用于接入外部平台消息，
// 不使用 rule/cron/on_start 等消息触发字段
// =========================================================================
// [title: 适配器示例]                   适配器标题
// [name: adapterDemo]                   适配器标识
// [desc: 接入自定义平台的消息适配器]    描述
// [version: v1.0.0]                     版本号
// [author: your-name]                   作者
// [class: 适配器]                       分类
// [status: true]                        总开关，false = 停用

const { Adapter } = require('sillygirl');

// Adapter 用法速查：
//   platform / bot_id              平台与机器人标识，核心按它路由消息
//   replyHandler / actionHandler   核心下发回复 / 动作时触发，返回字符串作为处理结果
//   adapter.receive(msg)           收到平台消息后投递给核心
//   adapter.push(msg)              主动推送，返回平台消息 ID
//   adapter.destroy()              注销（未传 replyHandler 时为安全空操作）
const adapter = new Adapter({
  platform: 'demo',
  bot_id: 'demo-bot',
  replyHandler: async (message) => {
    // 在这里把核心回复转换成平台消息发送出去
    return '';
  },
});

console.log('adapter bootstrap');
`,
  "adapter-python": `# =========================================================================
# sillyGirl 适配器插件元数据（Python）：适配器用于接入外部平台消息，
# 不使用 rule/cron/on_start 等消息触发字段
# =========================================================================
# [title: Python 适配器示例]            适配器标题
# [name: pyAdapterDemo]                 适配器标识
# [desc: 接入自定义平台的消息适配器]    描述
# [version: v1.0.0]                     版本号
# [author: your-name]                   作者
# [class: 适配器]                       分类
# [status: true]                        总开关，false = 停用

import asyncio

from sillygirl import Adapter


async def main():
    def reply_handler(message):
        # 在这里把核心回复转换成平台消息发送出去
        return ""

    adapter = Adapter(platform="demo", bot_id="demo-bot", replyHandler=reply_handler)
    # adapter.receive(msg) 投递消息给核心；adapter.push(msg) 主动推送；await adapter.destroy() 注销
    print("python adapter ready")


asyncio.run(main())
`,
  typescript: `// =========================================================================
// sillyGirl 插件元数据一览（TypeScript 占位脚本，暂不支持在线调试）
// 元数据字段与 Node.js 插件完全一致，保存后仅落盘不加载
// =========================================================================
// [title: TypeScript 示例]
// [name: typescriptDemo]
// [desc: TypeScript 应用脚本说明]
// [version: v1.0.0]
// [author: your-name]
// [class: 工具]
// [rule: ^ts命令$]
// [admin: false]
// [priority: 0]
// [cron: 0 9 * * *]
// [on_start: false]
// [status: true]
// [public: false]

// TypeScript 应用脚本（暂不支持在线调试）
const name: string = "sillyGirl";

function greet(who: string): string {
  return \`hello \${who}\`;
}

console.log(greet(name));
`,
  golang: `// =========================================================================
// sillyGirl 插件元数据一览（Golang 占位脚本，暂不支持在线调试）
// 元数据字段与 Node.js 插件一致，保存后仅落盘不编译
// =========================================================================
// [title: Golang 示例]
// [name: golangDemo]
// [desc: Golang 应用脚本说明]
// [version: v1.0.0]
// [author: your-name]
// [class: 工具]
// [rule: ^go命令$]
// [status: true]
// [public: false]

// Golang 应用脚本（暂不支持在线调试）
package main

import "fmt"

func main() {
  fmt.Println("golang app script")
}
`,
  "adapter-go": `// =========================================================================
// sillyGirl 适配器插件元数据（Golang 占位脚本，暂不支持在线调试）
// 适配器不使用 rule/cron/on_start 等消息触发字段；保存后仅落盘不编译
// =========================================================================
// [title: Golang 适配器示例]
// [name: adapterGoDemo]
// [desc: Golang 平台适配器说明]
// [version: v1.0.0]
// [author: your-name]
// [class: 适配器]
// [status: true]

// Golang 适配器脚本（暂不支持在线调试）
package main

import "fmt"

func main() {
  fmt.Println("golang adapter script")
}
`,
};

type ScriptEditorRuntime = {
  Compartment: typeof import("@codemirror/state").Compartment;
  EditorState: typeof import("@codemirror/state").EditorState;
  EditorView: typeof import("@codemirror/view").EditorView;
  javascript: typeof import("@codemirror/lang-javascript").javascript;
  python: typeof import("@codemirror/lang-python").python;
  oneDark: typeof import("@codemirror/theme-one-dark").oneDark;
  basicSetup: typeof import("codemirror").basicSetup;
};

type AppScriptDetail = {
  id: string;
  title?: string;
  name?: string;
  language?: AppScriptLanguage;
  executable?: boolean;
  file?: string;
  path?: string;
  installed?: boolean;
  content: string;
};

export function useScriptsAdmin() {
  const scripts = reactive({
    items: [] as AppScriptInfo[],
    loading: false,
    keyword: "",
    category: "" as AppScriptLanguage | "",
    expanded: {} as Record<string, boolean>,
  });

  const scriptsTotal = computed(() => scripts.items.length);
  const scriptCategoryCounts = computed(() => {
    const counts: Record<string, number> = {};
    for (const item of scripts.items) {
      counts[item.language] = (counts[item.language] || 0) + 1;
    }
    return counts;
  });
  const filteredScriptItems = computed(() => {
    const keyword = scripts.keyword.trim().toLowerCase();
    return scripts.items.filter((item) => {
      if (
        scripts.category &&
        item.language !== (scripts.category as AppScriptLanguage)
      )
        return false;
      if (!keyword) return true;
      const haystack = `${item.title}\n${item.name}\n${item.file}`.toLowerCase();
      return haystack.includes(keyword);
    });
  });
  // 分类树：每个分类下属脚本（受搜索词过滤，不受分类选中过滤）。
  const scriptCategoryChildren = computed(() => {
    const keyword = scripts.keyword.trim().toLowerCase();
    const groups: Record<string, AppScriptInfo[]> = {};
    for (const language of appScriptLanguageOrder) {
      groups[language] = [];
    }
    for (const item of scripts.items) {
      if (keyword) {
        const haystack = `${item.title}\n${item.name}\n${item.file}`.toLowerCase();
        if (!haystack.includes(keyword)) continue;
      }
      const list = groups[item.language];
      if (list) list.push(item);
    }
    return groups;
  });

  const scriptsSearchKeyword = computed(() => scripts.keyword.trim());
  const hasScriptsKeyword = computed(() => !!scriptsSearchKeyword.value);
  // 搜索时展示命中分类（自动展开有结果的分类，无结果时显示全部）。
  const scriptSearchCategories = computed(() => {
    const keyword = scriptsSearchKeyword.value.toLowerCase();
    if (!keyword) return null;
    const children = scriptCategoryChildren.value;
    return scriptCategories.value.filter(
      (category) => (children[category.key] || []).length > 0,
    );
  });

  function toggleScriptCategory(language: string) {
    // 点分类：切换展开/收起；首次点击即展开。
    scripts.expanded[language] = !scripts.expanded[language];
    if (scripts.expanded[language]) {
      scripts.category = language as AppScriptLanguage;
    } else if (scripts.category === language) {
      scripts.category = "";
    }
  }

  async function loadAppScripts(options?: { silent?: boolean }) {
    scripts.loading = !options?.silent;
    try {
      const res = await get<ApiEnvelope<{ items: AppScriptInfo[] }>>(
        "/api/admin/app-scripts",
      );
      const data = apiData(res);
      scripts.items = data?.items || [];
    } catch (error) {
      message.error(
        error instanceof Error ? error.message : "应用脚本列表加载失败",
      );
    } finally {
      scripts.loading = false;
    }
  }

  // 脚本启用状态切换：复用本地插件状态接口（改写 [status] 注释后热重载）。
  const scriptStatusToggling = reactive({} as Record<string, boolean>);

  async function toggleAppScriptStatus(
    item: AppScriptInfo,
    status = !(item.status ?? true),
  ) {
    scriptStatusToggling[item.id] = true;
    try {
      const res = await post<ApiEnvelope<{ id?: string; status?: boolean }>>(
        `/api/admin/local-plugins/${encodeURIComponent(item.id)}/status`,
        { status },
      );
      const data = apiData(res);
      item.status = data?.status ?? status;
      message.success(
        `${item.title || item.name} 已${item.status ? "启用" : "停用"}`,
      );
    } catch (error) {
      message.error(
        error instanceof Error ? error.message : "脚本状态更新失败",
      );
    } finally {
      scriptStatusToggling[item.id] = false;
    }
  }

  const scriptCategories = computed(() =>
    appScriptLanguageOrder.map((language) => ({
      key: language,
      label: appScriptLanguageLabels[language],
      count: scriptCategoryCounts.value[language] || 0,
      executable: appScriptLanguageExecutable(language),
    })),
  );

  function appScriptLanguageExecutable(language: string) {
    return !["typescript", "golang", "adapter-go"].includes(language);
  }

  const scriptEditor = reactive({
    open: false,
    loading: false,
    saving: false,
    deleting: false,
    isNew: false,
    id: "",
    name: "",
    title: "",
    language: "node" as AppScriptLanguage,
    theme: "dark" as "dark" | "light",
    executable: true,
    installed: false,
    content: "",
    file: "",
    row: null as AppScriptInfo | null,
  });
  const scriptEditorHost = ref<HTMLElement | null>(null);

  const scriptDebug = reactive({
    running: false,
    lines: [] as Array<{ kind: string; text: string }>,
    exitCode: null as number | null,
    exitMessage: "",
    error: "",
  });
  let scriptDebugSource: EventSource | null = null;

  let scriptEditorRuntime: ScriptEditorRuntime | null = null;
  let scriptEditorRuntimePromise: Promise<ScriptEditorRuntime> | null = null;
  let scriptEditorEditable: Compartment | null = null;
  let scriptEditorLanguage: Compartment | null = null;
  let scriptEditorTheme: Compartment | null = null;
  let scriptEditorView: EditorView | null = null;

  async function loadScriptEditorRuntime() {
    if (scriptEditorRuntime) return scriptEditorRuntime;
    if (!scriptEditorRuntimePromise) {
      scriptEditorRuntimePromise = Promise.all([
        import("@codemirror/state"),
        import("@codemirror/view"),
        import("@codemirror/lang-javascript"),
        import("@codemirror/lang-python"),
        import("@codemirror/theme-one-dark"),
        import("codemirror"),
      ]).then(([state, view, javascriptLanguage, pythonLanguage, theme, codemirror]) => ({
        Compartment: state.Compartment,
        EditorState: state.EditorState,
        EditorView: view.EditorView,
        javascript: javascriptLanguage.javascript,
        python: pythonLanguage.python,
        oneDark: theme.oneDark,
        basicSetup: codemirror.basicSetup,
      }));
    }
    scriptEditorRuntime = await scriptEditorRuntimePromise;
    if (!scriptEditorEditable) {
      scriptEditorEditable = new scriptEditorRuntime.Compartment();
      scriptEditorLanguage = new scriptEditorRuntime.Compartment();
      scriptEditorTheme = new scriptEditorRuntime.Compartment();
    }
    return scriptEditorRuntime;
  }

  function scriptEditorLanguageExtension(): Extension {
    if (!scriptEditorRuntime) return [];
    switch (scriptEditor.language) {
      case "python":
      case "adapter-python":
        return scriptEditorRuntime.python();
      case "typescript":
      case "adapter-js":
      case "es5":
      case "node":
        return scriptEditorRuntime.javascript({
          typescript: scriptEditor.language === "typescript",
        });
      default:
        return [];
    }
  }

  function scriptEditorThemeExtension(): Extension {
    return scriptEditor.theme === "dark" && scriptEditorRuntime
      ? scriptEditorRuntime.oneDark
      : [];
  }

  function syncScriptEditorContent(value = scriptEditor.content) {
    if (!scriptEditorView) return;
    const current = scriptEditorView.state.doc.toString();
    if (current === value) return;
    scriptEditorView.dispatch({
      changes: { from: 0, to: current.length, insert: value },
    });
  }

  function syncScriptEditorLanguage() {
    if (!scriptEditorLanguage) return;
    scriptEditorView?.dispatch({
      effects: scriptEditorLanguage.reconfigure(
        scriptEditorLanguageExtension(),
      ),
    });
  }

  function syncScriptEditorTheme() {
    if (!scriptEditorTheme) return;
    scriptEditorView?.dispatch({
      effects: scriptEditorTheme.reconfigure(scriptEditorThemeExtension()),
    });
  }

  function toggleScriptEditorTheme() {
    scriptEditor.theme = scriptEditor.theme === "dark" ? "light" : "dark";
    syncScriptEditorTheme();
  }

  function destroyScriptEditor() {
    scriptEditorView?.destroy();
    scriptEditorView = null;
  }

  async function initScriptEditor() {
    if (scriptEditorView || !scriptEditorHost.value) return;
    const runtime = await loadScriptEditorRuntime();
    if (scriptEditorView || !scriptEditorHost.value) return;
    if (!scriptEditorLanguage || !scriptEditorTheme || !scriptEditorEditable)
      return;
    scriptEditorView = new runtime.EditorView({
      parent: scriptEditorHost.value,
      state: runtime.EditorState.create({
        doc: scriptEditor.content,
        extensions: [
          runtime.basicSetup,
          scriptEditorLanguage.of(scriptEditorLanguageExtension()),
          scriptEditorTheme.of(scriptEditorThemeExtension()),
          scriptEditorEditable.of(runtime.EditorView.editable.of(true)),
          runtime.EditorView.updateListener.of((update) => {
            if (update.docChanged)
              scriptEditor.content = update.state.doc.toString();
          }),
        ],
      }),
    });
  }

  function openNewScriptEditor(language: AppScriptLanguage = "node") {
    scriptEditor.isNew = true;
    scriptEditor.id = "";
    scriptEditor.name = "";
    scriptEditor.title = "新建脚本";
    scriptEditor.language = language;
    scriptEditor.executable = appScriptLanguageExecutable(language);
    scriptEditor.installed = false;
    scriptEditor.row = null;
    scriptEditor.content = appScriptStarters[language] || "";
    scriptEditor.open = true;
    scriptEditor.loading = false;
    stopScriptDebug();
    nextTick(() => {
      destroyScriptEditor();
      void initScriptEditor();
    });
  }

  async function openScriptEditor(row: AppScriptInfo) {
    scriptEditor.isNew = false;
    scriptEditor.id = row.id;
    scriptEditor.name = row.name || row.title || row.id;
    scriptEditor.title = row.title || row.name || row.id;
    scriptEditor.language = (row.language as AppScriptLanguage) || "node";
    scriptEditor.executable = row.executable !== false;
    scriptEditor.installed = row.installed !== false;
    scriptEditor.file = row.file || "";
    scriptEditor.row = row;
    scriptEditor.content = "";
    scriptEditor.open = true;
    scriptEditor.loading = true;
    stopScriptDebug();
    await nextTick();
    destroyScriptEditor();
    await initScriptEditor();
    try {
      const res = await get<ApiEnvelope<AppScriptDetail>>(
        `/api/admin/app-scripts/${encodeURIComponent(row.id)}`,
      );
      const data = apiData(res);
      scriptEditor.id = data.id || row.id;
      scriptEditor.name = data.name || row.name || row.id;
      scriptEditor.title = data.title || row.title || row.id;
      scriptEditor.language = (data.language as AppScriptLanguage) || scriptEditor.language;
      scriptEditor.executable = data.executable !== false;
      scriptEditor.installed = data.installed !== false;
      scriptEditor.file = data.file || data.path || scriptEditor.file;
      scriptEditor.content = data.content || "";
      syncScriptEditorLanguage();
      syncScriptEditorContent(scriptEditor.content);
    } catch (error) {
      message.error(
        error instanceof Error ? error.message : "读取脚本源码失败",
      );
    } finally {
      scriptEditor.loading = false;
    }
  }

  async function createScript(payload: {
    name: string;
    language: AppScriptLanguage;
    content: string;
  }) {
    scriptEditor.saving = true;
    try {
      const res = await post<ApiEnvelope<AppScriptInfo>>(
        "/api/admin/app-scripts",
        payload,
      );
      const data = apiData(res);
      message.success("脚本已创建");
      scriptEditor.open = false;
      destroyScriptEditor();
      await loadAppScripts();
      return data;
    } catch (error) {
      message.error(error instanceof Error ? error.message : "创建脚本失败");
      return null;
    } finally {
      scriptEditor.saving = false;
    }
  }

  async function saveScriptEditor() {
    if (!scriptEditor.content.trim()) {
      message.warning("脚本内容不能为空");
      return;
    }
    scriptEditor.saving = true;
    try {
      const res = await post<ApiEnvelope<{ id: string }>>(
        `/api/admin/app-scripts/${encodeURIComponent(scriptEditor.id)}`,
        { name: scriptEditor.name, language: scriptEditor.language, content: scriptEditor.content },
      );
      const data = apiData(res);
      if (data?.id) scriptEditor.id = data.id;
      scriptEditor.installed = true;
      message.success("脚本已保存");
      await loadAppScripts();
    } catch (error) {
      message.error(error instanceof Error ? error.message : "保存脚本失败");
    } finally {
      scriptEditor.saving = false;
    }
  }

  async function deleteScriptEditor() {
    if (!scriptEditor.id) return;
    scriptEditor.deleting = true;
    try {
      await post(
        `/api/admin/app-scripts/${encodeURIComponent(scriptEditor.id)}/deletions`,
      );
      message.success("脚本已删除");
      scriptEditor.open = false;
      destroyScriptEditor();
      stopScriptDebug();
      await loadAppScripts();
    } catch (error) {
      message.error(error instanceof Error ? error.message : "删除脚本失败");
    } finally {
      scriptEditor.deleting = false;
    }
  }

  function stopScriptDebug() {
    if (scriptDebugSource) {
      scriptDebugSource.close();
      scriptDebugSource = null;
    }
    scriptDebug.running = false;
  }

  function clearScriptDebug() {
    scriptDebug.lines = [];
    scriptDebug.exitCode = null;
    scriptDebug.exitMessage = "";
    scriptDebug.error = "";
  }

  function runScriptDebug() {
    if (!scriptEditor.id || scriptDebug.running) return;
    if (!scriptEditor.executable) {
      message.info("该语言分类暂不支持在线调试");
      return;
    }
    clearScriptDebug();
    scriptDebug.running = true;
    scriptDebug.exitMessage = "";
    const token = getAuthToken();
    const url = `/api/admin/app-scripts/${encodeURIComponent(
      scriptEditor.id,
    )}/executions?timeout=120&token=${encodeURIComponent(token)}`;
    const source = new EventSource(url);
    scriptDebugSource = source;
    source.onmessage = (event) => {
      const line = String(event.data || "");
      if (!line) return;
      const kind = line.split(" ")[0];
      const text = line.slice(kind.length).trim();
      if (kind === "exit" || kind === "kill") {
        scriptDebug.exitMessage = text;
        const match = /exit_code=(-?\d+)/.exec(text);
        scriptDebug.exitCode = match ? Number(match[1]) : null;
        return;
      }
      scriptDebug.lines.push({ kind, text });
      if (scriptDebug.lines.length > 2000) {
        scriptDebug.lines.splice(0, scriptDebug.lines.length - 2000);
      }
    };
    source.onerror = () => {
      if (scriptDebug.running) {
        scriptDebug.error = "调试连接已断开";
      }
      scriptDebug.running = false;
      stopScriptDebug();
    };
    const pollExit = window.setInterval(() => {
      if (scriptDebug.exitCode !== null || scriptDebug.exitMessage) {
        window.clearInterval(pollExit);
        scriptDebug.running = false;
        stopScriptDebug();
      }
    }, 500);
  }

  const activeScript = computed(() => scriptEditor.row);

  // 进入插件开发页：加载列表；默认不展开任何分类，右侧打开一个带文档
  // 标准插件注释写法模板（node）的新建脚本，方便用户照模板编写。
  async function enterScriptsPage() {
    await loadAppScripts();
    if (scriptEditor.open) return;
    openNewScriptEditor("node");
  }

  return {
    scripts,
    scriptsTotal,
    scriptCategories,
    scriptCategoryCounts,
    scriptCategoryChildren,
    scriptsSearchKeyword,
    hasScriptsKeyword,
    scriptSearchCategories,
    toggleScriptCategory,
    filteredScriptItems,
    scriptStatusToggling,
    toggleAppScriptStatus,
    loadAppScripts,
    enterScriptsPage,
    scriptEditor,
    scriptEditorHost,
    activeScript,
    openNewScriptEditor,
    openScriptEditor,
    createScript,
    saveScriptEditor,
    deleteScriptEditor,
    syncScriptEditorLanguage,
    toggleScriptEditorTheme,
    initScriptEditor,
    destroyScriptEditor,
    scriptDebug,
    runScriptDebug,
    stopScriptDebug,
    clearScriptDebug,
  };
}

export type ScriptsAdmin = ReturnType<typeof useScriptsAdmin>;
