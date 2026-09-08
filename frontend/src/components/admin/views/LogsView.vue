<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, nextTick, watch } from "vue";
import Button from "ant-design-vue/es/button";
import Input from "ant-design-vue/es/input";
import Space from "ant-design-vue/es/space";
import Switch from "ant-design-vue/es/switch";
import Typography from "ant-design-vue/es/typography";
import { Pause, Play, Trash2, ArrowDown } from "lucide-vue-next";
import { useAdminViewContext } from "../adminViewContext";
import { getAuthToken } from "../../../api";

const { page } = useAdminViewContext();

const logs = ref<string[]>([]);
const maxLogs = 2000;
const filterText = ref("");
const autoScroll = ref(true);
const paused = ref(false);
const connected = ref(false);
const logsContainer = ref<HTMLElement | null>(null);
const errorMsg = ref("");

let eventSource: EventSource | null = null;

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
});

onBeforeUnmount(() => {
  disconnect();
});
</script>

<template>
  <section v-if="page === 'logs'" class="panel">
    <div class="logs-toolbar">
      <Space>
        <Typography.Text strong style="font-size: 16px">实时日志</Typography.Text>
        <span class="logs-status" :class="{ connected }">
          {{ connected ? "已连接" : "未连接" }}
        </span>
      </Space>
      <Space>
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
    </div>

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
</style>