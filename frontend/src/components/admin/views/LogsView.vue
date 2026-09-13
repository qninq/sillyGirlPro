<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, nextTick, watch } from "vue";
import Button from "ant-design-vue/es/button";
import Input from "ant-design-vue/es/input";
import Segmented from "ant-design-vue/es/segmented";
import Space from "ant-design-vue/es/space";
import Switch from "ant-design-vue/es/switch";
import Typography from "ant-design-vue/es/typography";
import { Pause, Play, Trash2, ArrowDown, History, RefreshCw, Search, Send } from "lucide-vue-next";
import { useAdminViewContext } from "../adminViewContext";
import { getAuthToken } from "../../../api";

const { page } = useAdminViewContext();

const activeTab = ref<"logs" | "flow">("logs");

const logs = ref<string[]>([]);
const maxLogs = 2000;
const filterText = ref("");
const autoScroll = ref(true);
const paused = ref(false);
const connected = ref(false);
const logsContainer = ref<HTMLElement | null>(null);
const errorMsg = ref("");

let eventSource: EventSource | null = null;

type FlowEntry = {
  time: number;
  kind: string;
  platform: string;
  user: string;
  chat: string;
  content: string;
  handler?: string;
};

const flowEntries = ref<FlowEntry[]>([]);
const flowFilter = ref("");
const flowError = ref("");
let flowTimer: number | undefined;

function flowKindLabel(kind: string) {
  if (kind === "in") return "收到";
  if (kind === "match") return "命中";
  return "回复";
}

function flowKindClass(kind: string) {
  return `flow-kind-${kind}`;
}

function formatFlowTime(time: number) {
  if (!time) return "-";
  return new Date(time).toLocaleTimeString("zh-CN", { hour12: false });
}

function filteredFlowEntries() {
  const keyword = flowFilter.value.trim().toLowerCase();
  if (!keyword) return flowEntries.value;
  return flowEntries.value.filter((entry) =>
    [entry.platform, entry.user, entry.chat, entry.content, entry.handler]
      .filter(Boolean)
      .some((field) => String(field).toLowerCase().includes(keyword)),
  );
}

async function loadFlow() {
  try {
    const headers: Record<string, string> = {};
    const token = getAuthToken();
    if (token) headers.token = token;
    const res = await fetch("/api/admin/message-flow", { headers });
    const payload = await res.json();
    if (!payload || payload.status !== true) {
      throw new Error(payload?.message || "读取消息流水失败");
    }
    flowEntries.value = Array.isArray(payload.data) ? payload.data : [];
    flowError.value = "";
  } catch (error) {
    flowError.value = error instanceof Error ? error.message : "读取消息流水失败";
  }
}

function startFlowPolling() {
  if (flowTimer !== undefined) return;
  flowTimer = window.setInterval(() => {
    if (activeTab.value === "flow" && page.value === "logs") {
      void loadFlow();
    }
  }, 5000);
}

function stopFlowPolling() {
  if (flowTimer !== undefined) {
    window.clearInterval(flowTimer);
    flowTimer = undefined;
  }
}

watch(activeTab, (tab) => {
  if (tab === "flow") void loadFlow();
});

function filteredLogs() {
  const keyword = filterText.value.trim().toLowerCase();
  if (!keyword) return logs.value;
  return logs.value.filter((line) => line.toLowerCase().includes(keyword));
}

function scrollToBottom() {
  if (!autoScroll.value || paused.value) return;
  nextTick(() => {
    const el = logsContainer.value;
    if (el) el.scrollTop = el.scrollHeight;
  });
}

function clearLogs() {
  logs.value = [];
}

function togglePause() {
  paused.value = !paused.value;
}

function connect() {
  disconnect();
  errorMsg.value = "";
  const token = getAuthToken();
  const url = `/api/admin/logs/stream?lines=200&token=${encodeURIComponent(token)}`;
  eventSource = new EventSource(url);

  eventSource.onopen = () => {
    connected.value = true;
    errorMsg.value = "";
  };

  eventSource.onmessage = (event) => {
    if (paused.value) return;
    const line = event.data || "";
    // Skip keepalive comments
    if (line === "" || line.startsWith(": keepalive")) return;
    logs.value.push(line);
    if (logs.value.length > maxLogs) {
      logs.value.splice(0, logs.value.length - maxLogs);
    }
    scrollToBottom();
  };

  eventSource.onerror = () => {
    connected.value = false;
    errorMsg.value = "日志连接已断开，正在重连…";
    // EventSource will auto-reconnect
  };
}

