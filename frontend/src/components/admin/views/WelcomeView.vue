<script setup lang="ts">
import Alert from "ant-design-vue/es/alert";
import Button from "ant-design-vue/es/button";
import Card from "ant-design-vue/es/card";
import { CloudDownload, Server } from "lucide-vue-next";
import Col from "ant-design-vue/es/col";
import Modal from "ant-design-vue/es/modal";
import Progress from "ant-design-vue/es/progress";
import Row from "ant-design-vue/es/row";
import Space from "ant-design-vue/es/space";
import Statistic from "ant-design-vue/es/statistic";
import Tag from "ant-design-vue/es/tag";
import Typography from "ant-design-vue/es/typography";
import message from "ant-design-vue/es/message";
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useAdminViewContext } from "../adminViewContext";
import { getAuthToken } from "../../../api";

const {
  daidai,
  overviewIntegrations,
  overviewUserStats,
  overviewVersion,
  page,
  qinglong,
  realScripts,
  restartAfterUpdate,
  startOnlineUpdate,
  systemUpdate,
  user,
} = useAdminViewContext();

type StatsDay = { date: string; total: number; platforms: Record<string, number> };

const messageStats = ref<StatsDay[]>([]);
let statsTimer: number | undefined;

async function loadMessageStats() {
  try {
    const headers: Record<string, string> = {};
    const token = getAuthToken();
    if (token) headers.token = token;
    const res = await fetch("/api/admin/message-stats?days=14", { headers });
    const payload = await res.json();
    if (payload && payload.status === true && Array.isArray(payload.data)) {
      messageStats.value = payload.data;
    }
  } catch {
    // 静默：下一轮刷新重试。
  }
}

const maxStatsTotal = computed(() =>
  Math.max(1, ...messageStats.value.map((day) => day.total)),
);

const platformTotals = computed(() => {
  const totals = new Map<string, number>();
  for (const day of messageStats.value) {
    for (const [platform, count] of Object.entries(day.platforms || {})) {
      totals.set(platform, (totals.get(platform) || 0) + count);
    }
  }
  return [...totals.entries()]
    .map(([name, total]) => ({ name, total }))
    .sort((a, b) => b.total - a.total)
    .slice(0, 6);
});

function startStatsPolling() {
  if (statsTimer !== undefined) return;
  void loadMessageStats();
  statsTimer = window.setInterval(() => {
    if (page.value === "welcome") void loadMessageStats();
  }, 60_000);
}

function stopStatsPolling() {
  if (statsTimer !== undefined) {
    window.clearInterval(statsTimer);
    statsTimer = undefined;
  }
}

// ---- 系统信息 ----

type SystemInfo = {
  now: number;
  boot_at: number;
  uptime: number;
  cpu_percent: number;
  memory_used: number;
  memory_total: number;
  memory_percent: number;
  cpu_history: number[];
  memory_history: number[];
  version: string;
  go_version: string;
  os: string;
  arch: string;
  pid: number;
  port: number;
  storage_backend: string;
  data_dir: string;
};

const systemInfo = ref<SystemInfo | null>(null);
const systemClock = ref(Date.now());
let systemInfoTimer: number | undefined;
let clockTimer: number | undefined;

async function loadSystemInfo() {
  try {
    const headers: Record<string, string> = {};
    const token = getAuthToken();
    if (token) headers.token = token;
    const res = await fetch("/api/admin/system-info", { headers });
    const payload = await res.json();
    if (payload && payload.status === true && payload.data) {
      systemInfo.value = payload.data;
    }
  } catch {
    // 静默：下一轮轮询重试。
  }
}

const weekdayNames = ["周日", "周一", "周二", "周三", "周四", "周五", "周六"];

const systemNowText = computed(() => {
  const date = new Date(systemClock.value);
  return `${date.toLocaleDateString("zh-CN")} ${date.toLocaleTimeString("zh-CN", { hour12: false })} ${weekdayNames[date.getDay()]}`;
});

