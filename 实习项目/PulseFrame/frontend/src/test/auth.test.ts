// 功能：验证浏览器认证 API 客户端的请求和响应契约。
// 启动命令：在 frontend 目录运行 npm test。
import { afterEach, describe, expect, it, vi } from 'vitest'
import { csrfToken, currentUser, login, logout, register } from '../api/auth'

const validUser = { id: 7, username: 'creator_7' }

/**
 * 输入：status 与 body，测试用 HTTP 状态和响应体。
 * 输出：供 Fetch API 使用的 JSON Response。
 * 功能：为认证客户端测试构造确定的协议响应。
 */
function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

/**
 * 输入：无。
 * 输出：清除当前测试安装的全局 Fetch 替身。
 * 功能：隔离各项 API 测试的浏览器网络环境。
 */
function restoreFetch(): void {
  vi.unstubAllGlobals()
}

afterEach(restoreFetch)

/**
 * 输入：Vitest 套件执行上下文。
 * 输出：注册认证 API 客户端测试集合。
 * 功能：按请求契约、错误载荷和 CSRF 注销行为组织测试。
 */
describe('认证 API 客户端', () => {
  /**
   * 输入：无。
   * 输出：断言请求方法、JSON 凭据、Cookie 选项和公开用户响应。
   * 功能：验证注册与登录遵循后端 HTTP 契约。
   */
  async function testLoginAndRegistration(): Promise<void> {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse(201, validUser))
      .mockResolvedValueOnce(jsonResponse(200, validUser))
    vi.stubGlobal('fetch', fetchMock)

    await expect(register('creator_7', 'an-eight-char-password')).resolves.toEqual(validUser)
    await expect(login('creator_7', 'an-eight-char-password')).resolves.toEqual(validUser)

    const [registerRequest, loginRequest] = fetchMock.mock.calls
    if (!registerRequest || !loginRequest) throw new Error('Expected both credential requests')
    expect(registerRequest[0]).toBe('/api/v1/auth/register')
    expect(loginRequest[0]).toBe('/api/v1/auth/login')
    expect(registerRequest[1]).toMatchObject({ method: 'POST', credentials: 'include' })
    expect(JSON.parse(String(registerRequest[1]?.body))).toEqual({
      username: 'creator_7',
      password: 'an-eight-char-password',
    })
  }

  /**
   * 输入：无。
   * 输出：断言错误码与状态保留，错误契约损坏和用户响应非法均明确失败。
   * 功能：验证 API 客户端不会把接口错误或格式漂移当作成功。
   */
  async function testRejectsInvalidResponses(): Promise<void> {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse(502, {
        error: { code: 'DEPENDENCY_UNAVAILABLE' },
      }))
      .mockResolvedValueOnce(jsonResponse(401, {
        request_id: 'request-1',
        error: { code: 'UNAUTHENTICATED', message: 'authentication required' },
      }))
      .mockResolvedValueOnce(jsonResponse(200, { id: 0, username: 'creator_7' }))
      .mockResolvedValueOnce(jsonResponse(200, { id: 7, username: 'Creator' }))
      .mockResolvedValueOnce(jsonResponse(200, { csrf_token: 'short' }))
    vi.stubGlobal('fetch', fetchMock)

    await expect(login('creator_7', 'password')).rejects.toThrow('无效的错误响应')
    await expect(currentUser()).rejects.toMatchObject({
      status: 401,
      code: 'UNAUTHENTICATED',
    })
    await expect(currentUser()).rejects.toThrow('无效的用户资料')
    await expect(currentUser()).rejects.toThrow('无效的用户资料')
    await expect(csrfToken()).rejects.toThrow('无效的 CSRF 令牌')
  }

  /**
   * 输入：无。
   * 输出：断言退出请求使用当前会话 CSRF 头和浏览器 Cookie。
   * 功能：验证网页注销请求不会把令牌放入 URL 或请求体。
   */
  async function testLogoutUsesCsrfHeader(): Promise<void> {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }))
    vi.stubGlobal('fetch', fetchMock)

    await expect(logout('a'.repeat(64))).resolves.toBeUndefined()
    const [url, init] = fetchMock.mock.calls[0] ?? []
    expect(url).toBe('/api/v1/auth/logout')
    expect(init).toMatchObject({ method: 'POST', credentials: 'include' })
    expect(new Headers(init?.headers).get('X-CSRF-Token')).toBe('a'.repeat(64))
    expect(init?.body).toBeUndefined()
  }

  it('发送注册和登录凭据并携带网页会话', testLoginAndRegistration)
  it('拒绝错误响应和非法成功载荷', testRejectsInvalidResponses)
  it('通过 CSRF 请求头注销当前会话', testLogoutUsesCsrfHeader)
})
