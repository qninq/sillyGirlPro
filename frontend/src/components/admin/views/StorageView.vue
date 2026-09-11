<script setup lang="ts">
import Button from "ant-design-vue/es/button";
import Form from "ant-design-vue/es/form";
import Input from "ant-design-vue/es/input";
import Modal from "ant-design-vue/es/modal";
import Popconfirm from "ant-design-vue/es/popconfirm";
import Radio from "ant-design-vue/es/radio";
import Space from "ant-design-vue/es/space";
import Table from "ant-design-vue/es/table";
import {
  ChevronRight,
  FolderOpen,
  Pencil,
  Plus,
  RefreshCw,
  Search,
  Trash2,
} from "lucide-vue-next";
import type { StorageGroup } from "../../../composables/admin/useStorageAdmin";
import { useAdminViewContext } from "../adminViewContext";

const {
  applyEntrySearch,
  canManageSelectedBucket,
  changeEntryPage,
  deleteStorageEntry,
  destroyStorageEditor,
  formatStorageJson,
  handleStorageEditorKeydown,
  isSearchingBuckets,
  loadStorage,
  openAddEntry,
  openEditEntry,
  openRenameBucket,
  page,
  removeStorageBucket,
  selectStorageBucket,
  setStorageEditorMode,
  storageEditorHost,
  storageGroups,
  storageNodeOpen,
  storageState,
  submitEntry,
  submitRenameBucket,
  toggleStorageNode,
} = useAdminViewContext();

function groupChildren(group: StorageGroup) {
  const filter = storageState.bucketSearch.trim().toLowerCase();
  if (!filter) return group.children;
  return group.children.filter(
    (child) =>
      child.fullName.toLowerCase().includes(filter) ||
      child.label.toLowerCase().includes(filter),
  );
}

function groupActive(group: StorageGroup) {
  const selected = storageState.selected;
  return (
    selected === group.key || selected.startsWith(`${group.key}.`)
  );
}

function onGroupClick(group: StorageGroup) {
  if (group.leaf) {
    void selectStorageBucket(group.key);
    return;
  }
  const active = groupActive(group);
  toggleStorageNode(group.key);
  if (!active) {
    const children = groupChildren(group);
    if (children.length) void selectStorageBucket(children[0].fullName);
  }
}
</script>

