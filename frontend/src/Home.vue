<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue';
import AntApp from 'ant-design-vue/es/app';
import Avatar from 'ant-design-vue/es/avatar';
import Button from 'ant-design-vue/es/button';
import Card from 'ant-design-vue/es/card';
import Col from 'ant-design-vue/es/col';
import ConfigProvider from 'ant-design-vue/es/config-provider';
import Form from 'ant-design-vue/es/form';
import Input from 'ant-design-vue/es/input';
import Row from 'ant-design-vue/es/row';
import Space from 'ant-design-vue/es/space';
import Tag from 'ant-design-vue/es/tag';
import Typography from 'ant-design-vue/es/typography';
import message from 'ant-design-vue/es/message';
import zhCN from 'ant-design-vue/es/locale/zh_CN';
import AppBrand from './components/common/AppBrand.vue';
import { qqAvatarUrl } from './utils';
import { Bell, Bot, Mail, Puzzle, User, UserRoundCheck } from 'lucide-vue-next';

type ApiEnvelope<T> = {
  status: boolean;
  message: string;
  data: T;
};

type PublicUser = {
  id: string;
  username: string;
  nickname: string;
  created_at: number;
};

type AuthPayload = {
  token: string;
  expiresIn: number;
  user: PublicUser;
};

const authMode = ref<'login' | 'register'>('login');
const loading = ref(false);
const currentUser = ref<PublicUser | null>(null);
const userAuthTokenKey = 'sillygirl_user_jwt';
const loginForm = reactive({
  username: '',
  password: '',
});
const registerForm = reactive({
  email: '',
  nickname: '',
  password: '',
  confirm: '',
});

const qqEmailPattern = /^\d{5,12}@qq\.com$/;

const userInitial = computed(() => {
  const name = currentUser.value?.nickname || currentUser.value?.username || 'U';
  return name.slice(0, 1).toUpperCase();
});

const userAvatar = computed(() => {
  const username = currentUser.value?.username || '';
  return qqAvatarUrl(/^\d{5,12}$/.test(username) ? username : '');
});

async function requestJSON<T>(url: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers);
  if (!headers.has('Content-Type') && options.body) {
    headers.set('Content-Type', 'application/json');
  }
  const token = localStorage.getItem(userAuthTokenKey)?.trim();
  if (token && !headers.has('token')) headers.set('token', token);
  const res = await fetch(url, {
    ...options,
    headers,
  });
  const payload = (await res.json().catch(() => ({
    status: false,
    message: '服务响应异常',
    data: null,
  }))) as ApiEnvelope<T>;
  if (
    !payload ||
    typeof payload !== 'object' ||
    typeof payload.status !== 'boolean' ||
    typeof payload.message !== 'string' ||
    !('data' in payload)
  ) {
    throw new Error('服务响应格式错误');
  }
  if (!res.ok || payload.status === false) {
    throw new Error(payload.message || '请求失败');
  }
  return payload.data;
}

function setAuth(data: AuthPayload) {
  localStorage.setItem(userAuthTokenKey, data.token.trim());
  currentUser.value = data.user;
}

function clearAuth() {
  currentUser.value = null;
  localStorage.removeItem(userAuthTokenKey);
}

async function login() {
  loading.value = true;
  try {
    const data = await requestJSON<AuthPayload>('/api/user/sessions', {
      method: 'POST',
      body: JSON.stringify({
        username: loginForm.username.trim(),
        password: loginForm.password,
      }),
    });
    setAuth(data);
    message.success('登录成功');
    window.location.href = '/user';
  } catch (error) {
    message.error(error instanceof Error ? error.message : '登录失败');
  } finally {
    loading.value = false;
  }
}

async function register() {
  const email = registerForm.email.trim().toLowerCase();
  if (!qqEmailPattern.test(email)) {
    message.error('请输入正确的 QQ 邮箱（QQ号@qq.com）');
    return;
  }
  if (registerForm.password.length < 6) {
    message.error('密码至少 6 位');
    return;
  }
  if (registerForm.password !== registerForm.confirm) {
    message.error('两次输入的密码不一致');
    return;
  }
  loading.value = true;
  try {
    const data = await requestJSON<AuthPayload>('/api/user/accounts', {
      method: 'POST',
      body: JSON.stringify({
        email,
        nickname: registerForm.nickname.trim(),
        password: registerForm.password,
      }),
    });
    setAuth(data);
    message.success('注册成功');
    window.location.href = '/user';
  } catch (error) {
    message.error(error instanceof Error ? error.message : '注册失败');
  } finally {
    loading.value = false;
  }
}

