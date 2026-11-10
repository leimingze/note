// 脚本功能：维护登录状态，封装登录、注册、恢复会话与退出操作。
// 启动命令：由 App.vue 调用，无需单独启动。
import { reactive, readonly } from 'vue'
import { api } from './api'

const state = reactive({
  ready: false,
  signedIn: false,
  busy: false,
  message: '',
})

function friendlyError(err) {
  const map = {
    INVALID_CREDENTIALS: '用户名或密码错误',
    ACCOUNT_DISABLED: '账号已禁用',
    LOGIN_EXPIRED: '登录已过期，请重新登录',
    TOO_MANY_REQUESTS: '操作过于频繁，请稍后再试',
    RESOURCE_NOT_FOUND: '服务暂不可用',
  }
  return map[err.code] || err.message || '请求失败，请稍后再试'
}

export function useAuth() {
  return readonly(state)
}

export async function restore() {
  try {
    await api.refresh()
    state.signedIn = true
  } catch {
    state.signedIn = false
  } finally {
    state.ready = true
  }
}

export async function login(form) {
  state.busy = true
  state.message = ''
  try {
    await api.login(form)
    state.signedIn = true
  } catch (err) {
    state.message = friendlyError(err)
  } finally {
    state.busy = false
  }
}

export async function register(form) {
  state.busy = true
  state.message = ''
  try {
    await api.register(form)
    return true
  } catch (err) {
    state.message = friendlyError(err)
    return false
  } finally {
    state.busy = false
  }
}

export async function logout() {
  try {
    await api.logout()
  } finally {
    state.signedIn = false
  }
}
