<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import Button from "ant-design-vue/es/button";
import Input from "ant-design-vue/es/input";
import Select from "ant-design-vue/es/select";
import Switch from "ant-design-vue/es/switch";
import Table from "ant-design-vue/es/table";
import Tag from "ant-design-vue/es/tag";
import Tooltip from "ant-design-vue/es/tooltip";
import message from "ant-design-vue/es/message";
import { RefreshCw } from "lucide-vue-next";
import { get, post } from "../../../api";

interface CommandItem {
  key: string;
  title: string;
  desc: string;
  rules: string[];
  admin: boolean;
  status: boolean;
  origin: "builtin" | "plugin";
  source: string;
  plugin_id?: string;
  type: string;
  class: string;
  author: string;
  version: string;
  carry: boolean;
  cron_count: number;
  readonly?: boolean;
}

interface CommandListData {
  list: CommandItem[];
  stats: { builtin: number; plugin: number; adminOnly: number };
}

const rows = ref<CommandItem[]>([]);
const stats = ref({ builtin: 0, plugin: 0, adminOnly: 0 });
const loading = ref(false);
const keyword = ref("");
const originFilter = ref<string>("all");

const originOptions = [
  { value: "all", label: "全部来源" },
  { value: "builtin", label: "内置" },
  { value: "plugin", label: "插件" },
];

async function loadCommands() {
  loading.value = true;
  try {
    const res = await get<{ data: CommandListData }>("/api/admin/command-list");
    rows.value = res.data?.list || [];
    stats.value = res.data?.stats || { builtin: 0, plugin: 0, adminOnly: 0 };
  } catch (error) {
    message.error(error instanceof Error ? error.message : "加载指令列表失败");
  } finally {
    loading.value = false;
  }
}

const filteredRows = computed(() => {
  const kw = keyword.value.trim().toLowerCase();
  return rows.value.filter((row) => {
    if (originFilter.value !== "all" && row.origin !== originFilter.value) {
      return false;
    }
    if (!kw) return true;
    const fields = [
      row.title,
      row.desc,
      row.source,
      row.plugin_id || "",
      row.class,
      row.author,
      ...row.rules,
    ];
    return fields.some((field) =>
      (field || "").toLowerCase().includes(kw),
    );
  });
});

async function toggleAdmin(row: CommandItem, checked: boolean) {
  const previous = row.admin;
  row.admin = checked;
  try {
    await post(`/api/admin/command-list/${encodeURIComponent(row.key)}/admin`, {
      admin: checked,
    });
    if (checked) {
      stats.value.adminOnly += 1;
    } else {
      stats.value.adminOnly = Math.max(0, stats.value.adminOnly - 1);
    }
  } catch (error) {
    row.admin = previous;
    message.error(error instanceof Error ? error.message : "保存失败");
  }
}

onMounted(loadCommands);
</script>

<script lang="ts">
export default { name: "CommandsView" };
</script>

