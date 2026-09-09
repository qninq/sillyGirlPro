<script setup lang="ts">
import Button from "ant-design-vue/es/button";
import Dropdown from "ant-design-vue/es/dropdown";
import Form from "ant-design-vue/es/form";
import Input from "ant-design-vue/es/input";
import Menu from "ant-design-vue/es/menu";
import Modal from "ant-design-vue/es/modal";
import Select from "ant-design-vue/es/select";
import Spin from "ant-design-vue/es/spin";
import Tag from "ant-design-vue/es/tag";
import Tooltip from "ant-design-vue/es/tooltip";
import { computed, ref } from "vue";
import {
  Bug,
  ChevronRight,
  FileCode2,
  MoreHorizontal,
  Plus,
  Search,
  Settings2,
  Sun,
  Moon,
  Save,
} from "lucide-vue-next";
import type { AppScriptInfo } from "../../../types";
import {
  appScriptLanguageLabels,
  type AppScriptLanguage,
} from "../../../composables/admin/useScriptsAdmin";
import { useAdminViewContext } from "../adminViewContext";
import PluginConfigModal from "../PluginConfigModal.vue";

const {
  appScripts,
  appScriptsTotal,
  appScriptCategories,
  appScriptCategoryChildren,
  toggleScriptCategory,
  appScriptStatusToggling: scriptStatusToggling,
  toggleAppScriptStatus,
  enterScriptsPage,
  scriptEditor,
  scriptEditorHost,
  openScriptEditor,
  createScript,
  saveScriptEditor,
  deleteScriptEditor,
  toggleScriptEditorTheme,
  initScriptEditor,
  openMarketPluginConfig,
  page,
} = useAdminViewContext();

const scripts = appScripts;
const scriptsTotal = appScriptsTotal;
const scriptCategories = appScriptCategories;
const scriptCategoryChildren = appScriptCategoryChildren;

// 新建脚本弹窗状态。
const createModal = ref(false);
const createForm = ref<{ name: string; language: AppScriptLanguage }>({
  name: "",
  language: "node",
});
const createContent = ref("");
const createSaving = ref(false);

const editorLanguageLabel = computed(
  () =>
    appScriptLanguageLabels[scriptEditor.language as AppScriptLanguage] ||
    scriptEditor.language,
);

const canConfigure = computed(
  () =>
    scriptEditor.installed &&
    scriptEditor.id &&
    !scriptEditor.id.startsWith("file:") &&
    scriptEditor.row?.has_form === true,
);

function openCreateModal() {
  createForm.value = { name: "", language: "node" };
  createContent.value = "";
  createModal.value = true;
}

function onCreateLanguageChange(language: AppScriptLanguage) {
  createForm.value.language = language;
}

async function submitCreate() {
  const name = createForm.value.name.trim();
  if (!name) return;
  if (!createContent.value.trim()) return;
  createSaving.value = true;
  try {
    const created = await createScript({
      name,
      language: createForm.value.language,
      content: createContent.value,
    });
    if (created) createModal.value = false;
  } finally {
    createSaving.value = false;
  }
}

function selectCategory(language: string) {
  toggleScriptCategory(language);
}

function onScriptClick(item: AppScriptInfo) {
  void openScriptEditor(item);
}

function onConfigure() {
  const row = scriptEditor.row;
  if (!row) return;
  openMarketPluginConfig({
    id: row.id,
    title: row.title,
    has_form: row.has_form,
    install_status: row.installed ? 1 : 0,
  } as any);
}

const moreMenu = computed(() => [
  {
    key: "delete",
    label: "删除脚本",
    danger: true,
    disabled: !scriptEditor.id,
  },
]);

function onMoreMenu({ key }: { key: string }) {
  if (key === "delete") {
    Modal.confirm({
      title: "确认删除该脚本文件？",
      content: `将删除 ${scriptEditor.title || scriptEditor.name}`,
      okText: "删除",
      okType: "danger",
      cancelText: "取消",
      onOk: () => deleteScriptEditor(),
    });
  }
}
</script>

