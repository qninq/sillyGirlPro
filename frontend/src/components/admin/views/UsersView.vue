<script setup lang="ts">
import Avatar from "ant-design-vue/es/avatar";
import Button from "ant-design-vue/es/button";
import { Edit3, Plus, RefreshCw, Trash2 } from "lucide-vue-next";
import Form from "ant-design-vue/es/form";
import Input from "ant-design-vue/es/input";
import Modal from "ant-design-vue/es/modal";
import Popconfirm from "ant-design-vue/es/popconfirm";
import Space from "ant-design-vue/es/space";
import Switch from "ant-design-vue/es/switch";
import Table from "ant-design-vue/es/table";
import Tag from "ant-design-vue/es/tag";
import Typography from "ant-design-vue/es/typography";
import { qqAvatarUrl, timestamp } from "../../../utils";
import { useAdminViewContext } from "../adminViewContext";

const {
  loadNormalUsers,
  normalUsers,
  openNormalUser,
  page,
  removeNormalUser,
  saveNormalUser,
  user,
} = useAdminViewContext();

function rowAvatar(record: any) {
  const qq =
    record.bindings?.qq ||
    (/^\d{5,12}$/.test(record.username || "") ? record.username : "");
  return qqAvatarUrl(qq);
}

function rowInitial(record: any) {
  return (record.nickname || record.username || "U").slice(0, 1).toUpperCase();
}
</script>

