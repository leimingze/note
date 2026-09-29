export interface User {
  id: number
  username: string
}

export class ApiError extends Error {
  /**
   * 输入：status、code 和 message，分别为 HTTP 状态、稳定错误码与服务端错误说明。
   * 输出：可供界面按错误码映射提示语的异常对象。
   * 功能：保留认证接口的机器可读错误信息，避免界面解析错误文本。
   */
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

/**
 * 输入：username、password，未经客户端改写的注册凭据。
 * 输出：创建成功的公开用户资料；接口失败或响应结构非法时抛出错误。
 * 功能：调用注册接口，不保存或回传密码。
 */
export async function register(username: string, password: string): Promise<User> {
  const response = await request('/register', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  })
  return parseUser(await readJson(response))
}

/**
 * 输入：username、password，未经客户端改写的登录凭据。
 * 输出：登录成功的公开用户资料；接口失败或响应结构非法时抛出错误。
 * 功能：通过 HttpOnly Cookie 建立网页会话，不在脚本中接收会话令牌。
 */
export async function login(username: string, password: string): Promise<User> {
  const response = await request('/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  })
  return parseUser(await readJson(response))
}

/**
 * 输入：无。
 * 输出：当前用户资料；未登录或接口不可用时抛出错误。
 * 功能：使用浏览器会话 Cookie 恢复已登录状态。
 */
export async function currentUser(): Promise<User> {
  const response = await request('/me')
  return parseUser(await readJson(response))
}

/**
 * 输入：无。
 * 输出：当前会话绑定的 CSRF 令牌；认证失败或响应结构非法时抛出错误。
 * 功能：为网页状态修改请求读取不可缓存的会话令牌。
 */
export async function csrfToken(): Promise<string> {
  const response = await request('/csrf')
  const value = await readJson(response)
  if (!isRecord(value) || typeof value.csrf_token !== 'string' || !/^[0-9a-f]{64}$/.test(value.csrf_token)) {
    throw new Error('认证服务返回了无效的 CSRF 令牌。')
  }
  return value.csrf_token
}

/**
 * 输入：token，当前会话获取的 64 位小写十六进制 CSRF 令牌。
 * 输出：服务端撤销当前会话后正常结束；请求失败时抛出错误。
 * 功能：注销当前浏览器会话并让服务端清除 HttpOnly Cookie。
 */
export async function logout(token: string): Promise<void> {
  await request('/logout', {
    method: 'POST',
    headers: { 'X-CSRF-Token': token },
  })
}

/**
 * 输入：path，认证 API 的相对路径；init，可选 Fetch 请求配置。
 * 输出：成功的 HTTP 响应；非成功状态、网络故障或错误契约非法时抛出异常。
 * 功能：统一携带浏览器 Cookie 并解析后端稳定错误响应。
 */
async function request(path: string, init: RequestInit = {}): Promise<Response> {
  const headers = new Headers(init.headers)
  headers.set('Accept', 'application/json')
  const response = await fetch('/api/v1/auth' + path, {
    ...init,
    credentials: 'include',
    headers,
  })
  if (!response.ok) {
    throw await readApiError(response)
  }
  return response
}

/**
 * 输入：response，成功的认证 API 响应。
 * 输出：JSON 解码后的未知值；空、非法或不可解析响应时抛出错误。
 * 功能：把服务端 JSON 保持为未知类型，交给具体契约解析器校验。
 */
async function readJson(response: Response): Promise<unknown> {
  try {
    return await response.json() as unknown
  } catch {
    throw new Error('认证服务返回了无法解析的响应。')
  }
}

/**
 * 输入：response，非 2xx 的认证 API 响应。
 * 输出：携带状态码和稳定错误码的 ApiError；契约错误时抛出协议异常。
 * 功能：验证统一错误响应结构，防止客户端静默掩盖接口变更。
 */
async function readApiError(response: Response): Promise<ApiError> {
  const value = await readJson(response)
  if (!isRecord(value) || !isRecord(value.error) ||
    typeof value.error.code !== 'string' || typeof value.error.message !== 'string') {
    throw new Error('认证服务返回了无效的错误响应。')
  }
  return new ApiError(response.status, value.error.code, value.error.message)
}

/**
 * 输入：value，来自 JSON 响应的未知值。
 * 输出：用户编号为正安全整数且用户名符合账号规则时返回 User，否则抛出错误。
 * 功能：校验登录与本人信息响应的公开用户契约。
 */
function parseUser(value: unknown): User {
  if (!isRecord(value) || typeof value.id !== 'number' || !Number.isSafeInteger(value.id) ||
    value.id < 1 || typeof value.username !== 'string' || !/^[a-z0-9_]{3,32}$/.test(value.username)) {
    throw new Error('认证服务返回了无效的用户资料。')
  }
  return { id: value.id, username: value.username }
}

/**
 * 输入：value，任意 JSON 解码值。
 * 输出：普通对象时返回类型保护结果，否则返回 false。
 * 功能：为 API 响应字段校验提供安全的对象判定。
 */
function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}