<template>
  <section v-if="page === 'storage'" class="panel storage-panel">
    <aside class="storage-sidebar">
      <div class="storage-sidebar-header">
        <div class="storage-total">
          <span class="storage-total-title">
            <FolderOpen :size="15" />
            数据桶
          </span>
          <span class="storage-total-badge">{{
            storageState.buckets.length
          }}</span>
        </div>
        <Input
          id="storage-bucket-search"
          v-model:value="storageState.bucketSearch"
          aria-label="筛选数据桶"
          allow-clear
          placeholder="筛选数据桶"
          autocomplete="off"
        >
          <template #prefix><Search :size="14" /></template>
        </Input>
        <div class="storage-sidebar-actions">
          <Button
            size="small"
            :disabled="!canManageSelectedBucket"
            @click="openRenameBucket"
          >
            <template #icon><Pencil :size="14" /></template>改名
          </Button>
          <Popconfirm
            :title="`确认删除存储桶 ${storageState.selected}？`"
            description="删除后该桶内所有键值都会被移除，无法恢复。"
            ok-text="确认删除"
            cancel-text="取消"
            :disabled="!canManageSelectedBucket"
            @confirm="removeStorageBucket(storageState.selected)"
          >
            <Button
              danger
              size="small"
              :disabled="!canManageSelectedBucket"
              :loading="
                !!storageState.deletingBucket &&
                storageState.deletingBucket === storageState.selected
              "
            >
              <template #icon><Trash2 :size="14" /></template>删桶
            </Button>
          </Popconfirm>
        </div>
      </div>
      <div class="storage-groups">
        <div
          v-for="group in storageGroups"
          :key="group.key"
          class="storage-group"
          :class="{
            expanded:
              isSearchingBuckets || group.leaf || storageNodeOpen(group.key),
          }"
        >
          <button
            class="storage-group-item"
            :class="{ active: groupActive(group) }"
            type="button"
            :aria-expanded="isSearchingBuckets || storageNodeOpen(group.key)"
            @click="onGroupClick(group)"
          >
            <span class="storage-group-leading">
              <ChevronRight
                v-if="!group.leaf"
                :size="14"
                class="storage-group-chevron"
              />
              <span v-else class="storage-group-chevron-placeholder" />
              <span class="storage-group-name" :title="group.key">{{
                group.key
              }}</span>
            </span>
            <span
              class="storage-group-count"
              :class="{ 'has-count': group.children.length > 0 }"
              >{{ group.children.length }}</span
            >
          </button>
          <div
            v-if="!group.leaf && (isSearchingBuckets || storageNodeOpen(group.key))"
            class="storage-group-children"
          >
            <div
              v-for="child in groupChildren(group)"
              :key="child.fullName"
              class="storage-group-child"
              :class="{ active: storageState.selected === child.fullName }"
              role="button"
              tabindex="0"
              @click="selectStorageBucket(child.fullName)"
              @keydown.enter.prevent="selectStorageBucket(child.fullName)"
            >
              <span class="storage-group-child-name" :title="child.fullName">{{
                child.label
              }}</span>
            </div>
          </div>
        </div>
        <div v-if="!storageGroups.length" class="storage-groups-empty">
          没有匹配的数据桶
        </div>
      </div>
    </aside>

    <div class="storage-content">
      <div class="storage-content-head">
        <strong class="storage-content-title">{{
          storageState.selected || "请选择数据桶"
        }}</strong>
        <Space>
          <Button
            type="primary"
            :disabled="!storageState.selected"
            @click="openAddEntry"
          >
            <template #icon><Plus :size="16" /></template>新增
          </Button>
          <Popconfirm
            :title="`确认删除存储桶 ${storageState.selected}？`"
            description="删除后该桶内所有键值都会被移除，无法恢复。"
            ok-text="确认删除"
            cancel-text="取消"
            :disabled="!canManageSelectedBucket"
            @confirm="removeStorageBucket(storageState.selected)"
          >
            <Button
              danger
              :disabled="!canManageSelectedBucket"
              :loading="
                !!storageState.deletingBucket &&
                storageState.deletingBucket === storageState.selected
              "
            >
              <template #icon><Trash2 :size="16" /></template>删除数据桶
            </Button>
          </Popconfirm>
        </Space>
      </div>
      <div class="toolbar-left" style="margin-bottom: 12px">
        <Input
          id="storage-entry-search"
          v-model:value="storageState.entrySearch"
          aria-label="搜索 KEY / VALUE"
          allow-clear
          style="width: 280px"
          placeholder="搜索 KEY / VALUE"
          :disabled="!storageState.selected"
          autocomplete="off"
          @press-enter="applyEntrySearch"
        />
        <Button :disabled="!storageState.selected" @click="applyEntrySearch"
          ><template #icon><Search :size="16" /></template>查询</Button
        >
        <Button :disabled="!storageState.selected" @click="loadStorage"
          ><template #icon><RefreshCw :size="16" /></template>刷新</Button
        >
        <span class="storage-count">共 {{ storageState.total }} 条记录</span>
      </div>
      <Table
        :row-key="(record: any) => record.key"
        table-layout="fixed"
        :loading="storageState.loading"
        :data-source="storageState.rows"
        :pagination="{
          current: storageState.current,
          pageSize: storageState.pageSize,
          total: storageState.total,
          showSizeChanger: true,
        }"
        @change="changeEntryPage"
      >
        <Table.Column title="序号" data-index="index" :width="70" />
        <Table.Column title="Bucket" :width="180">
          <template #default="{ record }">
            <div class="storage-cell" :title="record.bucket">
              {{ record.bucket }}
            </div>
          </template>
        </Table.Column>
        <Table.Column title="KEY" :width="220">
          <template #default="{ record }">
            <div class="storage-cell" :title="record.key">{{ record.key }}</div>
          </template>
        </Table.Column>
        <Table.Column title="VALUE">
          <template #default="{ record }">
            <div class="storage-cell" :title="record.value">
              {{ record.value }}
            </div>
          </template>
        </Table.Column>
        <Table.Column title="操作" :width="100">
          <template #default="{ record }">
            <Button
              type="text"
              size="small"
              :title="`编辑 ${record.key}`"
              :aria-label="`编辑 ${record.key}`"
              @click="openEditEntry(record)"
              ><template #icon><Pencil :size="14" /></template
            ></Button>
            <Popconfirm
              title="确认删除该键？"
              ok-text="删除"
              cancel-text="取消"
              @confirm="deleteStorageEntry(record)"
            >
              <Button
                danger
                type="text"
                size="small"
                :title="`删除 ${record.key}`"
                :aria-label="`删除 ${record.key}`"
                ><template #icon><Trash2 :size="14" /></template
              ></Button>
            </Popconfirm>
          </template>
        </Table.Column>
      </Table>
    </div>
  </section>

  <Modal
    v-model:open="storageState.entryOpen"
    :title="storageState.entryIsEdit ? '编辑数据' : '新增数据'"
    :ok-text="storageState.entryIsEdit ? '保存' : '新增'"
    cancel-text="取消"
    :confirm-loading="storageState.savingEntry"
    :width="640"
    @ok="submitEntry"
    @after-close="destroyStorageEditor"
  >
    <Form layout="vertical">
      <Form.Item label="Bucket" required>
        <Input
          v-model:value="storageState.entryForm.bucket"
          placeholder="存储桶名称，不存在时自动创建"
          :disabled="storageState.entryIsEdit"
        />
      </Form.Item>
      <Form.Item label="KEY" required>
        <Input
          v-model:value="storageState.entryForm.key"
          placeholder="请输入 KEY"
        />
      </Form.Item>
      <Form.Item label="VALUE">
        <div class="storage-value-head">
          <Radio.Group
            :value="storageState.editorMode"
            size="small"
            @change="setStorageEditorMode(($event.target as HTMLInputElement).value as 'text' | 'json')"
          >
            <Radio.Button value="text">文本</Radio.Button>
            <Radio.Button value="json">JSON</Radio.Button>
          </Radio.Group>
          <Button
            v-if="storageState.editorMode === 'json'"
            size="small"
            @click="formatStorageJson"
            >格式化 JSON</Button
          >
        </div>
        <div
          class="storage-editor"
          @keydown="handleStorageEditorKeydown"
        >
          <div ref="storageEditorHost" class="storage-editor-host" />
        </div>
        <div v-if="storageState.editorMode === 'json'" class="storage-value-hint">
          可使用 Shift + Alt + F，或点击「格式化 JSON」。
        </div>
      </Form.Item>
    </Form>
  </Modal>

  <Modal
    v-model:open="storageState.renameOpen"
    title="存储桶改名"
    ok-text="保存"
    cancel-text="取消"
    :confirm-loading="storageState.renaming"
    @ok="submitRenameBucket"
  >
    <Form layout="vertical">
      <Form.Item label="原名称">
        <Input :value="storageState.selected" disabled />
      </Form.Item>
      <Form.Item
        label="新名称"
        required
        extra="可包含点号（如 im.wc），不能包含逗号、斜杠或空白字符。"
      >
        <Input v-model:value="storageState.renameName" placeholder="例如：im.wc" />
      </Form.Item>
    </Form>
  </Modal>