async function logout() {
  try {
    await requestJSON<null>('/api/user/sessions/current/deletions', {
      method: 'POST',
    });
  } catch (_) {
  } finally {
    clearAuth();
    authMode.value = 'login';
    message.success('已退出登录');
  }
}

async function loadCurrentUser() {
  try {
    const data = await requestJSON<{ user: PublicUser }>('/api/user/profile');
    currentUser.value = data.user;
  } catch (_) {
    clearAuth();
  }
}

onMounted(() => {
  loadCurrentUser();
});
</script>

<template>
  <ConfigProvider :locale="zhCN">
    <AntApp>
      <div class="home-page">
        <header class="home-topbar">
          <AppBrand class="home-brand" href="/" />
          <Space wrap>
            <Button type="primary" href="#account">用户入口</Button>
          </Space>
        </header>

        <main class="home-content">
          <div class="home-layout">
            <section class="home-main">
              <Card class="home-hero" :bordered="false">
                <Space wrap>
                  <Tag color="blue">用户服务入口</Tag>
                  <Tag color="green">服务运行中</Tag>
                  <Tag>QQ 号即账号</Tag>
                </Space>
                <Typography.Title :level="1" class="home-title">一个机器人，多平台贴心服务</Typography.Title>
                <Typography.Paragraph class="home-lead">
                  SillyGirl 是接入 QQ、QQ 官方机器人、Telegram、钉钉、微信和网页对话的多平台机器人。注册账号并绑定 QQ 后，
                  私聊机器人即可使用插件功能，管理员回复与定时任务结果也会第一时间私聊推送给你。
                </Typography.Paragraph>
                <Space wrap>
                  <Button type="primary" href="#account">登录或注册</Button>
                </Space>
              </Card>

              <Card class="home-panel" :bordered="false">
                <div class="home-toolbar">
                  <Space size="small">
                    <Bot :size="16" />
                    <Typography.Text strong>机器人能力</Typography.Text>
                    <Typography.Text class="muted">按插件持续扩展，注册后即可使用</Typography.Text>
                  </Space>
                </div>
                <Row :gutter="[12, 12]">
                  <Col :xs="24" :md="12">
                    <Card size="small">
                      <template #title><Space size="small"><Bot :size="16" />多平台接入</Space></template>
                      <Typography.Paragraph class="muted mb0">QQ、QQ 官方机器人、Telegram、钉钉、微信、网页对话，一套账号全平台通用。</Typography.Paragraph>
                    </Card>
                  </Col>
                  <Col :xs="24" :md="12">
                    <Card size="small">
                      <template #title><Space size="small"><Puzzle :size="16" />插件功能</Space></template>
                      <Typography.Paragraph class="muted mb0">人工客服留言、消息推送、群管理与入群服务、查询签到等能力，私聊机器人发指令即可使用。</Typography.Paragraph>
                    </Card>
                  </Col>
                  <Col :xs="24" :md="12">
                    <Card size="small">
                      <template #title><Space size="small"><Bell :size="16" />定时任务与推送</Space></template>
                      <Typography.Paragraph class="muted mb0">定时任务结果与管理员回复私聊推送，重要消息不错过。</Typography.Paragraph>
                    </Card>
                  </Col>
                  <Col :xs="24" :md="12">
                    <Card size="small">
                      <template #title><Space size="small"><UserRoundCheck :size="16" />QQ 号即账号</Space></template>
                      <Typography.Paragraph class="muted mb0">QQ 邮箱一键注册，QQ 号就是账号并自动完成绑定，机器人凭绑定识别你的身份。</Typography.Paragraph>
                    </Card>
                  </Col>
                </Row>
              </Card>

              <Card class="home-panel" :bordered="false">
                <div class="home-toolbar">
                  <Typography.Text strong>使用流程</Typography.Text>
                </div>
                <div class="home-link-list">
                  <div class="home-link-row">
                    <span class="home-link-icon">1</span>
                    <span class="home-link-main">
                      <Typography.Text strong>QQ 邮箱注册</Typography.Text>
                      <Typography.Text class="muted">填写 QQ号@qq.com 与密码，QQ 号即账号并自动绑定，无需重复绑定。</Typography.Text>
                    </span>
                    <Tag>注册</Tag>
                  </div>
                  <div class="home-link-row">
                    <span class="home-link-icon">2</span>
                    <span class="home-link-main">
                      <Typography.Text strong>私聊机器人发指令</Typography.Text>
                      <Typography.Text class="muted">给机器人发送插件指令即可使用功能，例如发送「人工」提交问题反馈。</Typography.Text>
                    </span>
                    <Tag color="blue">指令</Tag>
                  </div>
                  <div class="home-link-row">
                    <span class="home-link-icon">3</span>
                    <span class="home-link-main">
                      <Typography.Text strong>接收回复与推送</Typography.Text>
                      <Typography.Text class="muted">管理员回复、定时任务结果会私聊推送给你，用户中心可查看账号与绑定状态。</Typography.Text>
                    </span>
                    <Tag color="green">推送</Tag>
                  </div>
                </div>
              </Card>
            </section>

            <aside id="account" class="home-auth">
              <Card :bordered="false">
                <template #title>
                  <Space direction="vertical" size="small">
                    <Typography.Text strong>普通用户</Typography.Text>
                    <Typography.Text class="muted">注册或登录用户账号</Typography.Text>
                  </Space>
                </template>

                <div v-if="currentUser" class="home-user-card">
                  <Space align="center">
                    <Avatar :size="48" class="home-avatar" :src="userAvatar || undefined">{{ userAvatar ? "" : userInitial }}</Avatar>
                    <span>
                      <Typography.Text strong>{{ currentUser.nickname || currentUser.username }}</Typography.Text>
                      <Typography.Text class="muted block">@{{ currentUser.username }}</Typography.Text>
                    </span>
                  </Space>
                  <Button type="primary" block href="/user">进入用户中心</Button>
                  <Button block @click="logout">退出登录</Button>
                </div>

                <template v-else>
                  <div class="home-tabs">
                    <button type="button" class="home-tab" :class="{ active: authMode === 'login' }" @click="authMode = 'login'">登录</button>
                    <button type="button" class="home-tab" :class="{ active: authMode === 'register' }" @click="authMode = 'register'">注册</button>
                  </div>

                  <Form v-if="authMode === 'login'" layout="vertical" @finish="login">
                    <Form.Item label="账号（QQ 号）" required>
                      <Input id="home-login-username" v-model:value="loginForm.username" name="username" autocomplete="username" aria-label="登录账号" placeholder="请输入 QQ 号">
                        <template #prefix><User :size="16" /></template>
                      </Input>
                    </Form.Item>
                    <Form.Item label="密码" required>
                      <Input.Password id="home-login-password" v-model:value="loginForm.password" name="password" autocomplete="current-password" aria-label="登录密码" placeholder="请输入密码" />
                    </Form.Item>
                    <Button type="primary" block :loading="loading" @click="login">登录</Button>
                  </Form>

                  <Form v-else layout="vertical" @finish="register">
                    <Form.Item label="QQ 邮箱" required>
                      <Input id="home-register-email" v-model:value="registerForm.email" name="email" autocomplete="email" aria-label="注册邮箱" placeholder="QQ号@qq.com">
                        <template #prefix><Mail :size="16" /></template>
                      </Input>
                    </Form.Item>
                    <Form.Item label="昵称">
                      <Input id="home-register-nickname" v-model:value="registerForm.nickname" name="name" autocomplete="name" aria-label="注册昵称" placeholder="不填则使用邮箱" />
                    </Form.Item>
                    <Form.Item label="密码" required>
                      <Input.Password id="home-register-password" v-model:value="registerForm.password" name="new-password" autocomplete="new-password" aria-label="注册密码" placeholder="至少 6 位" />
                    </Form.Item>
                    <Form.Item label="确认密码" required>
                      <Input.Password
                        id="home-register-confirm"
                        v-model:value="registerForm.confirm"
                        name="new-confirm-password"
                        autocomplete="new-password"
                        aria-label="确认密码"
                        placeholder="再次输入密码"
                        @press-enter="register"
                      />
                    </Form.Item>
                    <Button type="primary" block :loading="loading" @click="register">创建账号</Button>
                  </Form>
                </template>

                <Typography.Paragraph class="home-auth-tip muted">
                  使用 QQ 邮箱注册，QQ 号即账号并自动绑定；登录后机器人按绑定识别你的身份。
                </Typography.Paragraph>
              </Card>
            </aside>
          </div>
        </main>
      </div>
    </AntApp>
  </ConfigProvider>