<template>
  <section v-if="page === 'users'" class="panel">
    <div class="toolbar">
      <div class="toolbar-left">
        <Typography.Text strong>普通用户</Typography.Text>
        <Tag>{{ normalUsers.total }}</Tag>
        <Input
          v-model:value="normalUsers.search"
          class="user-search"
          placeholder="搜索账号 / 邮箱 / QQ / TGID"
          allow-clear
        />
      </div>
      <Space>
        <Button type="primary" @click="openNormalUser()"
          ><template #icon><Plus :size="16" /></template>新增账号</Button
        >
        <Button @click="loadNormalUsers"
          ><template #icon><RefreshCw :size="16" /></template>刷新</Button
        >
      </Space>
    </div>
    <Table
      row-key="id"
      :loading="normalUsers.loading"
      :data-source="normalUsers.rows"
      :pagination="{ pageSize: 20, total: normalUsers.total }"
    >
      <Table.Column title="#" :width="72">
        <template #default="{ index }">{{ index + 1 }}</template>
      </Table.Column>
      <Table.Column title="账号" data-index="username" :width="220">
        <template #default="{ record }">
          <Space size="middle" align="center">
            <Avatar
              :size="40"
              class="user-row-avatar"
              :src="rowAvatar(record) || undefined"
              >{{ rowAvatar(record) ? "" : rowInitial(record) }}</Avatar
            >
            <Space direction="vertical" size="small">
              <Typography.Text strong>{{
                record.nickname || record.username
              }}</Typography.Text>
              <Typography.Text class="muted">{{
                record.username
              }}</Typography.Text>
            </Space>
          </Space>
        </template>
      </Table.Column>
      <Table.Column title="邮箱" data-index="email" :width="200">
        <template #default="{ record }">
          <Typography.Text class="mono">{{
            record.email || "-"
          }}</Typography.Text>
        </template>
      </Table.Column>
      <Table.Column title="QQ" :width="130">
        <template #default="{ record }">
          <Typography.Text class="mono">{{
            record.bindings?.qq || "-"
          }}</Typography.Text>
        </template>
      </Table.Column>
      <Table.Column title="TGID" :width="170">
        <template #default="{ record }">
          <Typography.Text class="mono">{{
            record.bindings?.telegram || "-"
          }}</Typography.Text>
        </template>
      </Table.Column>
      <Table.Column title="注册时间" data-index="created_at" :width="180">
        <template #default="{ text }">{{ timestamp(text) }}</template>
      </Table.Column>
      <Table.Column title="状态" :width="100">
        <template #default="{ record }">
          <Tag :color="record.disabled ? 'default' : 'green'">{{
            record.disabled ? "禁用" : "正常"
          }}</Tag>
        </template>
      </Table.Column>
      <Table.Column title="操作" fixed="right" :width="130">
        <template #default="{ record }">
          <Space size="small">
            <Button
              type="text"
              title="编辑账号"
              :aria-label="`编辑账号 ${record.username}`"
              @click="openNormalUser(record)"
            >
              <Edit3 :size="16" />
            </Button>
            <Popconfirm
              :title="`确认删除账号「${record.username}」？`"
              description="账号、QQ/TGID 绑定将一并删除。"
              ok-text="确认删除"
              cancel-text="取消"
              @confirm="removeNormalUser(record)"
            >
              <Button
                type="text"
                danger
                :loading="normalUsers.deleting[record.id]"
                title="删除账号"
                :aria-label="`删除账号 ${record.username}`"
              >
                <Trash2 :size="16" />
              </Button>
            </Popconfirm>
          </Space>
        </template>
      </Table.Column>
    </Table>
  </section>

  <Modal
    v-model:open="normalUsers.modalOpen"
    :title="
      normalUsers.editing
        ? `编辑账号：${normalUsers.editing.username}`
        : '新增账号'
    "
    width="620px"
    ok-text="保存"
    cancel-text="取消"
    :confirm-loading="normalUsers.saving"
    @ok="saveNormalUser"
  >
    <Form layout="vertical">
      <Form.Item
        label="账号"
        html-for="normal-user-username"
        required
        extra="3-32 位字母、数字、下划线、横线或点；创建后不可修改。"
      >
        <Input
          id="normal-user-username"
          v-model:value="normalUsers.form.username"
          name="normal-user-username"
          :disabled="!!normalUsers.editing"
          autocomplete="username"
          placeholder="请输入登录账号"
        />
      </Form.Item>
      <Form.Item
        :label="normalUsers.editing ? '新密码' : '密码'"
        html-for="normal-user-password"
        :required="!normalUsers.editing"
        :extra="normalUsers.editing ? '留空则保留原密码。' : '至少 6 位。'"
      >
        <Input.Password
          id="normal-user-password"
          v-model:value="normalUsers.form.password"
          name="normal-user-password"
          autocomplete="new-password"
          placeholder="请输入密码"
        />
      </Form.Item>
      <Form.Item label="昵称" html-for="normal-user-nickname">
        <Input
          id="normal-user-nickname"
          v-model:value="normalUsers.form.nickname"
          name="normal-user-nickname"
          maxlength="64"
          placeholder="留空则使用账号名"
        />
      </Form.Item>
      <Form.Item
        label="邮箱"
        html-for="normal-user-email"
        extra="仅支持 QQ 邮箱（QQ号@qq.com）；用户端注册的账号即邮箱前缀 QQ 号。"
      >
        <Input
          id="normal-user-email"
          v-model:value="normalUsers.form.email"
          name="normal-user-email"
          placeholder="例如：123456@qq.com"
        />
      </Form.Item>
      <Form.Item label="绑定 QQ" html-for="normal-user-qq">
        <Input
          id="normal-user-qq"
          v-model:value="normalUsers.form.qq"
          name="normal-user-qq"
          inputmode="numeric"
          placeholder="5-12 位 QQ 号；留空解除绑定"
        />
      </Form.Item>
      <Form.Item label="绑定 TGID" html-for="normal-user-tgid">
        <Input
          id="normal-user-tgid"
          v-model:value="normalUsers.form.telegram"
          name="normal-user-tgid"
          placeholder="Telegram 用户 ID；留空解除绑定"
        />
      </Form.Item>
      <Form.Item label="禁用账号" html-for="normal-user-disabled">
        <Switch
          id="normal-user-disabled"
          v-model:checked="normalUsers.form.disabled"
        />
      </Form.Item>
    </Form>
  </Modal>
</template>

<style scoped>
.user-search {
  width: 240px;
  margin-left: 12px;
}

.user-row-avatar {
  flex: 0 0 auto;
  color: #ffffff;
  background: #111827;
  font-weight: 700;
}
</style>