function formatUptime(seconds: number) {
  if (!seconds || seconds < 0) return "-";
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  if (days > 0) return `${days} 天 ${hours} 小时 ${minutes} 分`;
  if (hours > 0) return `${hours} 小时 ${minutes} 分`;
  return `${minutes} 分钟`;
}

function formatGigabytes(bytes: number) {
  if (!bytes) return "-";
  return `${(bytes / 1024 ** 3).toFixed(1)}G`;
}

function sparkPoints(values: number[]) {
  const width = 110;
  const height = 30;
  const pad = 2;
  if (!values.length) return "";
  const points = values.map((value, index) => {
    const x = values.length === 1 ? width : (index / (values.length - 1)) * width;
    const clamped = Math.min(100, Math.max(0, value));
    const y = height - pad - (clamped / 100) * (height - pad * 2);
    return `${x.toFixed(1)},${y.toFixed(1)}`;
  });
  return points.join(" ");
}

function sparkArea(values: number[]) {
  const points = sparkPoints(values);
  if (!points) return "";
  return `0,30 ${points} 110,30`;
}

function startSystemInfoPolling() {
  if (systemInfoTimer !== undefined) return;
  void loadSystemInfo();
  systemInfoTimer = window.setInterval(() => {
    if (page.value === "welcome") void loadSystemInfo();
  }, 5000);
}

function stopSystemInfoPolling() {
  if (systemInfoTimer !== undefined) {
    window.clearInterval(systemInfoTimer);
    systemInfoTimer = undefined;
  }
}

const platformMaxTotal = computed(() =>
  Math.max(1, ...platformTotals.value.map((item) => item.total)),
);

function statsBarHeight(total: number) {
  return `${Math.max(4, Math.round((total / platformMaxTotal.value) * 120))}px`;
}

watch(
  () => page.value,
  (value) => {
    if (value === "welcome") {
      void loadMessageStats();
      startStatsPolling();
      void loadSystemInfo();
      startSystemInfoPolling();
      if (clockTimer === undefined) {
        clockTimer = window.setInterval(() => {
          systemClock.value = Date.now();
        }, 1000);
      }
    }
  },
  { immediate: true },
);

onBeforeUnmount(() => {
  stopStatsPolling();
  stopSystemInfoPolling();
  if (clockTimer !== undefined) window.clearInterval(clockTimer);
});
</script>

