<script setup lang="ts">
import Alert from "ant-design-vue/es/alert";
import Button from "ant-design-vue/es/button";
import Form from "ant-design-vue/es/form";
import Input from "ant-design-vue/es/input";
import Modal from "ant-design-vue/es/modal";
import Popconfirm from "ant-design-vue/es/popconfirm";
import Select from "ant-design-vue/es/select";
import Space from "ant-design-vue/es/space";
import Spin from "ant-design-vue/es/spin";
import Tag from "ant-design-vue/es/tag";
import { Moon, Save, Sun, Trash2, Wand2 } from "lucide-vue-next";
import { computed } from "vue";
import { useAdminViewContext } from "./adminViewContext";

const {
  pluginEditor,
  pluginEditorHost,
  closeMarketPluginEditor,
  deleteMarketPluginEditor,
  formatMarketPluginEditor,
  handlePluginEditorOpenChange,
  saveMarketPluginEditor,
  onPluginEditorLanguageChange,
  pluginEditorLanguageOptions,
  syncPluginEditorLanguage,
  togglePluginEditorTheme,
} = useAdminViewContext();

const isScriptMode = computed(() => pluginEditor.mode === "script");
const modalTitle = computed(() =>
  isScriptMode.value
    ? "新建应用脚本"
    : pluginEditor.isNew
      ? "新增本地插件"
      : `编辑插件：${pluginEditor.title || pluginEditor.id}`,
);
const saveText = computed(() => (isScriptMode.value ? "创建" : "保存"));
const alertMessage = computed(() =>
  isScriptMode.value
    ? "可执行脚本（ES5/Node.js/Python/Adapter JS/Adapter Python）必须包含 [title: xxx]、[name: 文件名]、[desc: xxx]、[version: vx.y.z]，以及 [rule: xxx] 或 [cron: xxx]/[on_start: true]/[web: true]/[module: true] 之一；TypeScript/Golang/Adapter Go 为占位脚本，仅落盘不加载。切换语言时会自动替换对应模板。"
    : "本地新增插件会自动作为非公开插件进入插件市场；保存前必须包含 [title: xxx]、[name: 文件名]、[desc: xxx]、[version: vx.y.z]，以及 [rule: xxx] 或 [cron: xxx]/[on_start: true]/[web: true]/[module: true]。",
);
const nameExtra = computed(() =>
  isScriptMode.value
    ? "可执行脚本必须和源码里的 [name: xxx] 一致"
    : "新增时必须和源码里的 [name: xxx] 一致",
);
const namePlaceholder = computed(() =>
  isScriptMode.value ? "例如 myWorker" : "例如 localPlugin",
);

function onLanguageChange(value: unknown) {
  if (pluginEditor.mode === "script") onPluginEditorLanguageChange(value);
  else syncPluginEditorLanguage();
}
</script>

<template>
  <Modal
    v-model:open="pluginEditor.open"
    :title="modalTitle"
    width="1080px"
    :footer="null"
    :destroy-on-close="true"
    @cancel="closeMarketPluginEditor"
    @after-open-change="handlePluginEditorOpenChange"
  >
    <Spin :spinning="pluginEditor.loading">
      <Space direction="vertical" style="width: 100%" size="middle">
        <Alert type="info" show-icon :message="alertMessage" />
        <Form layout="inline" class="plugin-editor-meta">
          <Form.Item label="名称" required :extra="nameExtra">
            <Input
              id="plugin-editor-name"
              name="plugin-editor-name"
              v-model:value="pluginEditor.name"
              style="width: 260px"
              :placeholder="namePlaceholder"
              :disabled="pluginEditor.installed && !pluginEditor.isNew"
              @input="pluginEditor.name = String(($event.target as HTMLInputElement).value ?? '')"
              @change="pluginEditor.name = String(($event.target as HTMLInputElement).value ?? '')"
            />
          </Form.Item>
          <Form.Item label="语言" required>
            <Select
              id="plugin-editor-type"
              v-model:value="pluginEditor.type"
              style="width: 160px"
              :options="pluginEditorLanguageOptions"
              @change="onLanguageChange"
            />
          </Form.Item>
          <Form.Item v-if="!isScriptMode">
            <Tag :color="pluginEditor.installed ? 'green' : 'blue'">{{
              pluginEditor.installed ? "本地已安装" : "未安装/远程源码"
            }}</Tag>
          </Form.Item>
          <Form.Item>
            <Button @click="togglePluginEditorTheme">
              <template #icon
                ><Moon v-if="pluginEditor.theme === 'dark'" :size="16" /><Sun
                  v-else
                  :size="16"
              /></template>
              {{ pluginEditor.theme === "dark" ? "黑底" : "白底" }}
            </Button>
          </Form.Item>
        </Form>
        <div
          ref="pluginEditorHost"
          class="script-code-editor plugin-market-code-editor"
          :class="{
            'plugin-market-code-editor-dark': pluginEditor.theme === 'dark',
            'plugin-market-code-editor-light': pluginEditor.theme === 'light',
          }"
        ></div>
        <Space style="justify-content: flex-end; width: 100%">
          <Popconfirm
            v-if="pluginEditor.installed && !pluginEditor.isNew && !isScriptMode"
            title="确认删除这个本地插件文件？"
            @confirm="deleteMarketPluginEditor"
          >
            <Button danger :loading="pluginEditor.deleting"
              ><template #icon><Trash2 :size="16" /></template>删除</Button
            >
          </Popconfirm>
          <Button @click="formatMarketPluginEditor"
            ><template #icon><Wand2 :size="16" /></template>格式化</Button
          >
          <Button @click="closeMarketPluginEditor">取消</Button>
          <Button
            type="primary"
            :loading="pluginEditor.saving"
            @click="saveMarketPluginEditor"
            ><template #icon><Save :size="16" /></template>{{ saveText }}</Button
          >
        </Space>
      </Space>
    </Spin>
  </Modal>
</template>
