import { computed, nextTick, reactive, ref } from "vue";
import message from "ant-design-vue/es/message";
import { get, post } from "../../api";
import { apiData, type ApiEnvelope } from "./adminApi";
import type { Compartment, Extension } from "@codemirror/state";
import type { EditorView } from "@codemirror/view";

export type StorageEntryRow = {
  index?: string;
  bucket: string;
  key: string;
  value: string;
};

export type StorageGroupChild = {
  fullName: string;
  label: string;
};

export type StorageGroup = {
  key: string;
  leaf: boolean;
  children: StorageGroupChild[];
};

type StorageEditorRuntime = {
  Compartment: typeof import("@codemirror/state").Compartment;
  EditorState: typeof import("@codemirror/state").EditorState;
  EditorView: typeof import("@codemirror/view").EditorView;
  javascript: typeof import("@codemirror/lang-javascript").javascript;
  basicSetup: typeof import("codemirror").basicSetup;
};

const protectedStorageBuckets = new Set(["plugins", "sillyGirl", "auths"]);

function buildStorageGroups(names: string[]): StorageGroup[] {
  const map = new Map<string, StorageGroupChild[]>();
  for (const fullName of names) {
    const dot = fullName.indexOf(".");
    const key = dot === -1 ? fullName : fullName.slice(0, dot);
    const label = dot === -1 ? fullName : fullName.slice(dot + 1);
    const children = map.get(key) || [];
    children.push({ fullName, label });
    map.set(key, children);
  }
  const groups: StorageGroup[] = [];
  for (const [key, children] of map) {
    children.sort((a, b) => a.label.localeCompare(b.label));
    groups.push({
      key,
      leaf: children.length === 1 && children[0].label === key,
      children,
    });
  }
  groups.sort((a, b) => a.key.localeCompare(b.key));
  return groups;
}

function groupMatchesSearch(group: StorageGroup, filter: string): boolean {
  if (!filter) return true;
  const key = group.key.toLowerCase();
  if (key.includes(filter)) return true;
  return group.children.some(
    (child) =>
      child.fullName.toLowerCase().includes(filter) ||
      child.label.toLowerCase().includes(filter),
  );
}

function visibleGroupChildren(
  group: StorageGroup,
  filter: string,
): StorageGroupChild[] {
  if (!filter) return group.children;
  return group.children.filter(
    (child) =>
      child.fullName.toLowerCase().includes(filter) ||
      child.label.toLowerCase().includes(filter),
  );
}