</template>

<style scoped>
.home-page {
  min-height: 100vh;
  min-height: 100dvh;
  background: #f5f7fb;
}

.home-topbar {
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

.home-brand {
  display: flex;
  align-items: center;
  gap: 10px;
  color: #1f2937;
  font-weight: 700;
  letter-spacing: 0;
}

.home-content {
  width: min(1180px, 100%);
  margin: 0 auto;
  padding: 18px max(16px, env(safe-area-inset-right, 0px))
    max(36px, env(safe-area-inset-bottom, 0px))
    max(16px, env(safe-area-inset-left, 0px));
}

.home-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 380px;
  gap: 18px;
  align-items: start;
}

.home-main {
  min-width: 0;
}

.home-hero,
.home-panel,
.home-auth :deep(.ant-card) {
  border: 1px solid #edf0f5;
  border-radius: 8px;
}

.home-hero {
  min-height: 256px;
  display: grid;
  align-content: center;
}

.home-hero :deep(.ant-card-body) {
  display: grid;
  gap: 14px;
}

.home-title {
  max-width: 760px;
  margin: 0 !important;
  font-size: clamp(30px, 5vw, 48px) !important;
  line-height: 1.12 !important;
  letter-spacing: 0 !important;
}

.home-lead {
  max-width: 760px;
  color: #6b7280;
  font-size: 16px;
  line-height: 1.75;
}

