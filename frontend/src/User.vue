<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import Alert from "ant-design-vue/es/alert";
import AntApp from "ant-design-vue/es/app";
import Avatar from "ant-design-vue/es/avatar";
import Button from "ant-design-vue/es/button";
import Card from "ant-design-vue/es/card";
import ConfigProvider from "ant-design-vue/es/config-provider";
import Empty from "ant-design-vue/es/empty";
import Form from "ant-design-vue/es/form";
import Input from "ant-design-vue/es/input";
import Space from "ant-design-vue/es/space";
import Tag from "ant-design-vue/es/tag";
import Typography from "ant-design-vue/es/typography";
import message from "ant-design-vue/es/message";
import zhCN from "ant-design-vue/es/locale/zh_CN";
import AppBrand from "./components/common/AppBrand.vue";
import { qqAvatarUrl } from "./utils";
import { Link, LogOut } from "lucide-vue-next";

type ApiEnvelope<T> = {
  status: boolean;
  message: string;
  data: T;
};

class UserRequestError extends Error {
  data: unknown;
  constructor(message: string, data: unknown) {
    super(message);
    this.data = data;
  }
}

type PublicUser = {
  id: string;
  username: string;
  nickname: string;
  email?: string;
  created_at: number;
};

type Bindings = {
  qq?: string;
  telegram?: string;
  updated_at?: number;
};

type UserAnnouncement = {
  enabled?: boolean;
  content?: string;
  format?: "text" | "markdown" | "html" | string;
};

type UserProfile = {
  user: PublicUser;
  bindings: Bindings;
  announcement?: UserAnnouncement;
};

const userAuthTokenKey = "sillygirl_user_jwt";
const loading = ref(true);
const user = ref<PublicUser | null>(null);
const bindings = reactive<Bindings>({});
const announcement = reactive<UserAnnouncement>({
  enabled: false,
  content: "",
});
const bindForm = reactive({
  qq: "",
  telegram: "",
});

const userInitial = computed(() => {
  const name = user.value?.nickname || user.value?.username || "U";
  return name.slice(0, 1).toUpperCase();
});

// 头像优先用 QQ 绑定号拉取 QQ 头像；无绑定但账号本身是 QQ 号（自助注册）同样可用。
const userAvatar = computed(() => {
  const username = user.value?.username || "";
  const qq = bindings.qq || (/^\d{5,12}$/.test(username) ? username : "");
  return qqAvatarUrl(qq);
});

const announcementVisible = computed(
  () => !!announcement.enabled && !!String(announcement.content || "").trim(),
);
const announcementHTML = computed(() =>
  renderAnnouncement(announcement.content || "", announcement.format || "text"),
);