function disconnect() {
  if (eventSource) {
    eventSource.close();
    eventSource = null;
  }
  connected.value = false;
}

watch(paused, (p) => {
  if (!p && !connected.value) {
    connect();
  }
});

onMounted(() => {
  connect();
  startFlowPolling();
});

onBeforeUnmount(() => {
  disconnect();
  stopFlowPolling();
});
</script>

<template>
  <section v-if="page === 'logs'" class="panel">
    <div class="logs-toolbar">
      <Space>
        <Segmented
          v-model:value="activeTab"
          :options="[
            { label: '实时日志', value: 'logs' },
            { label: '消息流水', value: 'flow' },
          ]"
        />
        <span v-if="activeTab === 'logs'" class="logs-status" :class="{ connected }">
          {{ connected ? "已连接" : "未连接" }}
        </span>
      </Space>
      <Space v-if="activeTab === 'logs'">
        <Input
          v-model:value="filterText"
          placeholder="过滤日志关键词"
          style="width: 200px"
          allow-clear
        />
        <span class="logs-toggle-label">自动滚动</span>
        <Switch v-model:checked="autoScroll" size="small" />
        <Button
          :type="paused ? 'primary' : 'default'"
          size="small"
          @click="togglePause"
        >
          <template #icon>
            <Pause v-if="!paused" :size="14" />
            <Play v-else :size="14" />
          </template>
          {{ paused ? "继续" : "暂停" }}
        </Button>
        <Button size="small" danger @click="clearLogs">
          <template #icon><Trash2 :size="14" /></template>
          清空
        </Button>
      </Space>
      <Space v-else>
        <Input
          v-model:value="flowFilter"
          placeholder="过滤平台 / 用户 / 内容"
          style="width: 220px"
          allow-clear
        >
          <template #prefix><Search :size="14" /></template>
        </Input>
        <Button size="small" @click="loadFlow">
          <template #icon><RefreshCw :size="14" /></template>
          刷新
        </Button>
      </Space>
    </div>

    <template v-if="activeTab === 'logs'">
      <div v-if="errorMsg" class="logs-error">{{ errorMsg }}</div>

      <div ref="logsContainer" class="logs-container">
        <div v-if="filteredLogs().length === 0" class="logs-empty">
          {{ filterText ? "无匹配的日志" : "暂无日志" }}
        </div>
        <div
          v-for="(line, index) in filteredLogs()"
          :key="index"
          class="log-line"
          :class="{
            'log-error': line.includes('[ERROR]') || line.includes('[E]'),
            'log-warn': (line.includes('[WARNING]') || line.includes('[W]')) && !line.includes('[ERROR]') && !line.includes('[E]'),
            'log-info': (line.includes('[INFO]') || line.includes('[I]') || line.includes('[N]')) && !line.includes('[WARNING]') && !line.includes('[W]') && !line.includes('[ERROR]') && !line.includes('[E]'),
            'log-debug': line.includes('[DEBUG]') || line.includes('[D]'),
          }"
        >
          <pre>{{ line }}</pre>
        </div>
        <div v-if="paused && !filterText" class="logs-paused-hint">
          <ArrowDown :size="14" /> 日志已暂停
        </div>
      </div>
    </template>

    <template v-else>
      <div v-if="flowError" class="logs-error">{{ flowError }}</div>
      <div class="flow-container">
        <div v-if="filteredFlowEntries().length === 0" class="logs-empty">
          {{ flowFilter ? "无匹配的流水记录" : "暂无消息流水，来一条消息试试" }}
        </div>
        <div
          v-for="(entry, index) in filteredFlowEntries()"
          :key="`${entry.time}-${index}`"
          class="flow-row"
        >
          <span class="flow-time">{{ formatFlowTime(entry.time) }}</span>
          <span class="flow-kind" :class="flowKindClass(entry.kind)">
            <History v-if="entry.kind === 'in'" :size="12" />
            <Search v-else-if="entry.kind === 'match'" :size="12" />
            <Send v-else :size="12" />
            {{ flowKindLabel(entry.kind) }}
          </span>
          <span class="flow-platform">{{ entry.platform }}</span>
          <span class="flow-target">{{
            entry.chat ? `${entry.user} @ ${entry.chat}` : entry.user
          }}</span>
          <span class="flow-content" :title="entry.content">{{
            entry.content
          }}</span>
          <span v-if="entry.handler" class="flow-handler">{{
            entry.handler
          }}</span>
        </div>
      </div>
      <div class="flow-footnote">
        每条消息记录「收到 → 命中插件 → 回复」三个环节，最多保留最近 200 条，重启后清空。
      </div>
    </template>
  </section>
