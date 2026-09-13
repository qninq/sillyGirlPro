<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue";
import Badge from "ant-design-vue/es/badge";
import Empty from "ant-design-vue/es/empty";
import Popover from "ant-design-vue/es/popover";
import { BellRing } from "lucide-vue-next";
import { useAdminViewContext } from "./adminViewContext";
import { getAuthToken } from "../../api";

const { user } = useAdminViewContext();

type AlertEntry = {
  time: number;
  kind: string;
  title: string;
  content: string;
  delivered: number;
};

const SEEN_KEY = "sillygirl_alerts_seen";
const alerts = ref<AlertEntry[]>([]);
const open = ref(false);
let timer: number | undefined;

function kindLabel(kind: string) {
  if (kind === "adapter-offline") return "适配器掉线";
  if (kind === "plugin-crash") return "插件崩溃";
  return "系统告警";
}

function formatTime(time: number) {
  if (!time) return "-";
  return new Date(time * 1000).toLocaleString("zh-CN", { hour12: false });
}

async function loadAlerts() {
  try {
    const headers: Record<string, string> = {};
    const token = getAuthToken();
    if (token) headers.token = token;
    const res = await fetch("/api/admin/alerts", { headers });
    const payload = await res.json();
    if (payload && payload.status === true && Array.isArray(payload.data)) {
      alerts.value = payload.data;
    }
  } catch {
    // 静默：下一轮轮询重试。
  }
}

const unreadCount = computed(() => {
  const seen = Number(localStorage.getItem(SEEN_KEY) || 0);
  return alerts.value.filter((entry) => entry.time > seen).length;
});

function toggleOpen() {
  open.value = !open.value;
  if (open.value) {
    void loadAlerts();
    localStorage.setItem(SEEN_KEY, String(Math.floor(Date.now() / 1000)));
  }
}

function startPolling() {
  if (timer !== undefined || !user.value) return;
  void loadAlerts();
  timer = window.setInterval(() => void loadAlerts(), 60_000);
}

watch(user, (value) => {
  if (value) startPolling();
}, { immediate: true });

onBeforeUnmount(() => {
  if (timer !== undefined) {
    window.clearInterval(timer);
    timer = undefined;
  }
});
</script>

<template>
  <Popover
    v-model:open="open"
    trigger="click"
    placement="bottomRight"
    :width="380"
  >
    <template #content>
      <div class="alert-center-list">
        <div v-if="alerts.length === 0" class="alert-center-empty">
          <Empty
            :image="Empty.PRESENTED_IMAGE_SIMPLE"
            description="暂无告警，一切正常"
          />
        </div>
        <div
          v-for="entry in alerts"
          :key="`${entry.time}-${entry.title}`"
          class="alert-center-row"
        >
          <div class="alert-center-head">
            <span class="alert-center-title">{{ entry.title }}</span>
            <span class="alert-center-kind">{{ kindLabel(entry.kind) }}</span>
          </div>
          <div class="alert-center-content">{{ entry.content }}</div>
          <div class="alert-center-meta">
            {{ formatTime(entry.time) }} · 送达 {{ entry.delivered }} 个渠道
          </div>
        </div>
      </div>
    </template>
    <button
      type="button"
      class="alert-bell"
      :class="{ active: open }"
      title="告警中心"
      aria-label="告警中心"
      @click="toggleOpen"
    >
      <BellRing :size="17" />
      <Badge
        v-if="unreadCount"
        :count="unreadCount"
        :number-style="{ boxShadow: 'none' }"
      />
    </button>
  </Popover>
</template>

<style scoped>
.alert-bell {
  position: relative;
  display: grid;
  place-items: center;
  width: 34px;
  height: 34px;
  border: 1px solid #e6ebf4;
  border-radius: 10px;
  background: #ffffff;
  color: #475569;
  cursor: pointer;
}
.alert-bell.active,
.alert-bell:hover {
  color: #1677ff;
  border-color: #b8d3ff;
}
.alert-center-list {
  max-height: 360px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 340px;
}
.alert-center-empty {
  padding: 8px 0;
}
.alert-center-row {
  border: 1px solid #e6ebf4;
  border-radius: 10px;
  padding: 10px 12px;
}
.alert-center-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.alert-center-title {
  font-weight: 700;
  color: #b91c1c;
  font-size: 13px;
}
.alert-center-kind {
  font-size: 11px;
  padding: 1px 8px;
  border-radius: 999px;
  background: #fff1f0;
  color: #cf1322;
}
.alert-center-content {
  margin-top: 4px;
  color: #374151;
  font-size: 12.5px;
  line-height: 1.6;
}
.alert-center-meta {
  margin-top: 4px;
  color: #94a3b8;
  font-size: 11.5px;
}
</style>
