<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue';
import AntApp from 'ant-design-vue/es/app';
import Avatar from 'ant-design-vue/es/avatar';
import Button from 'ant-design-vue/es/button';
import Card from 'ant-design-vue/es/card';
import ConfigProvider from 'ant-design-vue/es/config-provider';
import Space from 'ant-design-vue/es/space';
import Form from 'ant-design-vue/es/form';
import Input from 'ant-design-vue/es/input';
import Typography from 'ant-design-vue/es/typography';
import message from 'ant-design-vue/es/message';
import zhCN from 'ant-design-vue/es/locale/zh_CN';
import AppBrand from './components/common/AppBrand.vue';
import { qqAvatarUrl } from './utils';
import Collapse from 'ant-design-vue/es/collapse';
import { ArrowRight, Bot, CalendarClock, Mail, MessagesSquare, Puzzle, ShieldCheck, User, UserRoundCheck, Users } from 'lucide-vue-next';

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

const authMode = ref<'login' | 'register' | 'reset'>('login');
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
  code: '',
});
const resetForm = reactive({
  email: '',
  password: '',
  confirm: '',
  code: '',
});
const codeSending = ref(false);
const codeCountdown = ref(0);
let codeTimer: number | undefined;

function startCodeCountdown() {
  codeCountdown.value = 60;
  codeTimer = window.setInterval(() => {
    codeCountdown.value -= 1;
    if (codeCountdown.value <= 0) {
      window.clearInterval(codeTimer);
      codeTimer = undefined;
    }
  }, 1000);
}

async function requestCode(email: string, purpose: 'register' | 'password-reset') {
  if (!qqEmailPattern.test(email)) {
    message.error('请先输入正确的 QQ 邮箱（QQ号@qq.com）');
    return;
  }
  if (codeCountdown.value > 0 || codeSending.value) return;
  codeSending.value = true;
  try {
    await requestJSON<{ expires_in: number; resend_in: number }>(
      '/api/user/email-codes',
      {
        method: 'POST',
        body: JSON.stringify({ email, purpose }),
      },
    );
    message.success('验证码已发送，请查收邮箱');
    startCodeCountdown();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '发送失败');
  } finally {
    codeSending.value = false;
  }
}

function sendRegisterCode() {
  return requestCode(registerForm.email.trim().toLowerCase(), 'register');
}

function sendResetCode() {
  return requestCode(resetForm.email.trim().toLowerCase(), 'password-reset');
}

onUnmounted(() => {
  if (codeTimer !== undefined) window.clearInterval(codeTimer);
});

const qqEmailPattern = /^\d{5,12}@qq\.com$/;

const platforms = ['QQ', 'QQ 官方机器人', '微信', 'Telegram', '钉钉', '网页对话'];

const features = [
  { icon: MessagesSquare, title: '多平台接入', desc: '一套账号通用 QQ、QQ 官方机器人、微信、Telegram、钉钉与网页对话，在哪都在线。' },
  { icon: Puzzle, title: '插件生态', desc: '人工客服、消息推送、签到查询等能力按插件持续扩展，私聊发指令即可使用。' },
  { icon: CalendarClock, title: '定时任务与私聊推送', desc: '定时任务结果与管理员回复第一时间私聊送达，重要消息不错过。' },
  { icon: UserRoundCheck, title: 'QQ 号即账号', desc: 'QQ 邮箱一键注册，QQ 号就是账号并自动完成绑定，无需手动关联。' },
  { icon: Users, title: '群管理与入群服务', desc: '入群审批、群内指令与自动化管理，把群交给机器人打理。' },
  { icon: ShieldCheck, title: '安全与自托管', desc: '邮箱验证码注册、密码加密存储，数据由服务提供者自托管。' },
];