.home-panel {
  margin-top: 18px;
}

.home-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.home-link-list {
  display: grid;
  gap: 8px;
}

.home-link-row {
  display: grid;
  grid-template-columns: 34px minmax(0, 1fr) auto;
  gap: 10px;
  align-items: center;
  min-height: 54px;
  padding: 9px 10px;
  color: #1f2937;
  background: #f8fafc;
  border: 1px solid #edf0f5;
  border-radius: 8px;
}

.home-link-row:hover {
  color: #1677ff;
  border-color: #91caff;
}

.home-link-icon {
  display: grid;
  place-items: center;
  width: 34px;
  height: 34px;
  color: #1677ff;
  background: #e6f4ff;
  border-radius: 8px;
  font-weight: 700;
}

.home-link-main {
  display: grid;
  gap: 3px;
  min-width: 0;
}

.home-auth {
  position: sticky;
  top: 74px;
  scroll-margin-top: calc(74px + env(safe-area-inset-top, 0px));
}

.home-tabs {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 6px;
  padding: 4px;
  margin-bottom: 16px;
  background: #f5f7fb;
  border: 1px solid #edf0f5;
  border-radius: 8px;
}

.home-tab {
  min-height: 34px;
  border: 0;
  border-radius: 6px;
  color: #6b7280;
  background: transparent;
  cursor: pointer;
}

.home-tab.active {
  color: #1f2937;
  background: #ffffff;
  box-shadow: 0 2px 8px rgba(15, 23, 42, 0.06);
  font-weight: 700;
}

.home-user-card {
  display: grid;
  gap: 16px;
}

.home-avatar {
  background: #111827;
}

.muted {
  color: #6b7280;
}

.block {
  display: block;
}

.mb0 {
  margin-bottom: 0;
}

@media (max-width: 920px) {
  .home-layout {
    grid-template-columns: 1fr;
  }

  .home-auth {
    position: static;
  }
}

@media (max-width: 560px) {
  .home-topbar {
    align-items: flex-start;
    flex-direction: column;
    gap: 10px;
    min-height: calc(56px + env(safe-area-inset-top, 0px));
    padding: max(12px, env(safe-area-inset-top, 0px))
      max(16px, env(safe-area-inset-right, 0px)) 12px
      max(16px, env(safe-area-inset-left, 0px));
  }

  .home-topbar :deep(.ant-space) {
    width: 100%;
  }

  .home-topbar :deep(.ant-space-item) {
    flex: 1;
  }

  .home-topbar :deep(.ant-btn) {
    width: 100%;
    min-height: 44px;
  }

  .home-content {
    width: 100%;
    padding: 12px max(12px, env(safe-area-inset-right, 0px))
      max(24px, env(safe-area-inset-bottom, 0px))
      max(12px, env(safe-area-inset-left, 0px));
  }

  .home-hero {
    min-height: auto;
  }

  .home-hero :deep(.ant-card-body),
  .home-panel :deep(.ant-card-body),
  .home-auth :deep(.ant-card-body) {
    padding: 16px;
  }

  .home-title {
    font-size: clamp(28px, 10vw, 38px) !important;
  }

  .home-tab,
  .home-content :deep(.ant-btn),
  .home-content :deep(.ant-input),
  .home-content :deep(.ant-input-affix-wrapper) {
    min-height: 44px;
  }

  .home-link-row {
    grid-template-columns: 34px minmax(0, 1fr);
  }

  .home-link-row :deep(.ant-tag) {
    grid-column: 2;
    width: max-content;
  }
}

@media (max-height: 500px) and (orientation: landscape) {
  .home-topbar {
    position: relative;
  }
}
</style>
