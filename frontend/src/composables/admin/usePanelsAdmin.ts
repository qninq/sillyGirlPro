import { computed, reactive, type Ref } from "vue";
import message from "ant-design-vue/es/message";
import { get, post } from "../../api";
import type { DaidaiPanel, QinglongPanel } from "../../types";
import { apiData, type ApiEnvelope } from "./adminApi";

type ContainerKind = "qinglong" | "daidai";

export type AdminPanelsResponse = {
  qinglong: { list: QinglongPanel[]; total: number };
  daidai: { list: DaidaiPanel[]; total: number };
};

export function usePanelsAdmin(containerKind: Ref<ContainerKind>) {
  let panelsLoaded = false;
  let panelsRequest: Promise<void> | null = null;

  const qinglong = reactive({
    rows: [] as QinglongPanel[],
    total: 0,
    loading: false,
    editing: null as QinglongPanel | null,
    form: {} as QinglongPanel,
    testing: false,
    saving: false,
  });
  async function loadQinglongPanels(force = false) {
    await loadAdminPanels({ force });
  }
  function openQinglongPanel(row?: QinglongPanel) {
    const data = row || {
      name: "",
      address: "",
      client_id: "",
      client_secret: "",
    };
    qinglong.editing = data;
    qinglong.form = { ...data };
  }
  async function testQinglongPanel(panel = qinglong.form) {
    qinglong.testing = true;
    try {
      await post("/api/admin/panel-connection-tests", {
        ...panel,
        type: "qinglong",
      });
      message.success("青龙接口连接成功");
    } catch (error) {
      message.error(
        error instanceof Error ? error.message : "青龙接口连接失败",
      );
    } finally {
      qinglong.testing = false;
    }
  }
  async function saveQinglongPanel() {
    qinglong.saving = true;
    try {
      const payload = { ...qinglong.form, type: "qinglong" };
      if (qinglong.form.id) {
        await post(
          `/api/admin/panels/${encodeURIComponent(qinglong.form.id)}`,
          payload,
        );
      } else {
        await post("/api/admin/panels", payload);
      }
      qinglong.editing = null;
      message.success("青龙面板已添加");
      await loadQinglongPanels(true);
    } catch (error) {
      message.error(
        error instanceof Error ? error.message : "青龙面板添加失败",
      );
    } finally {
      qinglong.saving = false;
    }
  }
  async function removeQinglongPanel(row: QinglongPanel) {
    await post(`/api/admin/panels/${encodeURIComponent(row.id || "")}/deletions`);
    message.success("已删除");
    void loadQinglongPanels(true);
  }

  const daidai = reactive({
    rows: [] as DaidaiPanel[],
    total: 0,
    loading: false,
    editing: null as DaidaiPanel | null,
    form: {} as DaidaiPanel,
    testing: false,
    saving: false,
  });
  async function loadDaidaiPanels(force = false) {
    await loadAdminPanels({ force });
  }

  function applyAdminPanels(data?: AdminPanelsResponse) {
    qinglong.rows = data?.qinglong?.list || [];
    qinglong.total = data?.qinglong?.total || 0;
    daidai.rows = data?.daidai?.list || [];
    daidai.total = data?.daidai?.total || 0;
  }

  async function loadAdminPanels(
    options: { force?: boolean; refreshStatus?: boolean } = {},
  ) {
    const force = options.force === true;
    const refreshStatus = options.refreshStatus === true;
    if (!force && panelsLoaded) return;
    if (panelsRequest) return panelsRequest;

    qinglong.loading = true;
    daidai.loading = true;
    const request = (async () => {
      const endpoint = refreshStatus
        ? "/api/admin/panel-status-checks"
        : "/api/admin/panels";
      const res = refreshStatus
        ? await post<ApiEnvelope<AdminPanelsResponse>>(endpoint, {})
        : await get<ApiEnvelope<AdminPanelsResponse>>(endpoint);
      applyAdminPanels(apiData(res));
      panelsLoaded = true;
    })();
    panelsRequest = request;
    try {
      await request;
    } finally {
      if (panelsRequest === request) panelsRequest = null;
      qinglong.loading = false;
      daidai.loading = false;
    }
  }

  function openDaidaiPanel(row?: DaidaiPanel) {
    const data = row || { name: "", address: "", app_key: "", app_secret: "" };
    daidai.editing = data;
    daidai.form = { ...data };
  }
  async function testDaidaiPanel(panel = daidai.form) {
    daidai.testing = true;
    try {
      await post("/api/admin/panel-connection-tests", {
        ...panel,
        type: "daidai",
      });
      message.success("呆呆面板连接成功");
    } catch (error) {
      message.error(
        error instanceof Error ? error.message : "呆呆面板连接失败",
      );
    } finally {
      daidai.testing = false;
    }
  }
  async function saveDaidaiPanel() {
    daidai.saving = true;
    try {
      const payload = { ...daidai.form, type: "daidai" };
      if (daidai.form.id) {
        await post(
          `/api/admin/panels/${encodeURIComponent(daidai.form.id)}`,
          payload,
        );
      } else {
        await post("/api/admin/panels", payload);
      }
      daidai.editing = null;
      message.success("呆呆面板已添加");
      await loadDaidaiPanels(true);
    } catch (error) {
      message.error(
        error instanceof Error ? error.message : "呆呆面板添加失败",
      );
    } finally {
      daidai.saving = false;
    }
  }
  async function removeDaidaiPanel(row: DaidaiPanel) {
    await post(`/api/admin/panels/${encodeURIComponent(row.id || "")}/deletions`);
    message.success("已删除");
    void loadDaidaiPanels(true);
  }

  const containerOptions = [
    { label: "青龙", value: "qinglong" },
    { label: "呆呆", value: "daidai" },
  ] as { label: string; value: ContainerKind }[];
  const containerHelpText = computed(() => {
    if (containerKind.value === "qinglong")
      return "保存前会检测 /open/auth/token 是否可用。";
    return "保存前会调用 /api/open-api/token，使用 app_key/app_secret 验证 Open API。";
  });
  const containerAddLabel = computed(() => {
    if (containerKind.value === "qinglong") return "添加青龙面板";
    return "添加呆呆面板";
  });

  function loadActiveContainerPanels() {
    return loadAdminPanels();
  }

  function refreshActiveContainerPanels() {
    return loadAdminPanels({ force: true, refreshStatus: true });
  }

  function openActiveContainerPanel() {
    if (containerKind.value === "qinglong") {
      openQinglongPanel();
      return;
    }
    openDaidaiPanel();
  }

  return {
    qinglong,
    loadQinglongPanels,
    openQinglongPanel,
    testQinglongPanel,
    saveQinglongPanel,
    removeQinglongPanel,
    daidai,
    loadDaidaiPanels,
    applyAdminPanels,
    loadAdminPanels,
    openDaidaiPanel,
    testDaidaiPanel,
    saveDaidaiPanel,
    removeDaidaiPanel,
    containerOptions,
    containerHelpText,
    containerAddLabel,
    loadActiveContainerPanels,
    refreshActiveContainerPanels,
    openActiveContainerPanel,
  };
}