export function useStorageAdmin() {
  const storageState = reactive({
    buckets: [] as string[],
    expanded: [] as string[],
    bucketSearch: "",
    selected: "",
    entrySearch: "",
    rows: [] as StorageEntryRow[],
    current: 1,
    pageSize: 20,
    total: 0,
    loading: false,
    loadingBuckets: false,
    savingEntry: false,
    deletingBucket: "",
    renaming: false,
    renameOpen: false,
    renameName: "",
    entryOpen: false,
    entryIsEdit: false,
    entryForm: { bucket: "", originalKey: "", key: "", value: "" },
    editorMode: "text" as "text" | "json",
  });

  const storageEditorHost = ref<HTMLDivElement | null>(null);
  let storageEditorRuntime: StorageEditorRuntime | null = null;
  let storageEditorRuntimePromise: Promise<StorageEditorRuntime> | null = null;
  let storageEditorView: EditorView | null = null;
  let storageEditorLanguage: Compartment | null = null;

  const storageGroups = computed<StorageGroup[]>(() => {
    const filter = storageState.bucketSearch.trim().toLowerCase();
    return buildStorageGroups(storageState.buckets).filter((group) =>
      groupMatchesSearch(group, filter),
    );
  });

  const isSearchingBuckets = computed(() => !!storageState.bucketSearch.trim());

  const canManageSelectedBucket = computed(
    () =>
      !!storageState.selected &&
      !protectedStorageBuckets.has(storageState.selected),
  );

  function isProtectedStorageBucket(name: string) {
    return protectedStorageBuckets.has(name);
  }

  function storageNodeOpen(key: string) {
    return storageState.expanded.includes(key);
  }

  async function loadStorageBuckets() {
    storageState.loadingBuckets = true;
    try {
      const res = await get<ApiEnvelope<Array<{ value: string }>>>(
        "/api/admin/storage/buckets",
      );
      const names = (apiData(res) || [])
        .map((item) => String(item.value ?? ""))
        .filter(Boolean);
      storageState.buckets = names;
      if (storageState.selected && !names.includes(storageState.selected)) {
        storageState.selected = "";
        storageState.rows = [];
        storageState.total = 0;
      }
    } finally {
      storageState.loadingBuckets = false;
    }
  }

  async function loadStorageEntries(
    page = storageState.current,
    pageSize = storageState.pageSize,
  ) {
    if (!storageState.selected) {
      storageState.rows = [];
      storageState.total = 0;
      return;
    }
    storageState.loading = true;
    try {
      const params = new URLSearchParams({
        bucket: storageState.selected,
        page: String(page),
        page_size: String(pageSize),
      });
      const search = storageState.entrySearch.trim();
      if (search) params.set("search", search);
      const res = await get<
        ApiEnvelope<{
          list: StorageEntryRow[];
          total: number;
          page?: number;
          page_size?: number;
        }>
      >(`/api/admin/storage/bucket-entries?${params.toString()}`);
      const data = apiData(res);
      storageState.rows = data?.list || [];
      storageState.current = data?.page || page;
      storageState.pageSize = data?.page_size || pageSize;
      storageState.total = data?.total || 0;
    } finally {
      storageState.loading = false;
    }
  }

  async function loadStorage() {
    await loadStorageBuckets();
    if (storageState.selected) await loadStorageEntries(1);
  }

  async function selectStorageBucket(name: string) {
    if (!name) return;
    storageState.selected = name;
    storageState.entrySearch = "";
    await loadStorageEntries(1);
  }

  function toggleStorageNode(key: string) {
    const expanded = new Set(storageState.expanded);
    if (expanded.has(key)) expanded.delete(key);
    else expanded.add(key);
    storageState.expanded = [...expanded];
  }

  function applyEntrySearch() {
    return loadStorageEntries(1);
  }

  function changeEntryPage(pagination: {
    current?: number;
    pageSize?: number;
  }) {
    return loadStorageEntries(
      pagination.current || 1,
      pagination.pageSize || storageState.pageSize,
    );
  }

  function writeStorageEntry(bucket: string, key: string, value: string) {
    return post("/api/admin/storage/bucket-entries", { bucket, key, value });
  }

  async function loadStorageEditorRuntime() {
    if (storageEditorRuntime) return storageEditorRuntime;
    if (!storageEditorRuntimePromise) {
      storageEditorRuntimePromise = Promise.all([
        import("@codemirror/state"),
        import("@codemirror/view"),
        import("@codemirror/lang-javascript"),
        import("codemirror"),
      ]).then(([state, view, javascriptLanguage, codemirror]) => ({
        Compartment: state.Compartment,
        EditorState: state.EditorState,
        EditorView: view.EditorView,
        javascript: javascriptLanguage.javascript,
        basicSetup: codemirror.basicSetup,
      }));
    }
    storageEditorRuntime = await storageEditorRuntimePromise;
    return storageEditorRuntime;
  }

function storageEditorExtensions(runtime: StorageEditorRuntime): Extension[] {
  return [
    runtime.basicSetup,
    // 长文本值自动换行，避免单行横向滚动难以查看。
    runtime.EditorView.lineWrapping,
    storageEditorLanguage!.of(
      storageState.editorMode === "json" ? runtime.javascript() : [],
    ),
    runtime.EditorView.updateListener.of((update) => {
      if (update.docChanged) {
        storageState.entryForm.value = update.state.doc.toString();
      }
    }),
  ];
}

  async function initStorageEditor() {
    const host = storageEditorHost.value;
    if (!host) return;
    destroyStorageEditor();
    const runtime = await loadStorageEditorRuntime();
    if (!storageEditorHost.value) return;
    if (!storageEditorLanguage) {
      storageEditorLanguage = new runtime.Compartment();
    }
    storageEditorView = new runtime.EditorView({
      parent: storageEditorHost.value,
      state: runtime.EditorState.create({
        doc: storageState.entryForm.value,
        extensions: storageEditorExtensions(runtime),
      }),
    });
  }

  function destroyStorageEditor() {
    storageEditorView?.destroy();
    storageEditorView = null;
  }

  function syncStorageEditorDoc(value: string) {
    if (!storageEditorView) return;
    const current = storageEditorView.state.doc.toString();
    if (current === value) return;
    storageEditorView.dispatch({
      changes: { from: 0, to: current.length, insert: value },
    });
  }

  function setStorageEditorMode(mode: "text" | "json") {
    storageState.editorMode = mode;
    if (!storageEditorView || !storageEditorLanguage || !storageEditorRuntime) {
      return;
    }
    storageEditorView.dispatch({
      effects: storageEditorLanguage.reconfigure(
        mode === "json" ? storageEditorRuntime.javascript() : [],
      ),
    });
  }

  function formatStorageJson() {
    if (storageState.editorMode !== "json") return;
    const raw = storageState.entryForm.value ?? "";
    const payload = typedJsonPayloadOf(raw);
    let parsed: unknown;
    try {
      parsed = JSON.parse(payload ?? (raw.length ? raw : "null"));
    } catch (error) {
      const reason = error instanceof Error ? error.message : String(error);
      message.error(`JSON 格式错误：${reason.slice(0, 160)}`);
      return;
    }
    const pretty = JSON.stringify(parsed, null, 2);
    storageState.entryForm.value = payload === null ? pretty : "o:" + pretty;
    syncStorageEditorDoc(storageState.entryForm.value);
    setStorageEditorMode("json");
  }

  function handleStorageEditorKeydown(event: KeyboardEvent) {
    if (
      storageState.editorMode === "json" &&
      event.shiftKey &&
      event.altKey &&
      event.code === "KeyF"
    ) {
      event.preventDefault();
      formatStorageJson();
    }
  }

  // 后端桶值类型标签（core/bucket.go encodeBucketValue）：对象值以 "o:" + JSON 存储。
  // JSON 校验与格式化只针对标签后的载荷；只认对象/数组，避免误伤恰好以 o: 开头的普通字符串。
  // 返回 null 表示内容整体才是 JSON（或不是 JSON）。
  function typedJsonPayloadOf(value: string): string | null {
    const raw = (value || "").trim();
    if (!raw.startsWith("o:")) return null;
    const payload = raw.slice(2).trim();
    if (!payload.startsWith("{") && !payload.startsWith("[")) return null;
    return payload;
  }

  function detectStorageEditorMode(value: string): "text" | "json" {
    const text = (value || "").trim();
    if (!text) return "text";
    const payload = typedJsonPayloadOf(text) ?? text;
    try {
      JSON.parse(payload);
      return "json";
    } catch {
      return "text";
    }
  }

  function openAddEntry() {
    storageState.entryForm = {
      bucket: storageState.selected,
      originalKey: "",
      key: "",
      value: "",
    };
    storageState.entryIsEdit = false;
    storageState.editorMode = "text";
    storageState.entryOpen = true;
    void nextTick(initStorageEditor);
  }

  function openEditEntry(row: StorageEntryRow) {
    const rawValue = row.value ?? "";
    const mode = detectStorageEditorMode(rawValue);
    let value = rawValue;
    if (mode === "json") {
      const payload = typedJsonPayloadOf(rawValue);
      try {
        const pretty = JSON.stringify(JSON.parse(payload ?? rawValue), null, 2);
        // o: 类型标签保留在编辑内容前，保存时原样带回，存储值结构不变。
        value = payload === null ? pretty : "o:" + pretty;
      } catch {
        value = rawValue;
      }
    }
    storageState.entryForm = {
      bucket: row.bucket,
      originalKey: row.key,
      key: row.key,
      value,
    };
    storageState.entryIsEdit = true;
    storageState.editorMode = mode;
    storageState.entryOpen = true;
    void nextTick(initStorageEditor);
  }

  async function submitEntry() {
    const bucket = storageState.entryForm.bucket.trim();
    const key = storageState.entryForm.key.trim();
    if (!bucket) {
      message.error("请输入 Bucket");
      return;
    }
    if (!key) {
      message.error("请输入 KEY");
      return;
    }
    let value = storageState.entryForm.value ?? "";
    if (storageState.editorMode === "json") {
      const payload = typedJsonPayloadOf(value);
      let parsed: unknown;
      try {
        parsed = JSON.parse(payload ?? (value.length ? value : "null"));
      } catch (error) {
        const reason = error instanceof Error ? error.message : String(error);
        message.error(`VALUE 不是有效的 JSON：${reason.slice(0, 160)}`);
        return;
      }
      // 展示用格式化排版，保存统一压缩为单行，保持存储紧凑；o: 类型标签原样保留。
      const compact = JSON.stringify(parsed);
      value = payload === null ? compact : "o:" + compact;
    }
    storageState.savingEntry = true;
    try {
      await writeStorageEntry(bucket, key, value);
      if (storageState.entryIsEdit && key !== storageState.entryForm.originalKey) {
        await writeStorageEntry(bucket, storageState.entryForm.originalKey, "");
      }
      message.success("已保存");
      storageState.entryOpen = false;
      destroyStorageEditor();
      storageState.selected = bucket;
      await loadStorageBuckets();
      await loadStorageEntries(storageState.current);
    } catch (error) {
      message.error(error instanceof Error ? error.message : "保存失败");
    } finally {
      storageState.savingEntry = false;
    }
  }

  async function deleteStorageEntry(row: StorageEntryRow) {
    try {
      await writeStorageEntry(row.bucket, row.key, "");
      message.success("已删除");
      await loadStorageEntries(storageState.current);
    } catch (error) {
      message.error(error instanceof Error ? error.message : "删除失败");
    }
  }

  function openRenameBucket() {
    storageState.renameName = storageState.selected;
    storageState.renameOpen = true;
  }

  async function submitRenameBucket() {
    const from = storageState.selected;
    const to = storageState.renameName.trim();
    if (!from) return;
    if (!to) {
      message.error("请输入新的存储桶名称");
      return;
    }
    storageState.renaming = true;
    try {
      await post(
        `/api/admin/storage/buckets/${encodeURIComponent(from)}/renames`,
        { name: to },
      );
      message.success("存储桶已改名");
      storageState.renameOpen = false;
      storageState.selected = to;
      await loadStorageBuckets();
      await loadStorageEntries(1);
    } catch (error) {
      message.error(error instanceof Error ? error.message : "存储桶改名失败");
    } finally {
      storageState.renaming = false;
    }
  }

  async function removeStorageBucket(name: string) {
    if (!name) return;
    storageState.deletingBucket = name;
    try {
      await post(
        `/api/admin/storage/buckets/${encodeURIComponent(name)}/deletions`,
      );
      message.success("存储桶已删除");
      if (storageState.selected === name) {
        storageState.selected = "";
        storageState.rows = [];
        storageState.total = 0;
      }
      await loadStorageBuckets();
    } catch (error) {
      message.error(error instanceof Error ? error.message : "存储桶删除失败");
    } finally {
      storageState.deletingBucket = "";
    }
  }

  return {
    storageState,
    storageEditorHost,
    storageGroups,
    isSearchingBuckets,
    canManageSelectedBucket,
    isProtectedStorageBucket,
    storageNodeOpen,
    loadStorage,
    selectStorageBucket,
    toggleStorageNode,
    applyEntrySearch,
    changeEntryPage,
    openAddEntry,
    openEditEntry,
    submitEntry,
    deleteStorageEntry,
    destroyStorageEditor,
    setStorageEditorMode,
    formatStorageJson,
    handleStorageEditorKeydown,
    openRenameBucket,
    submitRenameBucket,
    removeStorageBucket,
  };
}