const steps = [
  { title: 'QQ 邮箱注册', desc: '填写 QQ号@qq.com 与密码，QQ 号即账号并自动绑定机器人。' },
  { title: '私聊机器人发指令', desc: '发送插件指令即可使用功能，例如发送「人工」提交问题反馈。' },
  { title: '接收回复与推送', desc: '管理员回复与定时任务结果会私聊推送给你，重要消息不错过。' },
];

const faqs = [
  { q: '如何绑定机器人？', a: '用 QQ 邮箱注册后，QQ 号会自动完成绑定，无需手动操作；直接私聊机器人发送指令即可使用。' },
  { q: '忘记密码怎么办？', a: '在登录页点击「忘记密码」，通过注册邮箱接收验证码即可自助重置。' },
  { q: '支持哪些聊天平台？', a: '目前支持 QQ、QQ 官方机器人、微信、Telegram、钉钉与网页对话，一套账号全平台通用。' },
  { q: '我的数据安全吗？', a: '密码加密存储，数据由服务提供者自托管，敏感操作需要邮箱验证码确认。' },
];

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
        code: registerForm.code.trim(),
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

async function resetPassword() {
  const email = resetForm.email.trim().toLowerCase();
  if (!qqEmailPattern.test(email)) {
    message.error('请先输入正确的 QQ 邮箱（QQ号@qq.com）');
    return;
  }
  if (!resetForm.code.trim()) {
    message.error('请输入邮箱验证码');
    return;
  }
  if (resetForm.password.length < 6) {
    message.error('密码至少 6 位');
    return;
  }
  if (resetForm.password !== resetForm.confirm) {
    message.error('两次输入的密码不一致');
    return;
  }
  loading.value = true;
  try {
    await requestJSON('/api/user/password/resets', {
      method: 'POST',
      body: JSON.stringify({
        email,
        code: resetForm.code.trim(),
        password: resetForm.password,
      }),
    });
    message.success('密码已重置，请使用新密码登录');
    authMode.value = 'login';
    loginForm.username = email.split('@')[0] || loginForm.username;
    loginForm.password = '';
    resetForm.code = '';
    resetForm.password = '';
    resetForm.confirm = '';
  } catch (error) {
    message.error(error instanceof Error ? error.message : '重置失败');
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
          <nav class="home-nav" aria-label="页面导航">
            <a href="#features">功能</a>
            <a href="#start">开始使用</a>
            <a href="#faq">常见问题</a>
          </nav>
          <div class="home-topbar-actions">
            <Button type="primary" href="#account">登录 / 注册</Button>
          </div>
        </header>

        <main>
          <section class="home-hero">
            <div class="home-shell home-hero-grid">
              <div class="home-hero-copy">
                <div class="home-badges">
                  <span class="home-badge home-badge-live"><i></i>服务运行中</span>
                  <span class="home-badge">QQ 号即账号</span>
                  <span class="home-badge">邮箱验证码保护</span>
                </div>
                <h1 class="home-title">
                  一个机器人<br />服务你所有的聊天平台
                </h1>
                <p class="home-lead">
                  QQ、QQ 官方机器人、微信、Telegram、钉钉、网页对话——注册一个账号，
                  私聊机器人即可使用插件功能、接收推送、管理社群。
                </p>
                <div class="home-hero-actions">
                  <Button type="primary" size="large" href="#account">
                    立即注册
                    <template #icon><ArrowRight :size="16" /></template>
                  </Button>
                  <Button size="large" href="#features">了解功能</Button>
                </div>
                <div class="home-chips">
                  <span v-for="plt in platforms" :key="plt" class="home-chip">{{ plt }}</span>
                </div>
              </div>
              <div class="home-demo" aria-hidden="true">
                <div class="chat-window">
                  <div class="chat-head">
                    <span class="chat-avatar"><Bot :size="18" /></span>
                    <div class="chat-head-text">
                      <strong>SillyGirl</strong>
                      <span><i class="chat-dot"></i>在线</span>
                    </div>
                  </div>
                  <div class="chat-body">
                    <div class="chat-row chat-user"><span class="chat-bubble">签到查询</span></div>
                    <div class="chat-row chat-bot"><span class="chat-bubble">今日已连续签到 3 天，积分 +10，明天再来～</span></div>
                    <div class="chat-row chat-user"><span class="chat-bubble">人工</span></div>
                    <div class="chat-row chat-bot"><span class="chat-bubble">已为你提交人工客服，管理员会在这里回复你。</span></div>
                    <div class="chat-row chat-push"><span class="chat-bubble">管理员已回复你的反馈</span></div>
                  </div>
                </div>
              </div>
            </div>
          </section>

          <section id="features" class="home-section">
            <div class="home-shell">
              <div class="home-section-head">
                <h2>机器人能做什么</h2>
                <p>核心能力开箱即用，更多功能按插件持续扩展。</p>
              </div>
              <div class="home-feature-grid">
                <article v-for="feature in features" :key="feature.title" class="home-feature">
                  <span class="home-feature-icon"><component :is="feature.icon" :size="20" /></span>
                  <h3>{{ feature.title }}</h3>
                  <p>{{ feature.desc }}</p>
                </article>
              </div>
            </div>
          </section>

          <section id="start" class="home-section home-start">
            <div class="home-shell home-start-grid">
              <div class="home-start-copy">
                <div class="home-section-head home-head-left">
                  <h2>三步开始使用</h2>
                  <p>全程自助，注册完成即可私聊使用。</p>
                </div>
                <ol class="home-steps">
                  <li v-for="(step, index) in steps" :key="step.title" class="home-step">
                    <span class="home-step-num">{{ index + 1 }}</span>
                    <div class="home-step-body">
                      <h3>{{ step.title }}</h3>
                      <p>{{ step.desc }}</p>
                    </div>
                  </li>
                </ol>
              </div>
              <aside id="account" class="home-auth">
                <Card :bordered="false">
                  <template #title>
                    <div class="home-auth-title">
                      <strong>普通用户</strong>
                      <span class="muted">注册或登录用户账号</span>
                    </div>
                  </template>

                  <div v-if="currentUser" class="home-user-card">
                    <div class="home-user-row">
                      <Avatar :size="48" class="home-avatar" :src="userAvatar || undefined">{{ userAvatar ? "" : userInitial }}</Avatar>
                      <span>
                        <Typography.Text strong>{{ currentUser.nickname || currentUser.username }}</Typography.Text>
                        <Typography.Text class="muted block">@{{ currentUser.username }}</Typography.Text>
                      </span>
                    </div>
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
                      <div class="home-auth-extra">
                        <button type="button" class="home-link" @click="authMode = 'reset'">忘记密码？</button>
                      </div>
                      <Button type="primary" block :loading="loading" @click="login">登录</Button>
                    </Form>

                    <Form v-else-if="authMode === 'register'" layout="vertical" @finish="register">
                      <Form.Item label="QQ 邮箱" required>
                        <Input id="home-register-email" v-model:value="registerForm.email" name="email" autocomplete="email" aria-label="注册邮箱" placeholder="QQ号@qq.com">
                          <template #prefix><Mail :size="16" /></template>
                        </Input>
                      </Form.Item>
                      <Form.Item label="邮箱验证码" required>
                        <Space.Compact style="width: 100%">
                          <Input
                            id="home-register-code"
                            v-model:value="registerForm.code"
                            name="one-time-code"
                            autocomplete="one-time-code"
                            aria-label="邮箱验证码"
                            placeholder="6 位验证码"
                            :maxlength="6"
                            @press-enter="sendRegisterCode"
                          />
                          <Button
                            :loading="codeSending"
                            :disabled="codeCountdown > 0"
                            @click="sendRegisterCode"
                          >
                            {{ codeCountdown > 0 ? `${codeCountdown}s 后重发` : '发送验证码' }}
                          </Button>
                        </Space.Compact>
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

                    <Form v-else layout="vertical" @finish="resetPassword">
                      <div class="home-auth-extra">
                        <button type="button" class="home-link" @click="authMode = 'login'">← 返回登录</button>
                      </div>
                      <Typography.Paragraph class="muted home-reset-tip">
                        输入注册邮箱获取验证码，重置后使用新密码登录。
                      </Typography.Paragraph>
                      <Form.Item label="注册邮箱" required>
                        <Input id="home-reset-email" v-model:value="resetForm.email" name="email" autocomplete="email" aria-label="注册邮箱" placeholder="QQ号@qq.com">
                          <template #prefix><Mail :size="16" /></template>
                        </Input>
                      </Form.Item>
                      <Form.Item label="邮箱验证码" required>
                        <Space.Compact style="width: 100%">
                          <Input
                            id="home-reset-code"
                            v-model:value="resetForm.code"
                            name="one-time-code"
                            autocomplete="one-time-code"
                            aria-label="邮箱验证码"
                            placeholder="6 位验证码"
                            :maxlength="6"
                            @press-enter="sendResetCode"
                          />
                          <Button
                            :loading="codeSending"
                            :disabled="codeCountdown > 0"
                            @click="sendResetCode"
                          >
                            {{ codeCountdown > 0 ? `${codeCountdown}s 后重发` : '发送验证码' }}
                          </Button>
                        </Space.Compact>
                      </Form.Item>
                      <Form.Item label="新密码" required>
                        <Input.Password id="home-reset-password" v-model:value="resetForm.password" name="new-password" autocomplete="new-password" aria-label="新密码" placeholder="至少 6 位" />
                      </Form.Item>
                      <Form.Item label="确认新密码" required>
                        <Input.Password
                          id="home-reset-confirm"
                          v-model:value="resetForm.confirm"
                          name="new-confirm-password"
                          autocomplete="new-password"
                          aria-label="确认新密码"
                          placeholder="再次输入新密码"
                          @press-enter="resetPassword"
                        />
                      </Form.Item>
                      <Button type="primary" block :loading="loading" @click="resetPassword">重置密码</Button>
                    </Form>
                  </template>
                </Card>
              </aside>
            </div>
          </section>

          <section id="faq" class="home-section home-section-alt">
            <div class="home-shell home-narrow">
              <div class="home-section-head">
                <h2>常见问题</h2>
                <p>还有疑问？直接私聊机器人问问看。</p>
              </div>
              <Collapse class="home-faq">
                <Collapse.Panel v-for="faq in faqs" :key="faq.q" :header="faq.q">
                  <p class="home-faq-answer">{{ faq.a }}</p>
                </Collapse.Panel>
              </Collapse>
            </div>
          </section>

          <section class="home-cta">
            <div class="home-shell home-cta-inner">
              <h2>准备好让机器人开工了吗？</h2>
              <p>QQ 邮箱注册，QQ 号即账号，注册完成即可私聊使用。</p>
              <Button size="large" href="#account" class="home-cta-btn">
                立即创建账号
                <template #icon><ArrowRight :size="16" /></template>
              </Button>
            </div>
          </section>
        </main>

        <footer class="home-footer">
          <div class="home-shell home-footer-inner">
            <AppBrand href="/" />
            <span class="home-footer-note">一个机器人，多平台贴心服务</span>
          </div>
        </footer>
      </div>
    </AntApp>
  </ConfigProvider>
</template>

<style scoped>
.home-page {
  min-height: 100vh;
  min-height: 100dvh;
  display: flex;
  flex-direction: column;
  background: #f5f7fb;
}

.home-shell {
  width: 100%;
  max-width: 1120px;
  margin: 0 auto;
  padding: 0 24px;
}

/* 顶栏 */
.home-topbar {
  position: sticky;
  top: 0;
  z-index: 20;
  display: flex;
  align-items: center;
  gap: 16px;
  min-height: calc(60px + env(safe-area-inset-top, 0px));
  padding: env(safe-area-inset-top, 0px) max(20px, env(safe-area-inset-right, 0px)) 0 max(20px, env(safe-area-inset-left, 0px));
  background: rgba(255, 255, 255, 0.86);
  backdrop-filter: blur(10px);
  border-bottom: 1px solid #edf0f5;
}
.home-topbar > * {
  padding-bottom: env(safe-area-inset-top, 0px);
  box-sizing: border-box;
}
.home-brand {
  color: #1f2937;
  font-weight: 700;
}
.home-nav {
  display: flex;
  gap: 22px;
  margin-left: 24px;
}
.home-nav a {
  color: #475569;
  font-size: 14px;
  text-decoration: none;
  transition: color 0.2s;
}
.home-nav a:hover {
  color: #1677ff;
}
.home-topbar-actions {
  margin-left: auto;
}

/* Hero */
.home-hero {
  padding: 72px 0 56px;
  background:
    radial-gradient(720px 320px at 12% -10%, rgba(22, 119, 255, 0.12), transparent 60%),
    radial-gradient(640px 300px at 92% 8%, rgba(82, 196, 26, 0.1), transparent 55%),
    linear-gradient(180deg, #f8faff 0%, #f5f7fb 100%);
  border-bottom: 1px solid #edf0f5;
}
.home-hero-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.05fr) minmax(0, 0.95fr);
  gap: 48px;
  align-items: center;
}
.home-badges {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 18px;
}
.home-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 12px;
  border-radius: 999px;
  border: 1px solid #dbe4f3;
  background: #ffffff;
  color: #475569;
  font-size: 12px;
  font-weight: 600;
}
.home-badge-live {
  color: #1677ff;
  border-color: #b8d3ff;
  background: #eff5ff;
}
.home-badge-live i {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #52c41a;
  box-shadow: 0 0 0 3px rgba(82, 196, 26, 0.15);
}
.home-title {
  margin: 0 0 16px;
  font-size: clamp(30px, 4.6vw, 46px);
  line-height: 1.18;
  font-weight: 800;
  color: #0f172a;
  letter-spacing: -0.5px;
}
.home-lead {
  margin: 0 0 24px;
  max-width: 520px;
  font-size: 16px;
  line-height: 1.75;
  color: #475569;
}
.home-hero-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 22px;
}
.home-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.home-chip {
  padding: 3px 10px;
  border-radius: 999px;
  background: rgba(22, 119, 255, 0.08);
  color: #1d4ed8;
  font-size: 12px;
  font-weight: 600;
}

