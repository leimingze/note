// 脚本功能：提供统一的后端请求封装，自动附带令牌并在 401 时刷新后重试。
// 启动命令：由 auth.js 与视图组件调用，无需单独启动。

let token = null

async function refreshSession() {
  const res = await fetch('/api/refresh', { method: 'POST', credentials: 'include' })
  if (res.status !== 200) return false
  const data = await res.json()
  token = data.accessToken
  return true
}

async function parseResponse(res) {
  if (res.status === 204) return null
  const text = await res.text()
  const data = text ? JSON.parse(text) : null
  if (!res.ok) {
    const err = new Error((data && data.message) || `HTTP ${res.status}`)
    err.status = res.status
    err.code = data && data.code
    throw err
  }
  return data
}

export async function request(path, options = {}) {
  const headers = options.headers ? { ...options.headers } : {}
  if (token) headers.Authorization = `Bearer ${token}`
  if (options.body != null) headers['Content-Type'] = 'application/json'
  const init = {
    method: options.method || 'GET',
    headers,
    credentials: 'include',
    body: options.body == null ? undefined : JSON.stringify(options.body),
  }

  let res = await fetch('/api' + path, init)
  if (res.status === 401 && !options.skipRefresh && (await refreshSession())) {
    headers.Authorization = `Bearer ${token}`
    res = await fetch('/api' + path, { ...init, headers })
  }
  return parseResponse(res)
}

export const api = {
  async register(body) {
    return request('/register', { method: 'POST', body })
  },
  async login(body) {
    const data = await request('/login', { method: 'POST', body })
    token = data.accessToken
    return data
  },
  async refresh() {
    const data = await request('/refresh', { method: 'POST', skipRefresh: true })
    token = data.accessToken
    return data
  },
  async logout() {
    await request('/logout', { method: 'POST', skipRefresh: true })
    token = null
  },
}

export function clearToken() {
  token = null
}
