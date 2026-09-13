<script setup lang="ts">
import { History } from "lucide-vue-next";
import Button from "ant-design-vue/es/button";
import Popover from "ant-design-vue/es/popover";
import { useAdminViewContext } from "./adminViewContext";

const props = defineProps<{ label: string; platform: string }>();

const { botEvents } = useAdminViewContext();

function formatEventTime(time: number) {
  if (!time) return "-";
  return new Date(time * 1000).toLocaleString("zh-CN", { hour12: false });
}
</script>

<template>
  <Popover trigger="click" placement="left" :title="`${props.label} 最近事件`">
    <template #content>
      <div class="bot-event-list">
        <div
          v-if="!botEvents[props.platform]?.length"
          class="bot-event-empty"
        >
          暂无事件
        </div>
        <div
          v-for="eventItem in [...(botEvents[props.platform] || [])].reverse()"
          :key="`${eventItem.time}-${eventItem.text}`"
          class="bot-event-row"
        >
          <span class="bot-event-time">{{
            formatEventTime(eventItem.time)
          }}</span>
          <span>{{ eventItem.text }}</span>
        </div>
      </div>
    </template>
    <Button
      class="bot-card-events"
      type="text"
      shape="circle"
      :title="`${props.label}事件`"
      :aria-label="`${props.label}事件`"
    >
      <template #icon><History :size="18" /></template>
    </Button>
  </Popover>
</template>

<style scoped>
.bot-event-list {
  max-height: 260px;
  overflow: auto;
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 260px;
  max-width: 340px;
  font-size: 12px;
}
.bot-event-empty {
  color: #94a3b8;
}
.bot-event-row {
  display: flex;
  gap: 8px;
  align-items: baseline;
  color: #374151;
}
.bot-event-time {
  flex: 0 0 auto;
  color: #94a3b8;
  font-variant-numeric: tabular-nums;
}
</style>