function escapeHTML(value: string) {
  return String(value || "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

function inlineMarkdown(value: string) {
  return escapeHTML(value)
    .replace(/`([^`]+)`/g, "<code>$1</code>")
    .replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>")
    .replace(/\*([^*]+)\*/g, "<em>$1</em>")
    .replace(
      /\[([^\]]+)\]\((https?:\/\/[^)\s]+)\)/g,
      '<a href="$2" target="_blank" rel="noreferrer">$1</a>',
    );
}

function markdownToHTML(value: string) {
  const lines = String(value || "")
    .replace(/\r\n/g, "\n")
    .split("\n");
  const html: string[] = [];
  let inList = false;
  const closeList = () => {
    if (inList) {
      html.push("</ul>");
      inList = false;
    }
  };
  for (const line of lines) {
    const text = line.trim();
    if (!text) {
      closeList();
      html.push("<br>");
      continue;
    }
    const heading = text.match(/^(#{1,4})\s+(.+)$/);
    if (heading) {
      closeList();
      const level = heading[1].length;
      html.push(`<h${level}>${inlineMarkdown(heading[2])}</h${level}>`);
      continue;
    }
    const item = text.match(/^[-*]\s+(.+)$/);
    if (item) {
      if (!inList) {
        html.push("<ul>");
        inList = true;
      }
      html.push(`<li>${inlineMarkdown(item[1])}</li>`);
      continue;
    }
    closeList();
    html.push(`<p>${inlineMarkdown(text)}</p>`);
  }
  closeList();
  return html.join("");
}

const announcementAllowedTags = new Set([
  "A",
  "BLOCKQUOTE",
  "BR",
  "CODE",
  "DIV",
  "EM",
  "H1",
  "H2",
  "H3",
  "H4",
  "H5",
  "H6",
  "HR",
  "IMG",
  "LI",
  "OL",
  "P",
  "PRE",
  "SPAN",
  "STRONG",
  "UL",
]);

function safeAnnouncementURL(value: string, allowMailto = false) {
  const source = String(value || "").trim();
  if (!source) return "";
  try {
    const parsed = new URL(source, window.location.origin);
    if (
      parsed.protocol === "http:" ||
      parsed.protocol === "https:" ||
      (allowMailto && parsed.protocol === "mailto:")
    ) {
      return parsed.href;
    }
  } catch {
    // 无效地址交给调用方移除。
  }
  return "";
}

function sanitizeAnnouncementHTML(value: string) {
  const doc = new DOMParser().parseFromString(String(value || ""), "text/html");
  const blockedTags = new Set([
    "BASE",
    "EMBED",
    "FORM",
    "IFRAME",
    "MATH",
    "OBJECT",
    "SCRIPT",
    "STYLE",
    "SVG",
  ]);
  for (const element of Array.from(doc.body.querySelectorAll("*"))) {
    if (blockedTags.has(element.tagName)) {
      element.remove();
      continue;
    }
    if (!announcementAllowedTags.has(element.tagName)) {
      element.replaceWith(...Array.from(element.childNodes));
      continue;
    }
    for (const attribute of Array.from(element.attributes)) {
      const name = attribute.name.toLowerCase();
      const allowed =
        (element.tagName === "A" && ["href", "title"].includes(name)) ||
        (element.tagName === "IMG" &&
          ["src", "alt", "title", "width", "height"].includes(name));
      if (!allowed) element.removeAttribute(attribute.name);
    }
    if (element.tagName === "A") {
      const href = safeAnnouncementURL(
        element.getAttribute("href") || "",
        true,
      );
      if (href) element.setAttribute("href", href);
      else element.removeAttribute("href");
      element.setAttribute("target", "_blank");
      element.setAttribute("rel", "noopener noreferrer");
    } else if (element.tagName === "IMG") {
      const src = safeAnnouncementURL(element.getAttribute("src") || "");
      if (src) element.setAttribute("src", src);
      else element.remove();
    }
  }
  return doc.body.innerHTML;
}

function renderAnnouncement(content: string, format: string) {
  const mode = String(format || "text").toLowerCase();
  const html =
    mode === "html"
      ? String(content || "")
      : mode === "markdown" || mode === "md"
        ? markdownToHTML(content)
        : escapeHTML(content).replace(/\r?\n/g, "<br>");
  return sanitizeAnnouncementHTML(html);
}

async function requestJSON<T>(
  url: string,
  options: RequestInit = {},
): Promise<T> {
  const headers = new Headers(options.headers);
  if (!headers.has("Content-Type") && options.body) {
    headers.set("Content-Type", "application/json");
  }
  const token = localStorage.getItem(userAuthTokenKey)?.trim();
  if (token && !headers.has("token")) headers.set("token", token);
  const res = await fetch(url, {
    ...options,
    headers,
  });
  const payload = (await res.json().catch(() => ({
    status: false,
    message: "服务响应异常",
    data: null,
  }))) as ApiEnvelope<T>;
  if (
    !payload ||
    typeof payload !== "object" ||
    typeof payload.status !== "boolean" ||
    typeof payload.message !== "string" ||
    !("data" in payload)
  ) {
    throw new UserRequestError("服务响应格式错误", null);
  }
  if (!res.ok || payload.status === false) {
    throw new UserRequestError(payload.message || "请求失败", payload.data);
  }
  return payload.data;
}

function fillProfile(data: UserProfile) {
  user.value = data.user;
  Object.assign(bindings, data.bindings || {});
  Object.assign(
    announcement,
    data.announcement || { enabled: false, content: "" },
  );
  bindForm.qq = bindings.qq || "";
  bindForm.telegram = bindings.telegram || "";
}

async function loadProfile() {
  loading.value = true;
  try {
    const data = await requestJSON<UserProfile>("/api/user/profile");
    fillProfile(data);
  } catch (error) {
    localStorage.removeItem(userAuthTokenKey);
    user.value = null;
    message.error(error instanceof Error ? error.message : "请先登录");
  } finally {
    loading.value = false;
  }
}

async function logout() {
  try {
    await requestJSON<null>("/api/user/sessions/current/deletions", {
      method: "POST",
    });
  } catch (_) {
  } finally {
    localStorage.removeItem(userAuthTokenKey);
    window.location.href = "/";
  }
}

async function saveBinding(platform: "qq" | "telegram") {
  const value = platform === "qq" ? bindForm.qq : bindForm.telegram;
  const data = await requestJSON<Bindings>(`/api/user/bindings/${platform}`, {
    method: "POST",
    body: JSON.stringify({ value }),
  });
  Object.assign(bindings, data);
  message.success("绑定已保存");
}

async function removeBinding(platform: "qq" | "telegram") {
  const data = await requestJSON<Bindings>(
    `/api/user/bindings/${platform}/deletions`,
    {
      method: "POST",
    },
  );
  Object.assign(bindings, data);
  if (platform === "qq") bindForm.qq = "";
  if (platform === "telegram") bindForm.telegram = "";
  message.success("绑定已解除");
}

onMounted(() => {
  loadProfile();
});
</script>

<template>
  <ConfigProvider :locale="zhCN">
    <AntApp>
      <div class="user-page">
        <header class="user-topbar">
          <AppBrand class="user-brand" href="/" />
          <Space v-if="user" align="center">
            <Avatar :size="34" class="user-avatar" :src="userAvatar || undefined">{{ userAvatar ? "" : userInitial }}</Avatar>
            <span class="user-name">{{ user.nickname || user.username }}</span>
            <Button @click="logout"
              ><template #icon><LogOut :size="16" /></template>退出</Button
            >
          </Space>
        </header>

        <main class="user-content">
          <Card
            v-if="!loading && !user"
            class="user-login-card"
            :bordered="false"
          >
            <Empty description="请先登录普通用户账号" />
            <Button type="primary" href="/">返回登录</Button>
          </Card>

          <template v-if="user">
            <Alert
              v-if="announcementVisible"
              class="user-announcement"
              type="info"
              show-icon
              message="公告"
            >
              <template #description>
                <div
                  class="user-announcement-content"
                  v-html="announcementHTML"
                ></div>
              </template>
            </Alert>

            <section class="user-summary">
              <Card :bordered="false">
                <Space align="center">
                  <Avatar :size="56" class="user-avatar" :src="userAvatar || undefined">{{
                    userAvatar ? "" : userInitial
                  }}</Avatar>
                  <span>
                    <Typography.Title :level="3" class="user-title">{{
                      user.nickname || user.username
                    }}</Typography.Title>
                    <Typography.Text class="muted"
                      >@{{ user.username }}</Typography.Text
                    >
                    <Typography.Text
                      v-if="user.email"
                      class="muted user-email"
                      >{{ user.email }}</Typography.Text
                    >
                  </span>
                </Space>
              </Card>
              <Card :bordered="false">
                <Space direction="vertical" size="small">
                  <Typography.Text strong>绑定状态</Typography.Text>
                  <Space wrap>
                    <Tag :color="bindings.qq ? 'green' : 'default'"
                      >QQ {{ bindings.qq || "未绑定" }}</Tag
                    >
                    <Tag :color="bindings.telegram ? 'green' : 'default'"
                      >TG {{ bindings.telegram || "未绑定" }}</Tag
                    >
                  </Space>
                </Space>
              </Card>
            </section>

            <Card class="user-panel" :bordered="false">
              <template #title>
                <Space><Link :size="18" />账号绑定</Space>
              </template>
              <Form layout="vertical">
                <template v-if="bindings.qq">
                  <Form.Item label="QQ 号">
                    <Space class="bound-row">
                      <Typography.Text class="mono">{{
                        bindings.qq
                      }}</Typography.Text>
                      <Button @click="removeBinding('qq')">解绑</Button>
                    </Space>
                  </Form.Item>
                </template>
                <template v-else>
                  <Form.Item label="QQ 号">
                    <Input
                      v-model:value="bindForm.qq"
                      placeholder="例如：860562056"
                    />
                  </Form.Item>
                  <Space class="bind-actions">
                    <Button type="primary" @click="saveBinding('qq')"
                      >绑定 QQ</Button
                    >
                  </Space>
                </template>

                <template v-if="bindings.telegram">
                  <Form.Item label="Telegram ID" class="bind-field">
                    <Space class="bound-row">
                      <Typography.Text class="mono">{{
                        bindings.telegram
                      }}</Typography.Text>
                      <Button @click="removeBinding('telegram')"
                        >解绑</Button
                      >
                    </Space>
                  </Form.Item>
                </template>
                <template v-else>
                  <Form.Item label="Telegram ID" class="bind-field">
                    <Input
                      v-model:value="bindForm.telegram"
                      placeholder="例如：123456789"
                    />
                  </Form.Item>
                  <Space class="bind-actions">
                    <Button type="primary" @click="saveBinding('telegram')"
                      >绑定 TG</Button
                    >
                  </Space>
                </template>
              </Form>
            </Card>
          </template>
        </main>
      </div>
    </AntApp>
  </ConfigProvider>
</template>

<style scoped>
.user-page {
  min-height: 100vh;
  min-height: 100dvh;
  background: #f5f7fb;
}

.user-topbar {
  position: sticky;
  top: 0;
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: calc(56px + env(safe-area-inset-top, 0px));
  padding-top: env(safe-area-inset-top, 0px);
  padding-right: max(20px, env(safe-area-inset-right, 0px));
  padding-left: max(20px, env(safe-area-inset-left, 0px));
  background: #ffffff;
  border-bottom: 1px solid #edf0f5;
}

.user-brand {
  display: flex;
  align-items: center;
  gap: 10px;
  color: #1f2937;
  font-weight: 700;
}

.user-avatar {
  background: #111827;
  color: #ffffff;
}

.user-content {
  width: min(1180px, 100%);
  margin: 0 auto;
  padding: 18px max(16px, env(safe-area-inset-right, 0px))
    max(36px, env(safe-area-inset-bottom, 0px))
    max(16px, env(safe-area-inset-left, 0px));
}

.user-announcement {
  margin-bottom: 16px;
}

.user-announcement :deep(.ant-alert-description) {
  overflow-wrap: anywhere;
}

.user-announcement-content :deep(p) {
  margin: 0 0 8px;
}

.user-announcement-content :deep(p:last-child),
.user-announcement-content :deep(ul:last-child),
.user-announcement-content :deep(h1:last-child),
.user-announcement-content :deep(h2:last-child),
.user-announcement-content :deep(h3:last-child),
.user-announcement-content :deep(h4:last-child) {
  margin-bottom: 0;
}

.user-announcement-content :deep(ul) {
  padding-left: 20px;
  margin: 0 0 8px;
}

.user-announcement-content :deep(h1),
.user-announcement-content :deep(h2),
.user-announcement-content :deep(h3),
.user-announcement-content :deep(h4) {
  margin: 0 0 8px;
  color: #1f2937;
  font-weight: 700;
}

.user-announcement-content :deep(code) {
  padding: 1px 5px;
  border-radius: 4px;
  background: #e0f2fe;
  color: #075985;
}

.user-summary {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 420px;
  gap: 16px;
  margin-bottom: 16px;
}

.user-panel,
.user-summary :deep(.ant-card),
.user-login-card {
  border: 1px solid #edf0f5;
  border-radius: 8px;
}

.user-login-card {
  max-width: 420px;
  margin: 80px auto 0;
  text-align: center;
}

.user-title {
  margin: 0 !important;
}

.user-name {
  font-weight: 700;
}

.bind-actions {
  margin-bottom: 14px;
}

.bound-row {
  width: 100%;
  justify-content: space-between;
}

.bind-field {
  margin-top: 12px;
}

.muted {
  color: #6b7280;
}

.user-email {
  display: block;
  margin-top: 2px;
  font-size: 13px;
  word-break: break-all;
}

.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  word-break: break-all;
}

.mb0 {
  margin-bottom: 0;
}

@media (max-width: 920px) {
  .user-summary {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 560px) {
  .user-topbar {
    align-items: flex-start;
    flex-direction: column;
    gap: 10px;
    min-height: calc(56px + env(safe-area-inset-top, 0px));
    padding: max(12px, env(safe-area-inset-top, 0px))
      max(16px, env(safe-area-inset-right, 0px)) 12px
      max(16px, env(safe-area-inset-left, 0px));
  }

  .user-content {
    width: 100%;
    padding: 12px max(12px, env(safe-area-inset-right, 0px))
      max(24px, env(safe-area-inset-bottom, 0px))
      max(12px, env(safe-area-inset-left, 0px));
  }

  .user-panel :deep(.ant-card-body),
  .user-summary :deep(.ant-card-body),
  .user-login-card :deep(.ant-card-body) {
    padding: 16px;
  }

  .user-topbar :deep(.ant-space) {
    width: 100%;
  }

  .user-name {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .user-topbar :deep(.ant-btn),
  .user-content :deep(.ant-btn) {
    min-height: 44px;
  }
}

@media (max-height: 500px) and (orientation: landscape) {
  .user-topbar {
    position: relative;
  }
}
</style>
