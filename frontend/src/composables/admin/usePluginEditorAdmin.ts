// 插件编辑器弹窗共享逻辑：插件市场「新增/编辑本地插件」与插件开发页「新建应用脚本」共用。
import { computed, nextTick, reactive, ref } from "vue";
import type { Compartment, Extension } from "@codemirror/state";
import type { EditorView } from "@codemirror/view";
import message from "ant-design-vue/es/message";
import { get, post } from "../../api";
import type { PluginInfo } from "../../types";
import { declaredPluginDependenciesFromContent } from "./pluginInstallPrompt";
import { apiData, type ApiEnvelope } from "./adminApi";
import {
  appScriptLanguageLabels,
  appScriptLanguageOrder,
  appScriptStarters,
  type AppScriptLanguage,
} from "./useScriptsAdmin";

type DependencyRuntime = "node" | "python";

export type UsePluginEditorAdminDeps = {
  plugins: { current: number; pageSize: number; tab: string };
  loadPlugins: (
    current?: number,
    pageSize?: number,
    refresh?: boolean,
    includeBootstrap?: boolean,
  ) => Promise<void>;
  loadUser: (
    setBooting?: boolean,
    reloadSetupOnUnauthorized?: boolean,
  ) => Promise<void>;
  createScript: (payload: {
    name: string;
    language: AppScriptLanguage;
    content: string;
  }) => Promise<unknown>;
  pluginInstalled: (row: PluginInfo) => boolean;
  marketPluginDependencyRuntime: (row: PluginInfo) => DependencyRuntime;
  offerPluginDependencyInstall: (
    row: PluginInfo,
    marketPage?: number,
    marketPageSize?: number,
  ) => Promise<void>;
};