/* 聊天演示窗口 */
.home-demo {
  display: flex;
  justify-content: center;
}
.chat-window {
  width: min(380px, 100%);
  border-radius: 18px;
  background: #ffffff;
  border: 1px solid #e6ebf4;
  box-shadow: 0 24px 60px -24px rgba(15, 23, 42, 0.28);
  overflow: hidden;
}
.chat-head {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px 16px;
  background: linear-gradient(135deg, #1677ff, #4c8dff);
  color: #ffffff;
}
.chat-avatar {
  display: grid;
  place-items: center;
  width: 34px;
  height: 34px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.2);
}
.chat-head-text {
  display: flex;
  flex-direction: column;
  line-height: 1.3;
}
.chat-head-text strong {
  font-size: 14px;
}
.chat-head-text span {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 12px;
  opacity: 0.92;
}
.chat-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #52e06a;
}
.chat-body {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 18px 16px 20px;
  background:
    radial-gradient(300px 140px at 90% 0%, rgba(22, 119, 255, 0.05), transparent 60%),
    #f8fafc;
  min-height: 264px;
}
.chat-row {
  display: flex;
}
.chat-row.chat-user {
  justify-content: flex-end;
}
.chat-row.chat-push {
  justify-content: center;
}
.chat-bubble {
  max-width: 82%;
  padding: 9px 13px;
  border-radius: 14px;
  font-size: 13px;
  line-height: 1.55;
}
.chat-user .chat-bubble {
  background: linear-gradient(135deg, #1677ff, #4c8dff);
  color: #ffffff;
  border-bottom-right-radius: 4px;
}
.chat-bot .chat-bubble {
  background: #ffffff;
  color: #1f2937;
  border: 1px solid #e6ebf4;
  border-bottom-left-radius: 4px;
}
.chat-push .chat-bubble {
  background: #fff7e6;
  color: #ad6800;
  border: 1px solid #ffe1a1;
  font-size: 12px;
}

/* 通用 section */
.home-section {
  padding: 64px 0;
}
.home-section-alt {
  background: #ffffff;
}
.home-section-head {
  text-align: center;
  margin-bottom: 36px;
}
.home-section-head h2 {
  margin: 0 0 10px;
  font-size: clamp(24px, 3vw, 32px);
  font-weight: 800;
  color: #0f172a;
  letter-spacing: -0.3px;
}
.home-section-head p {
  margin: 0;
  color: #64748b;
  font-size: 15px;
}
.home-head-left {
  text-align: left;
  margin-bottom: 24px;
}

/* 功能卡 */
.home-feature-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(280px, 100%), 1fr));
  gap: 16px;
}
.home-feature {
  padding: 22px 20px;
  border-radius: 16px;
  background: #ffffff;
  border: 1px solid #e6ebf4;
  transition: transform 0.2s, box-shadow 0.2s;
}
.home-feature:hover {
  transform: translateY(-3px);
  box-shadow: 0 16px 36px -18px rgba(15, 23, 42, 0.25);
}
.home-feature-icon {
  display: inline-grid;
  place-items: center;
  width: 42px;
  height: 42px;
  border-radius: 12px;
  background: rgba(22, 119, 255, 0.1);
  color: #1677ff;
  margin-bottom: 14px;
}
.home-feature h3 {
  margin: 0 0 8px;
  font-size: 16px;
  font-weight: 700;
  color: #0f172a;
}
.home-feature p {
  margin: 0;
  color: #64748b;
  font-size: 13.5px;
  line-height: 1.7;
}