<template>
  <section v-if="page === 'welcome'" class="panel">
    <Typography.Title :level="3" style="margin-top: 0">{{
      user?.name || "傻妞"
    }}</Typography.Title>
    <Space wrap style="margin-bottom: 14px">
      <Tag color="blue">当前版本 {{ overviewVersion.local }}</Tag>
      <Tag color="green">最新版本 {{ overviewVersion.remote }}</Tag>
      <Typography.Link :href="overviewVersion.repository" target="_blank"
        >GitHub</Typography.Link
      >
      <Button
        type="primary"
        size="small"
        :loading="systemUpdate.running"
        @click="startOnlineUpdate"
      >
        <template #icon><CloudDownload :size="15" /></template>
        在线更新
      </Button>
    </Space>
    <Row :gutter="[12, 12]">
      <Col :xs="12" :sm="12" :md="8"
        ><Card><Statistic title="脚本数量" :value="realScripts.length" /></Card
      ></Col>
      <Col :xs="12" :sm="12" :md="8"
        ><Card
          ><Statistic
            title="今日新增用户"
            :value="overviewUserStats.today" /></Card
      ></Col>
      <Col :xs="12" :sm="12" :md="8"
        ><Card
          ><Statistic
            title="总用户数量"
            :value="overviewUserStats.total" /></Card
      ></Col>
      <Col :xs="12" :sm="12" :md="8"
        ><Card
          ><Statistic
            title="青龙容器"
            :value="
              overviewIntegrations.find((item) => item.key === 'qinglong')
                ?.count || 0
            " /></Card
      ></Col>
      <Col :xs="12" :sm="12" :md="8"
        ><Card
          ><Statistic
            title="呆呆容器"
            :value="
              overviewIntegrations.find((item) => item.key === 'daidai')
                ?.count || 0
            " /></Card
      ></Col>
    </Row>

    <Row :gutter="[12, 12]" style="margin-top: 12px">
      <Col :xs="24" :md="12">
    <Card style="height: 100%">
      <template #title>
        <span>消息概览</span>
      </template>
      <template v-if="platformTotals.length">
        <div class="stats-chart">
          <div
            v-for="item in platformTotals"
            :key="item.name"
            class="stats-bar-col"
            :title="`${item.name}：共 ${item.total} 条`"
          >
            <span class="stats-bar-value">{{
              item.total || ""
            }}</span>
            <div class="stats-bar-wrap">
              <div
                class="stats-bar"
                :style="{ height: statsBarHeight(item.total) }"
              ></div>
            </div>
            <span class="stats-bar-label">{{ item.name }}</span>
          </div>
        </div>
      </template>
      <Typography.Text v-else class="muted"
        >暂无消息数据，私聊机器人一条消息后再来看。</Typography.Text
      >
    </Card>
      </Col>
      <Col :xs="24" :md="12">
    <Card style="height: 100%">
      <template #title>
        <Space wrap size="small">
          <Space size="small"
            ><Server :size="15" /><span>系统信息</span></Space
          >
          <Typography.Text class="muted" style="font-weight: 400">{{
            systemNowText
          }}</Typography.Text>
        </Space>
      </template>
      <div v-if="systemInfo" class="system-info-grid">
        <div class="system-info-item">
          <span>运行时长</span>
          <strong>{{ formatUptime(systemInfo.uptime) }}</strong>
        </div>
        <div class="system-info-item">
          <span>启动时间</span>
          <strong>{{
            systemInfo.boot_at
              ? new Date(systemInfo.boot_at * 1000).toLocaleString("zh-CN", {
                  hour12: false,
                })
              : "-"
          }}</strong>
        </div>
        <div class="system-info-item">
          <span>CPU 占用</span>
          <div class="system-info-value">
            <svg
              class="spark-line"
              width="110"
              height="30"
              viewBox="0 0 110 30"
              aria-hidden="true"
            >
              <polygon
                class="spark-area"
                :points="sparkArea(systemInfo.cpu_history)"
              ></polygon>
              <polyline
                class="spark-stroke"
                :points="sparkPoints(systemInfo.cpu_history)"
                fill="none"
                vector-effect="non-scaling-stroke"
              ></polyline>
            </svg>
            <strong>{{
              systemInfo.cpu_percent >= 0
                ? `${systemInfo.cpu_percent.toFixed(1)}%`
                : "-"
            }}</strong>
          </div>
        </div>
        <div class="system-info-item">
          <span>内存占用</span>
          <div class="system-info-value">
            <svg
              class="spark-line"
              width="110"
              height="30"
              viewBox="0 0 110 30"
              aria-hidden="true"
            >
              <polygon
                class="spark-area"
                :points="sparkArea(systemInfo.memory_history)"
              ></polygon>
              <polyline
                class="spark-stroke"
                :points="sparkPoints(systemInfo.memory_history)"
                fill="none"
                vector-effect="non-scaling-stroke"
              ></polyline>
            </svg>
            <strong>{{
              systemInfo.memory_total
                ? `${formatGigabytes(systemInfo.memory_used)}/${formatGigabytes(systemInfo.memory_total)}`
                : "-"
            }}</strong>
          </div>
        </div>
        <div class="system-info-item">
          <span>主程序版本</span>
          <strong>{{ systemInfo.version }}</strong>
        </div>
        <div class="system-info-item">
          <span>Go 版本</span>
          <strong>{{ systemInfo.go_version }}</strong>
        </div>
        <div class="system-info-item">
          <span>平台架构</span>
          <strong>{{ systemInfo.os }} / {{ systemInfo.arch }}</strong>
        </div>
        <div class="system-info-item">
          <span>进程 PID</span>
          <strong>{{ systemInfo.pid }}</strong>
        </div>
        <div class="system-info-item">
          <span>面板端口</span>
          <strong>{{ systemInfo.port }}</strong>
        </div>
        <div class="system-info-item">
          <span>存储后端</span>
          <strong>{{ systemInfo.storage_backend }}</strong>
        </div>
        <div class="system-info-item system-info-item-wide">
          <span>数据目录</span>
          <strong class="mono">{{ systemInfo.data_dir }}</strong>
        </div>
      </div>
      <Typography.Text v-else class="muted">正在读取系统信息…</Typography.Text>
    </Card>
      </Col>
    </Row>
  </section>

  <Modal
    v-model:open="systemUpdate.open"
    title="在线更新"
    :footer="null"
    :closable="!systemUpdate.running && !systemUpdate.restartChecking"
    :mask-closable="!systemUpdate.running && !systemUpdate.restartChecking"
  >
    <Space direction="vertical" style="width: 100%" size="middle">
      <Progress
        :percent="systemUpdate.percent"
        :status="
          systemUpdate.status === 'error'
            ? 'exception'
            : systemUpdate.status === 'done'
              ? 'success'
              : 'active'
        "
      />
      <Alert
        :type="
          systemUpdate.status === 'error'
            ? 'error'
            : systemUpdate.status === 'done'
              ? 'success'
              : 'info'
        "
        :message="systemUpdate.message || '准备更新'"
        show-icon
      />
      <div v-if="systemUpdate.result" class="update-result">
        <Typography.Text class="block"
          >版本：{{ systemUpdate.result.before || "-" }} ->
          {{ systemUpdate.result.after || "-" }}</Typography.Text
        >
        <Typography.Text class="block"
          >文件：{{ systemUpdate.result.asset || "-" }}</Typography.Text
        >
        <Typography.Text class="block muted">{{
          systemUpdate.result.output || ""
        }}</Typography.Text>
      </div>
      <Space
        v-if="systemUpdate.status === 'done' && !systemUpdate.restartChecking"
        style="justify-content: flex-end; width: 100%"
      >
        <Button @click="systemUpdate.open = false">关闭</Button>
        <Button
          v-if="systemUpdate.result"
          type="primary"
          :loading="systemUpdate.restarting"
          @click="restartAfterUpdate"
          >立即重启</Button
        >
      </Space>
      <Space
        v-if="systemUpdate.status === 'error'"
        style="justify-content: flex-end; width: 100%"
      >
        <Button @click="systemUpdate.open = false">关闭</Button>
      </Space>
    </Space>
  </Modal>
