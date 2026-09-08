import { reactive } from "vue";
import message from "ant-design-vue/es/message";
import { get, post } from "../../api";
import type { CarryGroup } from "../../types";
import { apiData, type ApiEnvelope } from "./adminApi";

export function useCarryAdmin() {
  const carry = reactive({
    rows: [] as CarryGroup[],
    total: 0,
    editing: null as CarryGroup | null,
    form: {} as any,
    selects: {} as any,
  });
  async function loadCarry(current = 1, pageSize = 20) {
    const res = await get<ApiEnvelope<{ list: CarryGroup[]; total: number }>>(
      `/api/admin/carry-groups?page=${current}&page_size=${pageSize}`,
    );
    const data = apiData(res);
    carry.rows = data?.list || [];
    carry.total = data?.total || 0;
  }
  async function loadCarrySelects(row?: CarryGroup) {
    const res = await get<ApiEnvelope<any>>(
      `/api/admin/carry-group-options?chat_id=${encodeURIComponent(row?.chat_id || "")}&platform=${encodeURIComponent(row?.platform || "")}`,
    );
    carry.selects = apiData(res) || {};
  }
  async function changeCarryPlatform(platform: string) {
    carry.form.platform = platform;
    carry.form.bots_id = [];
    await loadCarrySelects({ ...(carry.form as CarryGroup), platform });
  }
  async function openCarry(row?: CarryGroup) {
    const data = row || {
      chat_id: "",
      platform: "",
      remark: "",
      bots_id: [],
      targets: [],
    };
    carry.editing = data;
    await loadCarrySelects(data);
    carry.form = {
      ...data,
      bots_id: data.bots_id || [],
      targets: (data.targets || []).map((item) => ({
        platform: item.platform || "",
        chat_id: item.chat_id || "",
        type: item.type === "private" ? "private" : "group",
      })),
    };
  }
  async function saveCarry() {
    if (!carry.form.chat_id?.trim()) {
      message.error("请输入群号");
      return;
    }
    if (!carry.form.platform) {
      message.error("请选择平台");
      return;
    }
    const targets = (carry.form.targets || [])
      .map((item: any) => ({
        platform: String(item?.platform || "").trim(),
        chat_id: String(item?.chat_id || "").trim(),
        type: item?.type === "private" ? "private" : "group",
      }))
      .filter((item: any) => item.platform && item.chat_id);
    if (!targets.length) {
      message.error("请至少配置一个转发目标");
      return;
    }
    const payload = {
      chat_id: carry.form.chat_id.trim(),
      platform: carry.form.platform,
      remark: carry.form.remark || "",
      bots_id: carry.form.bots_id || [],
      targets,
    };
    if (carry.editing?.chat_id) {
      await post(
        `/api/admin/carry-groups/${encodeURIComponent(carry.editing.chat_id)}`,
        payload,
      );
    } else {
      await post("/api/admin/carry-groups", payload);
    }
    carry.editing = null;
    message.success("已保存");
    loadCarry();
  }
  async function toggleCarry(row: CarryGroup, enable: boolean) {
    await post(
      `/api/admin/carry-groups/${encodeURIComponent(row.chat_id)}`,
      { platform: row.platform || "", enable },
    );
    message.success(enable ? "已启用" : "已停用");
    loadCarry();
  }
  async function removeCarry(row: CarryGroup) {
    await post(`/api/admin/carry-groups/${encodeURIComponent(row.chat_id)}/deletions`);
    message.success("已删除");
    loadCarry();
  }

  return {
    carry,
    loadCarry,
    loadCarrySelects,
    changeCarryPlatform,
    openCarry,
    saveCarry,
    toggleCarry,
    removeCarry,
  };
}