/* 开始使用：三步 + 表单 */
.home-start {
  background: #ffffff;
  border-top: 1px solid #edf0f5;
  border-bottom: 1px solid #edf0f5;
}
.home-start-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 400px);
  gap: 56px;
  align-items: start;
}
.home-steps {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.home-step {
  position: relative;
  display: flex;
  gap: 16px;
  padding: 14px 0;
}
.home-step:not(:last-child)::before {
  content: "";
  position: absolute;
  left: 17px;
  top: 54px;
  bottom: -8px;
  width: 2px;
  background: #e2e8f0;
}
.home-step-num {
  flex: 0 0 auto;
  display: grid;
  place-items: center;
  width: 36px;
  height: 36px;
  border-radius: 50%;
  background: linear-gradient(135deg, #1677ff, #4c8dff);
  color: #ffffff;
  font-weight: 700;
  font-size: 15px;
}
.home-step-body h3 {
  margin: 4px 0 6px;
  font-size: 16px;
  font-weight: 700;
  color: #0f172a;
}
.home-step-body p {
  margin: 0;
  color: #64748b;
  font-size: 13.5px;
  line-height: 1.7;
}

/* 认证卡片 */
.home-auth {
  position: sticky;
  top: 84px;
}
.home-auth :deep(.ant-card) {
  border-radius: 16px;
  box-shadow: 0 20px 48px -24px rgba(15, 23, 42, 0.28);
}
.home-auth-title {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.home-auth-title strong {
  font-size: 16px;
  color: #0f172a;
}
.home-auth-title .muted {
  font-size: 12px;
  font-weight: 400;
}
.home-tabs {
  display: flex;
  gap: 6px;
  padding: 4px;
  border-radius: 12px;
  background: #f1f5f9;
  margin-bottom: 18px;
}
.home-tab {
  flex: 1;
  padding: 8px 0;
  border: 0;
  border-radius: 9px;
  background: transparent;
  color: #64748b;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
}
.home-tab.active {
  background: #ffffff;
  color: #1677ff;
  box-shadow: 0 1px 4px rgba(15, 23, 42, 0.12);
}
.home-auth-extra {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 12px;
}
.home-link {
  padding: 0;
  border: 0;
  background: none;
  color: #1677ff;
  font-size: 13px;
  cursor: pointer;
}
.home-reset-tip {
  margin: 0 0 12px;
  font-size: 13px;
}
.home-user-card {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.home-user-row {
  display: flex;
  align-items: center;
  gap: 12px;
}
.home-avatar {
  background: linear-gradient(135deg, #1677ff, #4c8dff);
  color: #ffffff;
  font-weight: 700;
}

/* FAQ */
.home-narrow {
  max-width: 760px;
}
.home-faq {
  background: transparent;
}
.home-faq :deep(.ant-collapse-item) {
  border-radius: 12px;
  border: 1px solid #e6ebf4;
  background: #ffffff;
  margin-bottom: 10px;
  overflow: hidden;
}
.home-faq-answer {
  margin: 0;
  color: #475569;
  line-height: 1.75;
}

/* CTA */
.home-cta {
  padding: 64px 0;
  background:
    radial-gradient(560px 240px at 20% 0%, rgba(255, 255, 255, 0.14), transparent 60%),
    linear-gradient(135deg, #1663ff, #4c8dff);
  color: #ffffff;
}
.home-cta-inner {
  text-align: center;
}
.home-cta-inner h2 {
  margin: 0 0 10px;
  font-size: clamp(24px, 3.4vw, 34px);
  font-weight: 800;
  letter-spacing: -0.3px;
}
.home-cta-inner p {
  margin: 0 0 24px;
  opacity: 0.92;
  font-size: 15px;
}
.home-cta-btn.ant-btn {
  background: #ffffff;
  color: #1663ff;
  border-color: #ffffff;
  font-weight: 700;
}
.home-cta-btn.ant-btn:hover {
  background: #f0f6ff;
  color: #1663ff;
  border-color: #f0f6ff;
}

/* 页脚 */
.home-footer {
  margin-top: auto;
  padding: 24px 0 calc(24px + env(safe-area-inset-bottom, 0px));
  background: #ffffff;
  border-top: 1px solid #edf0f5;
}
.home-footer-inner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}
.home-footer-note {
  color: #94a3b8;
  font-size: 13px;
}
.home-footer .home-brand {
  font-size: 14px;
}

.muted {
  color: #94a3b8;
}
.block {
  display: block;
}

/* 平板 */
@media (max-width: 960px) {
  .home-nav {
    display: none;
  }
  .home-hero {
    padding: 48px 0 48px;
  }
  .home-hero-grid {
    grid-template-columns: 1fr;
    gap: 36px;
  }
  .home-demo {
    justify-content: flex-start;
  }
  .home-start-grid {
    grid-template-columns: 1fr;
    gap: 36px;
  }
  .home-auth {
    position: static;
  }
  .home-section {
    padding: 48px 0;
  }
}

/* 手机 */
@media (max-width: 600px) {
  .home-shell {
    padding: 0 16px;
  }
  .home-hero {
    padding: 36px 0 40px;
  }
  .home-hero-actions :deep(.ant-btn) {
    flex: 1;
  }
  .home-title {
    font-size: 28px;
  }
  .home-section {
    padding: 40px 0;
  }
  .home-cta {
    padding: 48px 0;
  }
  .home-cta-inner .home-cta-btn.ant-btn {
    width: 100%;
  }
  .home-topbar-actions {
    display: none;
  }
  .home-nav {
    display: flex;
    margin-left: auto;
    gap: 14px;
  }
  .home-nav a {
    font-size: 13px;
  }
}
</style>
