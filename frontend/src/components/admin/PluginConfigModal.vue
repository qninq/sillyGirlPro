<script setup lang="ts">
import Alert from "ant-design-vue/es/alert";
import Empty from "ant-design-vue/es/empty";
import Form from "ant-design-vue/es/form";
import Input from "ant-design-vue/es/input";
import InputNumber from "ant-design-vue/es/input-number";
import Modal from "ant-design-vue/es/modal";
import Radio from "ant-design-vue/es/radio";
import Select from "ant-design-vue/es/select";
import Spin from "ant-design-vue/es/spin";
import Switch from "ant-design-vue/es/switch";
import Typography from "ant-design-vue/es/typography";
import { useAdminViewContext } from "./adminViewContext";

const {
  fieldOptions,
  fieldType,
  pluginConfigFieldVisible,
  pluginConfigs,
  pluginPanelChoices,
  pluginPanelEmptyText,
  pluginPanelKind,
  pluginSettingsCanSave,
  savePluginConfig,
  schemaFields,
} = useAdminViewContext();
</script>

<template>
  <Modal
    v-model:open="pluginConfigs.modalOpen"
    :title="`${pluginConfigs.selected?.plugin || pluginConfigs.selected?.title || '插件'} 设置`"
    width="720px"
    ok-text="保存"
    cancel-text="取消"
    :confirm-loading="pluginConfigs.saving"
    :ok-button-props="{ disabled: !pluginSettingsCanSave }"
    @ok="savePluginConfig"
  >
    <Spin :spinning="pluginConfigs.loading">
      <div v-if="pluginConfigs.selected" class="plugin-settings-modal-body">
        <div
          v-if="
            pluginConfigs.configurable ||
            pluginConfigs.selected.registered === false
          "
          class="config-form plugin-config-modal-form"
        >
          <Typography.Text class="muted mono">{{
            pluginConfigs.selected.file || "main.js"
          }}</Typography.Text>
          <Alert
            v-if="pluginConfigs.selected.registered === false"
            type="warning"
            show-icon
            style="margin-top: 16px"
            message="该插件检测到配置代码，但安装时没有成功导出配置表单。请确认 new form({...}) 在脚本顶层执行，且脚本初始化没有报错。"
          />
          <Form
            v-if="pluginConfigs.configurable"
            layout="vertical"
            style="margin-top: 16px"
          >
            <template v-for="field in schemaFields" :key="field.key">
              <Form.Item
                v-if="pluginConfigFieldVisible(field)"
                :label="field.prop.title || field.key"
                :html-for="`plugin-config-${field.key}`"
                :extra="field.prop.description"
              >
                <template v-if="pluginPanelKind(field)">
                  <Radio.Group
                    :id="`plugin-config-${field.key}`"
                    v-model:value="pluginConfigs.form[field.key]"
                    class="plugin-panel-id-picker"
                    button-style="solid"
                    :aria-label="field.prop.title || field.key"
                  >
                    <Radio.Button
                      v-for="id in pluginPanelChoices(field)"
                      :key="id"
                      :value="id"
                      >{{ id }}</Radio.Button
                    >
                  </Radio.Group>
                  <div
                    v-if="!pluginPanelChoices(field).length"
                    class="plugin-panel-empty"
                  >
                    {{ pluginPanelEmptyText(field) }}
                  </div>
                </template>
                <Select
                  v-else-if="fieldType(field.prop) === 'enum'"
                  :id="`plugin-config-${field.key}`"
                  v-model:value="pluginConfigs.form[field.key]"
                  :options="fieldOptions(field.prop)"
                />
                <Switch
                  v-else-if="fieldType(field.prop) === 'boolean'"
                  :id="`plugin-config-${field.key}`"
                  v-model:checked="pluginConfigs.form[field.key]"
                />
                <InputNumber
                  v-else-if="
                    fieldType(field.prop) === 'number' ||
                    fieldType(field.prop) === 'integer'
                  "
                  :id="`plugin-config-${field.key}`"
                  v-model:value="pluginConfigs.form[field.key]"
                  style="width: 100%"
                  :min="field.prop.minimum"
                  :max="field.prop.maximum"
                />
                <Input.TextArea
                  v-else-if="
                    fieldType(field.prop) === 'object' ||
                    fieldType(field.prop) === 'array'
                  "
                  :id="`plugin-config-${field.key}`"
                  :name="field.key"
                  v-model:value="pluginConfigs.text[field.key]"
                  :rows="6"
                  class="mono"
                />
                <Input.Password
                  v-else-if="
                    field.prop.format === 'password' ||
                    field.prop['ui:widget'] === 'password'
                  "
                  :id="`plugin-config-${field.key}`"
                  :name="field.key"
                  v-model:value="pluginConfigs.form[field.key]"
                />
                <Input.TextArea
                  v-else-if="
                    field.prop.format === 'textarea' ||
                    field.prop['ui:widget'] === 'textarea'
                  "
                  :id="`plugin-config-${field.key}`"
                  :name="field.key"
                  v-model:value="pluginConfigs.form[field.key]"
                  :rows="4"
                />
                <Input
                  v-else
                  :id="`plugin-config-${field.key}`"
                  :name="field.key"
                  v-model:value="pluginConfigs.form[field.key]"
                />
              </Form.Item>
            </template>
          </Form>
        </div>
        <Empty v-else description="该插件没有其他配置项" />
      </div>
    </Spin>
  </Modal>
</template>