<template>
  <section v-if="page === 'scripts'" class="panel scripts-panel">
    <div class="scripts-layout">
      <!-- 左侧：语言分类目录 -->
      <aside class="scripts-sidebar">
        <div class="scripts-sidebar-header">
          <div class="scripts-total">
            <span class="scripts-total-title">脚本目录</span>
            <span class="scripts-total-badge">{{ scriptsTotal }}</span>
          </div>
          <Input
            v-model:value="scripts.keyword"
            placeholder="筛选脚本"
            allow-clear
          >
            <template #prefix><Search :size="14" /></template>
          </Input>
        </div>
        <Spin :spinning="scripts.loading" wrapper-class-name="scripts-categories-spin">
        <div class="scripts-categories">
          <div
            v-for="category in scriptCategories"
            :key="category.key"
            class="scripts-category"
            :class="{ expanded: scripts.expanded[category.key] }"
          >
            <button
              class="scripts-category-item"
              :class="{ active: scripts.category === category.key }"
              type="button"
              :aria-expanded="!!scripts.expanded[category.key]"
              @click="selectCategory(category.key)"
            >
              <span class="scripts-category-label">
                <span class="min-w-0 flex-1 scripts-category-name">{{
                  category.label
                }}</span>
                <Tag v-if="!category.executable" class="scripts-category-tag"
                  >占位</Tag
                >
              </span>
              <span class="scripts-category-side">
                <span
                  class="scripts-category-count"
                  :class="{ 'has-count': category.count > 0 }"
                  >{{ category.count }}</span
                >
                <ChevronRight :size="14" class="scripts-category-chevron" />
              </span>
            </button>
            <!-- 分类下的脚本列表 -->
            <div
              v-if="scripts.expanded[category.key]"
              class="scripts-category-scripts"
            >
              <div
                v-for="item in scriptCategoryChildren[category.key] || []"
                :key="item.id"
                class="scripts-list-item"
                :class="{
                  active: scriptEditor.id === item.id && scriptEditor.open,
                }"
                role="button"
                tabindex="0"
                @click="onScriptClick(item)"
                @keydown.enter.prevent="onScriptClick(item)"
              >
                <div class="scripts-list-main">
                  <span class="scripts-list-text">
                    <span class="scripts-list-title" :title="item.title || item.name">{{
                      item.title || item.name
                    }}</span>
                  </span>
                  <span v-if="item.version" class="scripts-list-version">{{
                    item.version
                  }}</span>
                </div>
                <button
                  v-if="item.installed && item.executable"
                  type="button"
                  class="scripts-list-status"
                  :class="{ on: item.status ?? true }"
                  :disabled="scriptStatusToggling[item.id]"
                  :title="`点击切换启停`"
                  :aria-label="`${item.title || item.name}启用状态切换`"
                  @click.stop="toggleAppScriptStatus(item)"
                >
                  {{ (item.status ?? true) ? "启用" : "停用" }}
                </button>
                <Tag
                  v-else-if="!item.executable"
                  class="scripts-list-tag"
                  color="default"
                  >占位</Tag
                >
              </div>
              <div
                v-if="
                  (scriptCategoryChildren[category.key] || []).length === 0
                "
                class="scripts-list-empty"
              >
                暂无脚本
              </div>
            </div>
          </div>
        </div>
        </Spin>
      </aside>

      <!-- 右侧：代码编辑器 -->
      <div class="scripts-main">
        <div class="scripts-toolbar">
          <div class="scripts-toolbar-meta">
            <Tag v-if="scriptEditor.open" color="blue">{{
              editorLanguageLabel
            }}</Tag>
            <Button size="small" @click="toggleScriptEditorTheme">
              <template #icon>
                <Moon v-if="scriptEditor.theme === 'dark'" :size="14" />
                <Sun v-else :size="14" />
              </template>
            </Button>
          </div>
          <div class="scripts-toolbar-actions">
            <Button type="primary" @click="openCreateModal"
              ><template #icon><Plus :size="16" /></template>新建</Button
            >
            <Tooltip
              :title="canConfigure ? '配置脚本参数' : '该脚本没有可配置参数'"
            >
              <Button :disabled="!canConfigure" @click="onConfigure"
                ><template #icon><Settings2 :size="16" /></template>配参</Button
              >
            </Tooltip>
            <Tooltip
              :title="
                scriptEditor.executable
                  ? '运行一次并捕获输出'
                  : '该语言分类暂不支持在线调试'
              "
            >
              <Button
                :disabled="!scriptEditor.open || !scriptEditor.id || !scriptEditor.executable"
                ><template #icon><Bug :size="16" /></template>调试</Button
              >
            </Tooltip>
            <Button
              :loading="scriptEditor.saving"
              :disabled="!scriptEditor.id"
              @click="saveScriptEditor"
              ><template #icon><Save :size="16" /></template>保存</Button
            >
            <Dropdown>
              <Button
                ><template #icon><MoreHorizontal :size="16" /></template>更多</Button
              >
              <template #overlay>
                <Menu :items="moreMenu" @click="onMoreMenu" />
              </template>
            </Dropdown>
          </div>
        </div>

        <Spin :spinning="scriptEditor.loading" wrapper-class-name="scripts-editor-spin">
          <div v-if="scriptEditor.open" class="scripts-editor-wrap">
            <div
              ref="scriptEditorHost"
              class="script-code-editor scripts-code-editor"
              :class="{
                'plugin-market-code-editor-dark':
                  scriptEditor.theme === 'dark',
                'plugin-market-code-editor-light':
                  scriptEditor.theme === 'light',
              }"
            ></div>
          </div>
          <div v-else class="scripts-empty">
            <FileCode2 :size="40" />
            <p>从左侧选择一个脚本，或点击「新建」开始开发</p>
          </div>
        </Spin>

      </div>
    </div>

    <!-- 新建脚本弹窗 -->
    <Modal
      v-model:open="createModal"
      title="新建应用脚本"
      width="560px"
      :confirm-loading="createSaving"
      ok-text="创建"
      cancel-text="取消"
      @ok="submitCreate"
    >
      <Form layout="vertical">
        <Form.Item label="脚本名称" required>
          <Input
            v-model:value="createForm.name"
            placeholder="例如 myWorker"
          />
        </Form.Item>
        <Form.Item label="语言分类" required>
          <Select
            v-model:value="createForm.language"
            :options="scriptCategories.map((c) => ({ label: c.label, value: c.key }))"
            @change="onCreateLanguageChange"
          />
        </Form.Item>
        <Form.Item
          label="初始内容"
          required
          extra="JS/Python 系脚本需包含插件注释元数据（[title]/[name]/[desc]/[version] 及 [on_start]/[web]/[cron] 之一）"
        >
          <Input.TextArea
            v-model:value="createContent"
            :rows="12"
            class="mono"
            placeholder="粘贴或输入脚本初始源码"
          />
        </Form.Item>
      </Form>
    </Modal>

    <PluginConfigModal />
  </section>
</template>