export function usePluginEditorAdmin(deps: UsePluginEditorAdminDeps) {
  const { plugins, loadPlugins, loadUser, createScript } = deps;
  const { pluginInstalled, marketPluginDependencyRuntime } = deps;
  const { offerPluginDependencyInstall } = deps;

  const pluginEditor = reactive({
    open: false,
    loading: false,
    saving: false,
    deleting: false,
    isNew: false,
    id: "",
    name: "",
    title: "",
    type: "node" as DependencyRuntime | AppScriptLanguage,
    theme: "dark" as "dark" | "light",
    installed: false,
    content: "",
    row: null as PluginInfo | null,
    // plugin：插件市场新增/编辑本地插件；script：插件开发页新建应用脚本。
    mode: "plugin" as "plugin" | "script",
  });
  const pluginEditorHost = ref<HTMLElement | null>(null);
  type PluginEditorRuntime = {
    Compartment: typeof import("@codemirror/state").Compartment;
    EditorState: typeof import("@codemirror/state").EditorState;
    EditorView: typeof import("@codemirror/view").EditorView;
    javascript: typeof import("@codemirror/lang-javascript").javascript;
    python: typeof import("@codemirror/lang-python").python;
    oneDark: typeof import("@codemirror/theme-one-dark").oneDark;
    basicSetup: typeof import("codemirror").basicSetup;
  };
  let pluginEditorRuntime: PluginEditorRuntime | null = null;
  let pluginEditorRuntimePromise: Promise<PluginEditorRuntime> | null = null;
  let pluginEditorEditable: Compartment | null = null;
  let pluginEditorLanguage: Compartment | null = null;
  let pluginEditorTheme: Compartment | null = null;
  let pluginEditorView: EditorView | null = null;

  async function loadPluginEditorRuntime() {
    if (pluginEditorRuntime) return pluginEditorRuntime;
    if (!pluginEditorRuntimePromise) {
      pluginEditorRuntimePromise = Promise.all([
        import("@codemirror/state"),
        import("@codemirror/view"),
        import("@codemirror/lang-javascript"),
        import("@codemirror/lang-python"),
        import("@codemirror/theme-one-dark"),
        import("codemirror"),
      ]).then(
        ([
          state,
          view,
          javascriptLanguage,
          pythonLanguage,
          theme,
          codemirror,
        ]) => ({
          Compartment: state.Compartment,
          EditorState: state.EditorState,
          EditorView: view.EditorView,
          javascript: javascriptLanguage.javascript,
          python: pythonLanguage.python,
          oneDark: theme.oneDark,
          basicSetup: codemirror.basicSetup,
        }),
      );
    }
    pluginEditorRuntime = await pluginEditorRuntimePromise;
    if (!pluginEditorEditable) {
      pluginEditorEditable = new pluginEditorRuntime.Compartment();
      pluginEditorLanguage = new pluginEditorRuntime.Compartment();
      pluginEditorTheme = new pluginEditorRuntime.Compartment();
    }
    return pluginEditorRuntime;
  }

  const pluginEditorStarter = appScriptStarters.node;

  function pluginEditorLanguageExtension(): Extension {
    if (!pluginEditorRuntime) return [];
    if (pluginEditor.mode === "script") {
      switch (pluginEditor.type) {
        case "python":
        case "adapter-python":
          return pluginEditorRuntime.python();
        case "typescript":
          return pluginEditorRuntime.javascript({ typescript: true });
        case "es5":
        case "node":
        case "adapter-js":
          return pluginEditorRuntime.javascript();
        default:
          return [];
      }
    }
    return pluginEditor.type === "python" ||
      /from sillygirl import|import sillygirl/.test(pluginEditor.content)
      ? pluginEditorRuntime.python()
      : pluginEditorRuntime.javascript();
  }
  function syncPluginEditorContent(value = pluginEditor.content) {
    if (!pluginEditorView) return;
    const current = pluginEditorView.state.doc.toString();
    if (current === value) return;
    pluginEditorView.dispatch({
      changes: { from: 0, to: current.length, insert: value },
    });
  }
  function syncPluginEditorLanguage() {
    if (!pluginEditorLanguage) return;
    pluginEditorView?.dispatch({
      effects: pluginEditorLanguage.reconfigure(
        pluginEditorLanguageExtension(),
      ),
    });
  }
  function pluginEditorThemeExtension(): Extension {
    return pluginEditor.theme === "dark" && pluginEditorRuntime
      ? pluginEditorRuntime.oneDark
      : [];
  }
  function syncPluginEditorTheme() {
    if (!pluginEditorTheme) return;
    pluginEditorView?.dispatch({
      effects: pluginEditorTheme.reconfigure(pluginEditorThemeExtension()),
    });
  }
  function togglePluginEditorTheme() {
    pluginEditor.theme = pluginEditor.theme === "dark" ? "light" : "dark";
    syncPluginEditorTheme();
  }
  function destroyPluginEditor() {
    pluginEditorView?.destroy();
    pluginEditorView = null;
  }
  async function initPluginEditor() {
    if (pluginEditorView || !pluginEditorHost.value) return;
    const runtime = await loadPluginEditorRuntime();
    if (pluginEditorView || !pluginEditorHost.value) return;
    if (!pluginEditorLanguage || !pluginEditorTheme || !pluginEditorEditable)
      return;
    pluginEditorView = new runtime.EditorView({
      parent: pluginEditorHost.value,
      state: runtime.EditorState.create({
        doc: pluginEditor.content,
        extensions: [
          runtime.basicSetup,
          pluginEditorLanguage.of(pluginEditorLanguageExtension()),
          pluginEditorTheme.of(pluginEditorThemeExtension()),
          pluginEditorEditable.of(runtime.EditorView.editable.of(true)),
          runtime.EditorView.updateListener.of((update) => {
            if (update.docChanged)
              pluginEditor.content = update.state.doc.toString();
          }),
        ],
      }),
    });
  }
  function openNewMarketPluginEditor() {
    pluginEditor.mode = "plugin";
    pluginEditor.isNew = true;
    pluginEditor.id = "";
    pluginEditor.name = "localPlugin";
    pluginEditor.title = "新增本地插件";
    pluginEditor.type = "node";
    pluginEditor.installed = false;
    pluginEditor.row = null;
    pluginEditor.content = pluginEditorStarter;
    pluginEditor.open = true;
    pluginEditor.loading = false;
    nextTick(() => {
      destroyPluginEditor();
      void initPluginEditor();
    });
  }
  async function openMarketPluginEditor(row: PluginInfo) {
    pluginEditor.mode = "plugin";
    pluginEditor.isNew = false;
    pluginEditor.id = row.id;
    pluginEditor.name = row.title || row.id;
    pluginEditor.title = row.title || row.id;
    pluginEditor.type = marketPluginDependencyRuntime(row);
    pluginEditor.installed = pluginInstalled(row);
    pluginEditor.row = row;
    pluginEditor.content = "";
    pluginEditor.open = true;
    pluginEditor.loading = true;
    await nextTick();
    destroyPluginEditor();
    await initPluginEditor();
    try {
      const res = await get<
        ApiEnvelope<{
          id: string;
          title?: string;
          name?: string;
          type?: string;
          installed?: boolean;
          content: string;
        }>
      >(`/api/admin/local-plugins/${encodeURIComponent(row.id)}`);
      const data = apiData(res);
      pluginEditor.id = data.id || row.id;
      pluginEditor.name = data.name || row.title || row.id;
      pluginEditor.title = data.title || row.title || row.id;
      pluginEditor.type = data.type === "python" ? "python" : "node";
      pluginEditor.installed = data.installed !== false;
      pluginEditor.content = data.content || "";
      syncPluginEditorLanguage();
      syncPluginEditorContent(pluginEditor.content);
    } catch (error) {
      message.error(
        error instanceof Error ? error.message : "读取插件源码失败",
      );
    } finally {
      pluginEditor.loading = false;
    }
  }
  function closeMarketPluginEditor() {
    pluginEditor.open = false;
    destroyPluginEditor();
  }
  // 插件开发页「新建」：复用插件编辑器弹窗，全部语言分类可选，保存走应用脚本接口。
  function openNewScriptPluginEditor(language: AppScriptLanguage = "node") {
    pluginEditor.mode = "script";
    pluginEditor.isNew = true;
    pluginEditor.id = "";
    pluginEditor.name = "";
    pluginEditor.title = "新建应用脚本";
    pluginEditor.type = language;
    pluginEditor.installed = false;
    pluginEditor.row = null;
    pluginEditor.content = appScriptStarters[language] || "";
    pluginEditor.open = true;
    pluginEditor.loading = false;
    nextTick(() => {
      destroyPluginEditor();
      void initPluginEditor();
    });
  }
  // 脚本模式语言选项：全部 8 种分类。
  const pluginEditorLanguageOptions = computed(() =>
    pluginEditor.mode === "script"
      ? appScriptLanguageOrder.map((value) => ({
          label: appScriptLanguageLabels[value],
          value,
        }))
      : [
          { label: "NodeJS", value: "node" },
          { label: "Python", value: "python" },
        ],
  );
  // 脚本模式切换语言：内容仍为初始模板时同步替换为对应语言模板。
  function onPluginEditorLanguageChange(value: unknown) {
    if (pluginEditor.mode !== "script") return;
    const language = value as AppScriptLanguage;
    const previous = pluginEditor.type as AppScriptLanguage;
    pluginEditor.type = language;
    syncPluginEditorLanguage();
    if (
      !pluginEditor.content.trim() ||
      pluginEditor.content === appScriptStarters[previous]
    ) {
      pluginEditor.content = appScriptStarters[language] || "";
      syncPluginEditorContent(pluginEditor.content);
    }
  }
  function handlePluginEditorOpenChange(open: boolean) {
    if (open) nextTick(() => void initPluginEditor());
    else destroyPluginEditor();
  }
  function pluginEditorMetaValue(content: string, key: string) {
    const metaKeyWanted = key.toLowerCase();
    let value = "";
    for (const line of String(content || "").split(/\r?\n/)) {
      const legacy =
        /^[ \t]*(?:\/\/|#+)[ \t]*\[[ \t]*([\d\w+-]+)[ \t]*:[ \t]*(.*)[ \t]*\][^\r\n]*$/.exec(
          line,
        );
      if (legacy) {
        const metaKey = String(legacy[1] || "").toLowerCase();
        const metaValue = String(legacy[2] || "").trim();
        if (metaKey === metaKeyWanted && metaValue) value = metaValue;
        continue;
      }
      const at = /^[ \t]*(?:\*[ \t]*)?@([\d\w+-]+)(?:[ \t]+(.+?))?[ \t]*$/.exec(
        line,
      );
      if (!at) continue;
      const metaKey = String(at[1] || "").toLowerCase();
      const metaValue = String(at[2] || "").trim();
      if (metaKey === metaKeyWanted && metaValue) value = metaValue;
    }
    return value;
  }

  function pluginEditorMetaEnabled(content: string, key: string) {
    const value = pluginEditorMetaValue(content, key).toLowerCase();
    return (
      value === "true" || value === "1" || value === "yes" || value === "on"
    );
  }
  function normalizePluginEditorFileBase(value: string) {
    return String(value || "")
      .trim()
      .replace(/\\/g, "/")
      .split("/")
      .pop()!
      .replace(/\.(js|py)$/i, "");
  }
  function validatePluginEditorRequired() {
    const content = pluginEditor.content || "";
    const missing: string[] = [];
    for (const item of ["title", "name", "desc", "version"]) {
      if (!pluginEditorMetaValue(content, item)) {
        missing.push(
          (
            {
              title: "[title: xxx]",
              name: "[name: 文件名]",
              desc: "[desc: xxx]",
              version: "[version: vx.y.z]",
            } as Record<string, string>
          )[item],
        );
      }
    }
    if (
      !pluginEditorMetaValue(content, "rule") &&
      !pluginEditorMetaValue(content, "cron") &&
      !pluginEditorMetaEnabled(content, "on_start") &&
      !pluginEditorMetaEnabled(content, "web") &&
      !pluginEditorMetaEnabled(content, "module")
    ) {
      missing.push(
        "[rule: xxx] 或 [cron: xxx]/[on_start: true]/[web: true]/[module: true]",
      );
    }
    if (missing.length) {
      message.warning(`插件注释缺少必须字段：${missing.join("、")}`);
      return false;
    }
    const inputName = normalizePluginEditorFileBase(pluginEditor.name);
    const metaName = normalizePluginEditorFileBase(
      pluginEditorMetaValue(content, "name"),
    );
    if (!inputName) {
      message.warning("插件名称不能为空，且必须和 [name: 文件名] 一致");
      return false;
    }
    if (inputName !== metaName) {
      message.warning(
        `插件名称必须和 [name: ${metaName || "文件名"}] 一致，当前填写：${inputName}`,
      );
      return false;
    }
    return true;
  }

  async function formatMarketPluginEditor() {
    if (!pluginEditor.content.trim()) return;
    if (pluginEditor.type === "python") {
      message.info("Python 插件暂不做前端格式化，请保存前自行确认缩进");
      return;
    }
    try {
      const [
        { default: prettier },
        { default: parserBabel },
        { default: parserEstree },
      ] = await Promise.all([
        import("prettier/standalone"),
        import("prettier/plugins/babel"),
        import("prettier/plugins/estree"),
      ]);
      const formatted = await prettier.format(pluginEditor.content, {
        parser: "babel",
        plugins: [parserBabel, parserEstree],
        singleQuote: true,
        trailingComma: "all",
      });
      pluginEditor.content = formatted.trimEnd() + "\n";
      syncPluginEditorContent(pluginEditor.content);
      message.success("格式化完成");
    } catch (error) {
      message.error(error instanceof Error ? error.message : "格式化失败");
    }
  }
  async function saveMarketPluginEditor() {
    const nameInput = document.getElementById(
      "plugin-editor-name",
    ) as HTMLInputElement | null;
    if (nameInput) pluginEditor.name = nameInput.value;
    // 脚本模式：走应用脚本创建接口；可执行语言做与后端一致的元数据校验。
    if (pluginEditor.mode === "script") {
      const language = pluginEditor.type as AppScriptLanguage;
      const name = pluginEditor.name.trim();
      if (!name) {
        message.warning("脚本名称不能为空");
        return;
      }
      if (
        !["typescript", "golang", "adapter-go"].includes(language) &&
        !validatePluginEditorRequired()
      )
        return;
      pluginEditor.saving = true;
      try {
        const created = await createScript({
          name,
          language,
          content: pluginEditor.content,
        });
        if (created) {
          pluginEditor.open = false;
          destroyPluginEditor();
        }
      } finally {
        pluginEditor.saving = false;
      }
      return;
    }
    const creatingPlugin = pluginEditor.isNew || !pluginEditor.installed;
    const currentMarketPage = plugins.current;
    if (!validatePluginEditorRequired()) return;
    pluginEditor.saving = true;
    try {
      const payload = {
        id: pluginEditor.id,
        name: pluginEditor.name,
        type: pluginEditor.type,
        content: pluginEditor.content,
      };
      const res = creatingPlugin
        ? await post<
            ApiEnvelope<{
              id: string;
              type?: string;
              title?: string;
              path?: string;
            }>
          >("/api/admin/local-plugins", payload)
        : await post<
            ApiEnvelope<{
              id: string;
              type?: string;
              title?: string;
              path?: string;
            }>
          >(
            `/api/admin/local-plugins/${encodeURIComponent(pluginEditor.id)}`,
            payload,
          );
      const data = apiData(res);
      pluginEditor.id = data?.id || pluginEditor.id;
      pluginEditor.installed = true;
      const savedRuntime: DependencyRuntime =
        data?.type === "python" || pluginEditor.type === "python"
          ? "python"
          : "node";
      const savedDependencies = declaredPluginDependenciesFromContent(
        pluginEditor.content,
      );
      const savedStatus = pluginEditorMetaValue(pluginEditor.content, "status");
      const savedRow: PluginInfo = {
        id: pluginEditor.id,
        title:
          data?.title ||
          pluginEditor.name ||
          pluginEditor.title ||
          pluginEditor.id,
        type: savedRuntime,
        suffix: savedRuntime === "python" ? ".py" : ".js",
        status: savedStatus
          ? pluginEditorMetaEnabled(pluginEditor.content, "status")
          : true,
        install_status: 2,
        address: data?.path
          ? `local://?path=${encodeURIComponent(data.path)}`
          : "",
        dependencies: savedDependencies,
      };
      message.success(creatingPlugin ? "本地插件已新增" : "插件已保存");
      pluginEditor.open = false;
      destroyPluginEditor();
      if (creatingPlugin) plugins.tab = "private";
      await Promise.all([
        loadUser(),
        loadPlugins(
          creatingPlugin ? 1 : currentMarketPage,
          plugins.pageSize,
          true,
        ),
      ]);
      try {
        await offerPluginDependencyInstall(savedRow);
      } catch (error) {
        message.warning(
          `插件已保存，但依赖检测失败：${error instanceof Error ? error.message : "未知错误"}`,
        );
      }
    } catch (error) {
      message.error(error instanceof Error ? error.message : "保存插件失败");
    } finally {
      pluginEditor.saving = false;
    }
  }
  async function deleteMarketPluginEditor() {
    if (!pluginEditor.id || !pluginEditor.installed) return;
    pluginEditor.deleting = true;
    try {
      await post(
        `/api/admin/local-plugins/${encodeURIComponent(pluginEditor.id)}/deletions`,
      );
      message.success("插件已删除");
      pluginEditor.open = false;
      destroyPluginEditor();
      await Promise.all([loadUser(), loadPlugins(1, plugins.pageSize, true)]);
    } catch (error) {
      message.error(error instanceof Error ? error.message : "删除插件失败");
    } finally {
      pluginEditor.deleting = false;
    }
  }

  return {
    pluginEditor,
    pluginEditorHost,
    openNewMarketPluginEditor,
    openMarketPluginEditor,
    closeMarketPluginEditor,
    openNewScriptPluginEditor,
    pluginEditorLanguageOptions,
    onPluginEditorLanguageChange,
    handlePluginEditorOpenChange,
    syncPluginEditorLanguage,
    togglePluginEditorTheme,
    formatMarketPluginEditor,
    saveMarketPluginEditor,
    deleteMarketPluginEditor,
  };
}
