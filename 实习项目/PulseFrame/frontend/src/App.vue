<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ApiError, csrfToken, currentUser, login, logout, register, type User } from './api/auth'

const user = ref<User | null>(null)
const isRegistering = ref(false)
const username = ref('')
const password = ref('')
const passwordConfirmation = ref('')
const pending = ref(false)
const errorMessage = ref('')
const statusMessage = ref('')

onMounted(loadCurrentUser)

/**
 * 输入：无。
 * 输出：恢复会话用户；401 表示访客，其他接口错误显示为可理解提示。
 * 功能：在页面打开时依据服务端 Cookie 加载当前登录状态。
 */
async function loadCurrentUser(): Promise<void> {
  try {
    user.value = await currentUser()
  } catch (error: unknown) {
    if (error instanceof ApiError && error.status === 401) return
    errorMessage.value = messageFor(error)
  }
}

/**
 * 输入：当前表单中绑定的用户名、密码和确认密码。
 * 输出：注册用户或建立登录状态；请求失败时展示明确提示。
 * 功能：执行注册/登录流程，并保持注册后需显式登录的服务端契约。
 */
async function submitCredentials(): Promise<void> {
  if (pending.value) return
  errorMessage.value = ''
  statusMessage.value = ''
  if (isRegistering.value && Array.from(password.value).length < 8) {
    errorMessage.value = '密码至少需要 8 个字符。'
    return
  }
  if (isRegistering.value && password.value !== passwordConfirmation.value) {
    errorMessage.value = '两次输入的密码不一致。'
    return
  }

  pending.value = true
  try {
    if (isRegistering.value) {
      await register(username.value, password.value)
      isRegistering.value = false
      password.value = ''
      passwordConfirmation.value = ''
      statusMessage.value = '账号已创建，请登录。'
      return
    }
    user.value = await login(username.value, password.value)
    password.value = ''
    passwordConfirmation.value = ''
  } catch (error: unknown) {
    errorMessage.value = messageFor(error)
  } finally {
    pending.value = false
  }
}

/**
 * 输入：无。
 * 输出：切换注册与登录表单，并清除旧状态提示。
 * 功能：允许用户在两个账号入口间切换而不改变其凭据。
 */
function toggleMode(): void {
  isRegistering.value = !isRegistering.value
  errorMessage.value = ''
  statusMessage.value = ''
  password.value = ''
  passwordConfirmation.value = ''
}

/**
 * 输入：无。
 * 输出：成功时撤销当前会话并清除本地用户状态；失败时展示原因。
 * 功能：先读取会话 CSRF 令牌，再请求服务端注销当前设备。
 */
async function signOut(): Promise<void> {
  if (pending.value || user.value === null) return
  pending.value = true
  errorMessage.value = ''
  statusMessage.value = ''
  try {
    await logout(await csrfToken())
    user.value = null
    username.value = ''
    password.value = ''
    statusMessage.value = '已安全退出。'
  } catch (error: unknown) {
    if (error instanceof ApiError && error.status === 401) user.value = null
    errorMessage.value = messageFor(error)
  } finally {
    pending.value = false
  }
}

/**
 * 输入：error，接口、网络或本地执行过程中捕获的未知错误。
 * 输出：适合显示给用户的中文提示，不包含服务内部细节。
 * 功能：按稳定错误码解释常见认证失败并显式呈现其他故障。
 */
function messageFor(error: unknown): string {
  if (!(error instanceof ApiError)) return '暂时无法连接服务，请稍后重试。'
  switch (error.code) {
    case 'INVALID_CREDENTIALS': return '用户名或密码不正确。'
    case 'USERNAME_TAKEN': return '这个用户名已被使用。'
    case 'INVALID_REQUEST': return '用户名或密码格式不符合要求。'
    case 'RATE_LIMITED': return '尝试次数过多，请稍后再试。'
    case 'DEPENDENCY_UNAVAILABLE': return '认证服务暂时不可用，请稍后再试。'
    case 'CSRF_REJECTED': return '安全校验未通过，请刷新页面后重试。'
    default: return '请求未完成，请稍后重试。'
  }
}
</script>

<template>
  <main class="account-shell">
    <section class="account-panel" aria-label="PulseFrame 用户账号">
      <header class="brand">
        <span class="brand-symbol" aria-hidden="true">P</span>
        <span>PulseFrame</span>
      </header>

      <div class="account-content">
        <template v-if="user">
          <p class="eyebrow">个人账号</p>
          <h1>欢迎回来，<br /><span>{{ user.username }}</span></h1>
          <p class="intro">你已登录 PulseFrame。</p>
          <div class="profile-line">
            <span class="profile-avatar" aria-hidden="true">{{ user.username.slice(0, 1).toUpperCase() }}</span>
            <span>{{ user.username }}</span>
            <span class="active-indicator">已登录</span>
          </div>
          <button class="primary-button" type="button" :disabled="pending" @click="signOut">
            {{ pending ? '正在退出…' : '退出登录' }}
          </button>
        </template>

        <template v-else>
          <p class="eyebrow">{{ isRegistering ? '创建账号' : '账号登录' }}</p>
          <h1>{{ isRegistering ? '加入 PulseFrame' : '欢迎回来' }}</h1>
          <p class="intro">{{ isRegistering ? '建立你的创作者账号。' : '登录以继续。' }}</p>

          <form class="account-form" @submit.prevent="submitCredentials">
            <label for="username">用户名</label>
            <input
              id="username"
              v-model="username"
              aria-describedby="username-hint"
              :disabled="pending"
              autocomplete="username"
              autocapitalize="none"
              spellcheck="false"
              type="text"
              minlength="3"
              maxlength="32"
              pattern="[a-z0-9_]{3,32}"
              required
            />
            <p id="username-hint" class="field-hint">3–32 位小写字母、数字或下划线</p>

            <label for="password">密码</label>
            <input
              id="password"
              v-model="password"
              :disabled="pending"
              :autocomplete="isRegistering ? 'new-password' : 'current-password'"
              type="password"
              required
            />

            <template v-if="isRegistering">
              <label for="password-confirmation">确认密码</label>
              <input
                id="password-confirmation"
                v-model="passwordConfirmation"
                :disabled="pending"
                autocomplete="new-password"
                type="password"
                required
              />
            </template>

            <p v-if="errorMessage" class="feedback error-feedback" role="alert">{{ errorMessage }}</p>
            <p v-if="statusMessage" class="feedback status-feedback" role="status">{{ statusMessage }}</p>
            <button class="primary-button" type="submit" :disabled="pending">
              {{ pending ? '请稍候…' : (isRegistering ? '创建账号' : '登录') }}
            </button>
          </form>

          <button class="mode-switch" type="button" :disabled="pending" @click="toggleMode">
            {{ isRegistering ? '已有账号？返回登录' : '还没有账号？创建账号' }}
          </button>
        </template>
      </div>

      <footer class="panel-footer">
        <span>© 2026 PulseFrame</span>
        <span>用户账号</span>
      </footer>
    </section>

    <aside class="visual-panel" aria-label="创作者户外摄影现场">
      <img class="visual-image" src="/creator-capture.jpg" alt="创作者在山脊上记录户外景色" />
      <div class="visual-caption">
        <span>创作者现场</span>
        <span>01 / PULSEFRAME</span>
      </div>
    </aside>
  </main>
</template>