<template>
  <section class="panel commands-panel">
    <div class="commands-header">
      <div class="commands-title-wrap">
        <div class="commands-title">
          <span class="commands-title-text">指令列表</span>
          <Tag class="count-tag">内置 {{ stats.builtin }}</Tag>
          <Tag class="count-tag">插件 {{ stats.plugin }}</Tag>
          <Tag class="count-tag count-tag-admin">仅管理员 {{ stats.adminOnly }}</Tag>
        </div>
        <div class="commands-subtitle">先看触发词，再确认用途和来源。</div>
      </div>
      <Button :loading="loading" @click="loadCommands">
        <template #icon><RefreshCw :size="14" /></template>
        刷新
      </Button>
    </div>

    <div class="commands-body">
      <div class="commands-toolbar">
        <Input
          v-model:value="keyword"
          placeholder="搜索指令、触发词、插件或路径"
          allow-clear
        >
          <template #prefix>
            <span class="search-icon">⌕</span>
          </template>
        </Input>
        <Select
          v-model:value="originFilter"
          :options="originOptions"
          class="commands-origin-select"
        />
      </div>

      <Table
        :data-source="filteredRows"
        :loading="loading"
        :row-key="(row: any) => row.key"
        :pagination="{ pageSize: 20, showSizeChanger: false }"
      >
        <Table.Column title="触发词" :width="240">
          <template #default="{ record }">
            <div class="rule-chips">
              <span
                v-for="rule in (record as CommandItem).rules.slice(0, 4)"
                :key="rule"
                class="rule-chip"
                :title="rule"
                >{{ rule }}</span
              >
              <span
                v-if="(record as CommandItem).rules.length > 4"
                class="rule-chip rule-chip-more"
                :title="(record as CommandItem).rules.join('\n')"
              >
                +{{ (record as CommandItem).rules.length - 4 }}
              </span>
            </div>
          </template>
        </Table.Column>
        <Table.Column title="用途">
          <template #default="{ record }">
            <div class="usage-title">{{ (record as CommandItem).title }}</div>
            <div v-if="(record as CommandItem).desc" class="usage-desc">
              {{ (record as CommandItem).desc }}
            </div>
          </template>
        </Table.Column>
        <Table.Column title="来源" :width="260">
          <template #default="{ record }">
            <div class="source-line">
              <Tag
                :class="
                  (record as CommandItem).origin === 'builtin'
                    ? 'source-tag source-builtin'
                    : 'source-tag source-plugin'
                "
              >
                {{
                  (record as CommandItem).origin === "builtin"
                    ? "内置"
                    : "市场插件"
                }}
              </Tag>
              <Tooltip :title="(record as CommandItem).source">
                <span class="source-name">{{
                  (record as CommandItem).source
                }}</span>
              </Tooltip>
              <Tag
                v-if="!(record as CommandItem).status"
                class="source-tag source-disabled"
                >已停用</Tag
              >
            </div>
          </template>
        </Table.Column>
        <Table.Column title="状态" :width="110">
          <template #default="{ record }">
            <span
              v-if="(record as CommandItem).admin"
              class="scope-badge scope-admin"
              >仅管理员</span
            >
            <span v-else class="scope-badge">所有人</span>
          </template>
        </Table.Column>
        <Table.Column title="管理员限制" :width="120" align="right">
          <template #default="{ record }">
            <Tooltip
              v-if="(record as CommandItem).readonly"
              title="内置命令的管理员限制由核心硬编码，不可修改"
            >
              <Switch
                :checked="(record as CommandItem).admin"
                size="small"
                disabled
              />
            </Tooltip>
            <Switch
              v-else
              :checked="(record as CommandItem).admin"
              size="small"
              @change="
                (checked: any) => toggleAdmin(record as CommandItem, !!checked)
              "
            />
          </template>
        </Table.Column>
      </Table>
    </div>
  </section>
</template>

<style scoped>
.commands-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.commands-title {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.commands-title-text {
  font-size: 18px;
  font-weight: 600;
}

.commands-subtitle {
  margin-top: 4px;
  font-size: 13px;
  color: #8c8c8c;
}

.count-tag {
  margin-inline-end: 0;
  background: #f5f5f5;
  color: #595959;
  border-radius: 10px;
}

.count-tag-admin {
  background: #f0f5ff;
  color: #2f54eb;
}

.commands-body {
  background: #fff;
  border: 1px solid #f0f0f0;
  border-radius: 8px;
  padding: 16px;
}

.commands-toolbar {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
}

.commands-origin-select {
  width: 140px;
  flex-shrink: 0;
}

.search-icon {
  color: #bfbfbf;
}

.rule-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.rule-chip {
  display: inline-block;
  max-width: 220px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  background: #f5f5f5;
  border-radius: 4px;
  padding: 0 6px;
  font-family: "Cascadia Code", "Fira Code", "Consolas", monospace;
  font-size: 12px;
  color: #d4380d;
  line-height: 20px;
}

.rule-chip-more {
  color: #8c8c8c;
}

.usage-title {
  font-weight: 600;
}

.usage-desc {
  margin-top: 2px;
  font-size: 12px;
  color: #8c8c8c;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.source-line {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.source-tag {
  margin-inline-end: 0;
  border-radius: 4px;
}

.source-builtin {
  background: #e6f4ff;
  color: #1677ff;
  border-color: #91caff;
}

.source-plugin {
  background: #f9f0ff;
  color: #722ed1;
  border-color: #d3adf7;
}

.source-disabled {
  background: #fff1f0;
  color: #cf1322;
  border-color: #ffa39e;
}

.source-name {
  font-size: 12px;
  color: #8c8c8c;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.scope-badge {
  display: inline-block;
  padding: 0 8px;
  border-radius: 10px;
  background: #f5f5f5;
  color: #595959;
  font-size: 12px;
  line-height: 20px;
}

.scope-admin {
  background: #1f1f1f;
  color: #fff;
}
</style>