</template>

<style scoped>
.logs-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 12px;
}

.logs-status {
  font-size: 12px;
  padding: 1px 8px;
  border-radius: 10px;
  background: #ff4d4f;
  color: #fff;
}
.logs-status.connected {
  background: #52c41a;
}

.logs-toggle-label {
  font-size: 13px;
  color: #666;
}

.logs-error {
  background: #fff2f0;
  border: 1px solid #ffccc7;
  border-radius: 6px;
  padding: 8px 12px;
  margin-bottom: 8px;
  color: #cf1322;
  font-size: 13px;
}

.logs-container {
  background: #1e1e1e;
  color: #d4d4d4;
  font-family: "Cascadia Code", "Fira Code", "Consolas", monospace;
  font-size: 13px;
  line-height: 1.6;
  border-radius: 8px;
  padding: 12px;
  height: calc(100vh - 220px);
  overflow-y: auto;
  white-space: pre-wrap;
  word-break: break-all;
}

.logs-container::-webkit-scrollbar {
  width: 6px;
}
.logs-container::-webkit-scrollbar-track {
  background: #2d2d2d;
}
.logs-container::-webkit-scrollbar-thumb {
  background: #555;
  border-radius: 3px;
}

.logs-empty {
  color: #888;
  text-align: center;
  padding: 40px 0;
  font-family: inherit;
}

.logs-paused-hint {
  position: sticky;
  bottom: 0;
  text-align: center;
  padding: 6px;
  background: rgba(30, 30, 30, 0.95);
  color: #888;
  font-size: 12px;
  font-family: inherit;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
}

.log-line {
  padding: 0;
  margin: 0;
}
.log-line pre {
  margin: 0;
  font-family: inherit;
  font-size: inherit;
  white-space: pre-wrap;
  word-break: break-all;
}

.log-error {
  color: #f44747;
}
.log-warn {
  color: #cca700;
}
.log-info {
  color: #6fc1ff;
}
.log-debug {
  color: #888;
}

/* 消息流水 */
.flow-container {
  border: 1px solid #e6ebf4;
  border-radius: 8px;
  background: #ffffff;
  height: calc(100vh - 250px);
  overflow-y: auto;
}
.flow-row {
  display: grid;
  grid-template-columns: 76px 64px 90px minmax(140px, 1.2fr) minmax(0, 2fr) minmax(90px, 0.8fr);
  gap: 10px;
  align-items: baseline;
  padding: 8px 12px;
  border-bottom: 1px solid #f1f5f9;
  font-size: 13px;
  color: #374151;
}
.flow-row:hover {
  background: #f8fafc;
}
.flow-time {
  color: #94a3b8;
  font-variant-numeric: tabular-nums;
}
.flow-kind {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  justify-self: start;
  padding: 1px 8px;
  border-radius: 999px;
  font-size: 12px;
}
.flow-kind-in {
  background: #e6f4ff;
  color: #1677ff;
}
.flow-kind-match {
  background: #f9f0ff;
  color: #722ed1;
}
.flow-kind-out {
  background: #f6ffed;
  color: #389e0d;
}
.flow-platform {
  color: #64748b;
}
.flow-target {
  color: #475569;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.flow-content {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: #1f2937;
}
.flow-handler {
  color: #722ed1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.flow-footnote {
  margin-top: 8px;
  color: #94a3b8;
  font-size: 12px;
}

@media (max-width: 640px) {
  .logs-toolbar {
    flex-direction: column;
    align-items: stretch;
  }
  .logs-toolbar .ant-space {
    flex-wrap: wrap;
    row-gap: 8px;
  }
  .logs-toolbar .ant-space-item {
    min-width: 0;
  }
  .logs-toggle-label {
    white-space: nowrap;
  }
}

@media (max-width: 720px) {
  .flow-row {
    grid-template-columns: 70px 56px 1fr;
    grid-auto-rows: auto;
    row-gap: 4px;
  }
  .flow-platform,
  .flow-target,
  .flow-content,
  .flow-handler {
    grid-column: 1 / -1;
    white-space: normal;
  }
}
</style>
