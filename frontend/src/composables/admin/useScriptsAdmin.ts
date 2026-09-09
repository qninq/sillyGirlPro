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

const appScriptLanguageOrder: AppScriptLanguage[] = [
  "es5",
  "node",
  "python",
  "typescript",
  "golang",
  "adapter-js",
  "adapter-python",
  "adapter-go",
];

const appScriptStarters: Record<AppScriptLanguage, string> = {
  node: `// [title: 新式插件示例]
// [name: newPlugin]
// [desc: 新式插件说明]
// [rule: ^新式命令$]
// [version: v1.0.0]
// [author: admin]
// [class: 工具]
// [depe: ["axios"]]
// [status: true]

const { sender: s } = require('sillygirl');

async function main() {
  s.reply('插件已就绪');
}

main().catch((error) => console.error(error));
`,
  python: `# [title: 新式Python插件示例]
# [name: pythonDemo]
# [desc: 新式Python插件说明]
# [rule: ^新式命令$]
# [version: v1.0.0]
# [author: admin]
# [class: 工具]
# [depe: ["requests"]]
# [status: true]

import asyncio
from sillygirl import sender as s


async def main():
    await s.reply("插件已就绪")


asyncio.run(main())
`,
  es5: `// [title: ES5 插件示例]
// [name: es5Demo]
// [desc: ES5 插件说明]
// [rule: ^es5命令$]
// [version: v1.0.0]
// [author: admin]
// [class: 工具]
// [status: true]

var name = "sillyGirl";

function greet(who) {
  console.log("hello " + who);
}

greet(name);
`,
  "adapter-js": `// [title: Adapter 插件示例]
// [name: adapterDemo]
// [desc: 平台适配器插件说明]
// [version: v1.0.0]
// [author: admin]
// [class: 适配器]
// [status: true]

const { Adapter } = require('sillygirl');

// 适配器脚本通过 new Adapter({ platform, botId }) 接入平台消息。
console.log('adapter script bootstrap');
`,
  "adapter-python": `# [title: Python Adapter 插件示例]
# [name: pyAdapterDemo]
# [desc: Python 平台适配器插件说明]
# [version: v1.0.0]
# [author: admin]
# [class: 适配器]
# [status: true]

import asyncio
from sillygirl import sender as s


async def main():
    adapter = await s.getAdapter()
    print("python adapter ready:", adapter)


asyncio.run(main())
`,
  typescript: `// [title: TypeScript 插件示例]
// [name: typescriptDemo]
// [desc: TypeScript 插件说明]
// [rule: ^ts命令$]
// [version: v1.0.0]
// [author: admin]
// [class: 工具]
// [status: true]

// TypeScript 应用脚本（暂不支持在线调试）
const name: string = "sillyGirl";

function greet(who: string): string {
  return \`hello \${who}\`;
}

console.log(greet(name));
`,
  golang: `// [title: Golang 插件示例]
// [name: golangDemo]
// [desc: Golang 插件说明]
// [rule: ^go命令$]
// [version: v1.0.0]
// [author: admin]
// [class: 工具]
// [status: true]

// Golang 应用脚本（暂不支持在线调试）
package main

import "fmt"

func main() {
  fmt.Println("golang app script")
}
`,
  "adapter-go": `// [title: Golang Adapter 插件示例]
// [name: adapterGoDemo]
// [desc: Golang 平台适配器插件说明]
// [version: v1.0.0]
// [author: admin]
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
