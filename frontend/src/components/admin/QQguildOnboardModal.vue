<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from "vue";
import Button from "ant-design-vue/es/button";
import Empty from "ant-design-vue/es/empty";
import Modal from "ant-design-vue/es/modal";
import QRCode from "qrcode";
import Space from "ant-design-vue/es/space";
import Spin from "ant-design-vue/es/spin";
import Typography from "ant-design-vue/es/typography";
import message from "ant-design-vue/es/message";
import { RefreshCw } from "lucide-vue-next";
import { post } from "../../api";

interface QQguildOnboardTask {
  task_id: string;
  qr_code_url: string;
  expires_at: number;
}

interface QQguildOnboardPoll {
  status: "wait" | "scanned" | "confirmed" | "expired";
  app_id?: string;
  client_secret?: string;
  message?: string;
}

const props = defineProps<{ open: boolean }>();
const emit = defineEmits<{
  (e: "update:open", value: boolean): void;
  (e: "confirmed", payload: { app_id: string; client_secret: string }): void;
}>();

const starting = ref(false);
const qrcodeImg = ref("");
const status = ref<QQguildOnboardPoll["status"]>("wait");
const statusMessage = ref("");
const taskId = ref("");

let pollTimer: number | null = null;
let generation = 0;

function stopPoll() {
  if (pollTimer !== null) {
    window.clearTimeout(pollTimer);
    pollTimer = null;
  }
}

function close() {
  generation += 1;
  stopPoll();
  emit("update:open", false);
}

async function start() {
  stopPoll();
  generation += 1;
  const current = generation;
  starting.value = true;
  qrcodeImg.value = "";
  statusMessage.value = "正在生成绑定二维码…";
  try {
    const res = await post<{ data: QQguildOnboardTask }>(
      "/api/admin/qqguild-onboard-tasks",
      {},
    );
    if (current !== generation || !props.open) return;
    taskId.value = res.data.task_id;
    qrcodeImg.value = await QRCode.toDataURL(res.data.qr_code_url, {
      errorCorrectionLevel: "M",
      margin: 1,
      width: 260,
    });
    status.value = "wait";
    statusMessage.value = "请使用手机 QQ 扫描二维码";
    schedulePoll(300);
  } catch (error) {
    statusMessage.value =
      error instanceof Error ? error.message : "生成二维码失败";
    message.error(statusMessage.value);
  } finally {
    if (current === generation) {
      starting.value = false;
    }
  }
}

async function poll() {
  if (!taskId.value || !props.open) return;
  const current = generation;
  try {
    const res = await post<{ data: QQguildOnboardPoll }>(
      `/api/admin/qqguild-onboard-tasks/${encodeURIComponent(
        taskId.value,
      )}/polls`,
      {},
    );
    if (current !== generation || !props.open) return;
    const data = res.data;
    status.value = data.status;
    statusMessage.value = data.message || "";
    if (data.status === "confirmed") {
      emit("confirmed", {
        app_id: data.app_id || "",
        client_secret: data.client_secret || "",
      });
      message.success("扫码绑定成功，机器人配置已保存并启用");
      close();
      return;
    }
    if (data.status === "expired") return;
    schedulePoll(300);
  } catch (error) {
    if (current !== generation || !props.open) return;
    statusMessage.value =
      error instanceof Error ? error.message : "查询扫码状态失败";
    schedulePoll(1500);
  }
}

function schedulePoll(delay: number) {
  stopPoll();
  pollTimer = window.setTimeout(poll, delay);
}

watch(
  () => props.open,
  (open) => {
    if (open) {
      start();
    } else {
      generation += 1;
      stopPoll();
    }
  },
);

onBeforeUnmount(() => {
  generation += 1;
  stopPoll();
});
</script>

<template>
  <Modal
    :open="props.open"
    title="扫码添加 QQ 官方机器人"
    :footer="null"
    @cancel="close"
  >
    <div class="onboard-modal">
      <div class="onboard-qr-frame">
        <Spin :spinning="starting">
          <img
            v-if="qrcodeImg"
            :src="qrcodeImg"
            alt="QQ 扫码绑定二维码"
          />
          <Empty v-else :description="statusMessage || '等待生成二维码'" />
        </Spin>
      </div>
      <Typography.Text>{{ statusMessage }}</Typography.Text>
      <Space>
        <Button :loading="starting" @click="start">
          <template #icon><RefreshCw :size="16" /></template>
          重新生成
        </Button>
        <Button @click="close">关闭</Button>
      </Space>
    </div>
  </Modal>
</template>

<style scoped>
.onboard-modal {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 14px;
  padding: 6px 0;
}

.onboard-qr-frame {
  padding: 10px;
  border: 1px solid #f0f0f0;
  border-radius: 8px;
  min-width: 220px;
  min-height: 220px;
  display: flex;
  align-items: center;
  justify-content: center;
}
</style>