</template>

<style scoped>
.stats-chart {
  display: flex;
  align-items: flex-end;
  gap: 8px;
  overflow-x: auto;
  padding-bottom: 4px;
}
.stats-bar-col {
  flex: 1 0 40px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  min-width: 40px;
}
.stats-bar-value {
  font-size: 11px;
  color: #64748b;
  font-variant-numeric: tabular-nums;
  min-height: 14px;
}
.stats-bar-wrap {
  display: flex;
  align-items: flex-end;
  height: 120px;
  width: 100%;
  justify-content: center;
}
.stats-bar {
  width: 60%;
  max-width: 34px;
  border-radius: 6px 6px 2px 2px;
  background: linear-gradient(180deg, #4c8dff, #1677ff);
  min-height: 4px;
}
.stats-bar-label {
  font-size: 11px;
  color: #94a3b8;
  white-space: nowrap;
}
.stats-platforms {
  margin-top: 12px;
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
</style>

<style scoped>
.system-info-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(150px, 100%), 1fr));
  gap: 14px 20px;
}
.system-info-item > span {
  display: block;
  color: #94a3b8;
  font-size: 12px;
  margin-bottom: 4px;
}
.system-info-item strong {
  color: #0f172a;
  font-size: 14px;
  font-weight: 700;
  word-break: break-all;
}
.system-info-item-wide {
  grid-column: 1 / -1;
}
.system-info-value {
  display: flex;
  align-items: center;
  gap: 10px;
}
.spark-line {
  flex: 0 0 auto;
}
.spark-area {
  fill: rgba(22, 119, 255, 0.14);
}
.spark-stroke {
  stroke: #1677ff;
}
.mono {
  font-family: "Cascadia Code", "Fira Code", "Consolas", monospace;
}
</style>