</template>

<style scoped>
.storage-panel {
  display: flex;
  gap: 16px;
  align-items: stretch;
  /* 固定高度：分类目录与表格在自身内部滚动，不撑开整个页面（与插件开发页一致）。
     100vh − 顶栏 56 − content 上下内边距 18×2 − 面板边框 1×2 = 与左侧菜单栏底部对齐 */
  height: calc(100vh - 94px);
  height: calc(100dvh - 94px);
  min-height: 420px;
}
.storage-sidebar {
  width: 280px;
  flex: 0 0 280px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
  overflow: hidden;
}
.storage-sidebar-header {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px 10px 8px;
  border-bottom: 1px solid #edf0f5;
}
.storage-total {
  display: flex;
  align-items: center;
  gap: 8px;
}
.storage-total-title {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  font-size: 14px;
  font-weight: 700;
  color: #1f2937;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.storage-total-title svg {
  flex: 0 0 auto;
  color: #0958d9;
}
.storage-total-badge {
  margin-left: auto;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 20px;
  min-width: 32px;
  padding: 0 6px;
  border-radius: 6px;
  background: #f3f4f6;
  color: #374151;
  font-size: 11px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}
.storage-sidebar-actions {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
  width: 100%;
}
.storage-sidebar-actions > * {
  width: 100%;
  min-width: 0;
}
.storage-sidebar-actions :deep(.ant-btn) {
  width: 100%;
  height: 32px;
}
.storage-groups {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 10px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.storage-group {
  display: flex;
  flex-direction: column;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #fff;
  transition: border-color 0.2s ease;
}
.storage-group.expanded {
  border-color: #0958d966;
}
.storage-group-item {
  display: flex;
  width: 100%;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 10px;
  border: 0;
  border-radius: 8px;
  background: transparent;
  cursor: pointer;
  text-align: left;
  font-size: 14px;
  font-weight: 600;
  color: #1f2937;
  transition: background-color 0.15s ease;
}
.storage-group-item:hover {
  background: #f5f6f8;
}
.storage-group-item.active {
  background: #e6f4ff;
}
.storage-group-name {
  min-width: 0;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.storage-group-count {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 20px;
  min-width: 32px;
  padding: 0 6px;
  border-radius: 6px;
  background: #f3f4f6;
  color: #6b7280;
  font-size: 11px;
  font-weight: 500;
  font-variant-numeric: tabular-nums;
  flex: 0 0 auto;
}
.storage-group-count.has-count {
  background: #e6f4ff;
  color: #0958d9;
}
.storage-group-leading {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  flex: 1;
}
.storage-group-chevron-placeholder {
  width: 14px;
  flex: 0 0 auto;
}
.storage-group-chevron {
  flex: 0 0 auto;
  color: #9ca3af;
  transition: transform 0.2s ease;
}
.storage-group.expanded .storage-group-chevron {
  transform: rotate(90deg);
}
.storage-group-children {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 6px;
  border-top: 1px solid #f0f0f0;
}
.storage-group-child {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  min-width: 0;
  padding: 6px 8px;
  border: 1px solid transparent;
  border-radius: 6px;
  cursor: pointer;
  overflow: hidden;
  transition: background-color 0.15s ease;
}
.storage-group-child:hover {
  background: #f5f6f8;
}
.storage-group-child.active {
  background: #e6f4ff;
  border-color: #0958d933;
}
.storage-group-child-name {
  min-width: 0;
  flex: 1;
  font-size: 13px;
  font-weight: 600;
  color: #1f2937;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.storage-groups-empty {
  color: #94a3b8;
  padding: 12px 8px;
  font-size: 13px;
  text-align: center;
}
.storage-content {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  overflow: auto;
}
.storage-content-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}
.storage-content-title {
  font-size: 15px;
}
.storage-count {
  color: #94a3b8;
  font-size: 13px;
  margin-left: auto;
}
.storage-cell {
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.storage-value-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
}
.storage-editor-host {
  height: 320px;
  overflow: auto;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
}
.storage-editor-host :deep(.cm-editor) {
  height: 100%;
}
.storage-editor-host :deep(.cm-gutters) {
  color: #94a3b8;
  background: #f8fafc;
  border-right: 1px solid #eef1f6;
  user-select: none;
}
.storage-value-hint {
  margin-top: 8px;
  color: #94a3b8;
  font-size: 12px;
}
@media (max-width: 900px) {
  .storage-panel {
    flex-direction: column;
    height: auto;
    min-height: 0;
  }
  .storage-sidebar {
    width: auto;
    flex: none;
  }
  .storage-content {
    overflow: visible;
  }
}
</style>
