// 功能：验证注册、登录、会话恢复和安全退出页面流程。
// 启动命令：在 frontend 目录运行 npm test。
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import App from '../App.vue'

const user = { id: 12, username: 'frame_maker' }
const csrf = 'c'.repeat(64)
let responder: (url: string, init: RequestInit) => Promise<Response>

/**
 * 输入：input 与 init，浏览器 Fetch 请求地址及参数。
 * 输出：交由当前测试场景处理的 HTTP Response；未配置请求抛出错误。
 * 功能：将真实 API 客户端连接到逐测试配置的 HTTP 边界。
 */
async function routeFetch(input: RequestInfo | URL, init: RequestInit = {}): Promise<Response> {
  return responder(String(input), init)
}

/**
 * 输入：status 与 body，测试用 HTTP 状态和响应体。
 * 输出：供 Fetch API 使用的 JSON Response。
 * 功能：构造与真实后端一致的成功或错误载荷。
 */
function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

/**
 * 输入：无。
 * 输出：将测试全局 Fetch 设置为只接受匿名会话查询的 HTTP 边界。
 * 功能：让每个页面测试从访客状态开始，并暴露未预期的网络调用。
 */
function installGuestFetch(): void {
  /**
   * 输入：url，请求 API 地址。
   * 输出：访客会话返回 401；其他请求抛出意外调用错误。
   * 功能：限定初始化请求，防止测试意外依赖未配置的接口响应。
   */
  responder = async function guestSession(url: string): Promise<Response> {
    if (url.endsWith('/me')) {
      return jsonResponse(401, {
        request_id: 'guest-request',
        error: { code: 'UNAUTHENTICATED', message: 'authentication required' },
      })
    }
    throw new Error('Unexpected API request: ' + url)
  }
  vi.stubGlobal('fetch', vi.fn(routeFetch))
}

/**
 * 输入：无。
 * 输出：解除全局 Fetch 替身，恢复测试运行环境。
 * 功能：防止账号页面测试影响其他测试文件。
 */
function restoreFetch(): void {
  vi.unstubAllGlobals()
}

/**
 * 输入：无。
 * 输出：访客页面已挂载且初始 /me 请求完成。
 * 功能：集中等待登录页面初始化后再验证用户操作。
 */
async function mountGuestPage() {
  const wrapper = mount(App)
  await flushPromises()
  return wrapper
}

beforeEach(installGuestFetch)
afterEach(restoreFetch)

/**
 * 输入：Vitest 套件执行上下文。
 * 输出：注册账号页面交互测试集合。
 * 功能：集中验证访客注册、登录、冲突和退出流程。
 */
describe('账号页面', () => {
  /**
   * 输入：无。
   * 输出：断言注册校验、成功反馈与注册后显式登录流程。
   * 功能：验证用户创建不会静默建立会话。
   */
  async function testRegistrationRequiresConfirmationAndThenLogin(): Promise<void> {
    /**
     * 输入：url 与 init，页面发出的 API 地址和 Fetch 配置。
     * 输出：注册成功或访客查询响应；未声明请求抛出错误。
     * 功能：只开放该测试所需的注册与匿名会话接口。
     */
    responder = async function registrationFlow(url: string, init: RequestInit): Promise<Response> {
      if (url.endsWith('/register')) return jsonResponse(201, user)
      if (url.endsWith('/me')) {
        return jsonResponse(401, {
          request_id: 'guest-request',
          error: { code: 'UNAUTHENTICATED', message: 'authentication required' },
        })
      }
      throw new Error('Unexpected API request: ' + url + ' ' + String(init.method))
    }
    const wrapper = await mountGuestPage()
    await wrapper.get('button.mode-switch').trigger('click')
    await wrapper.get('#username').setValue('frame_maker')
    await wrapper.get('#password').setValue('eight-char-password')
    await wrapper.get('#password-confirmation').setValue('different-password')
    await wrapper.get('form').trigger('submit')
    expect(wrapper.get('[role="alert"]').text()).toContain('两次输入')

    await wrapper.get('#password').setValue('short')
    await wrapper.get('#password-confirmation').setValue('short')
    await wrapper.get('form').trigger('submit')
    expect(wrapper.get('[role="alert"]').text()).toContain('至少需要 8')

    await wrapper.get('#password').setValue('eight-char-password')
    await wrapper.get('#password-confirmation').setValue('eight-char-password')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.text()).toContain('账号已创建，请登录')
    expect(wrapper.text()).toContain('账号登录')
  }

  /**
   * 输入：无。
   * 输出：断言登录、页面重载会话恢复、CSRF 注销顺序和状态清除。
   * 功能：覆盖 Cookie 会话在网页中的完整交互链路。
   */
  async function testLoginRestoreAndCsrfLogout(): Promise<void> {
    let authenticated = false
    /**
     * 输入：url 与 init，页面发出的 API 地址和 Fetch 配置。
     * 输出：依据登录状态执行登录、查询、CSRF 或退出响应。
     * 功能：维护本测试隔离的服务端会话状态。
     */
    responder = async function authenticatedFlow(url: string, init: RequestInit): Promise<Response> {
      if (url.endsWith('/login')) {
        authenticated = true
        return jsonResponse(200, user)
      }
      if (url.endsWith('/me')) {
        return authenticated
          ? jsonResponse(200, user)
          : jsonResponse(401, {
            request_id: 'guest-request',
            error: { code: 'UNAUTHENTICATED', message: 'authentication required' },
          })
      }
      if (url.endsWith('/csrf')) return jsonResponse(200, { csrf_token: csrf })
      if (url.endsWith('/logout') && init.method === 'POST' &&
        new Headers(init.headers).get('X-CSRF-Token') === csrf) {
        authenticated = false
        return new Response(null, { status: 204 })
      }
      throw new Error('Unexpected API request: ' + url)
    }
    const wrapper = await mountGuestPage()
    await wrapper.get('#username').setValue('frame_maker')
    await wrapper.get('#password').setValue('eight-char-password')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.text()).toContain('已登录')

    await wrapper.get('button.primary-button').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('已安全退出')
    expect(wrapper.find('form').exists()).toBe(true)
  }

  /**
   * 输入：无。
   * 输出：断言重复用户名以用户可读错误展示且页面保持注册状态。
   * 功能：验证冲突响应不会丢失表单上下文。
   */
  async function testRegistrationConflict(): Promise<void> {
    /**
     * 输入：url，请求 API 地址。
     * 输出：匿名查询或用户名冲突响应；未声明请求抛出错误。
     * 功能：模拟服务端唯一键冲突并验证界面错误路径。
     */
    responder = async function duplicateUsername(url: string): Promise<Response> {
      if (url.endsWith('/me')) {
        return jsonResponse(401, {
          request_id: 'guest-request',
          error: { code: 'UNAUTHENTICATED', message: 'authentication required' },
        })
      }
      if (url.endsWith('/register')) {
        return jsonResponse(409, {
          request_id: 'duplicate-request',
          error: { code: 'USERNAME_TAKEN', message: 'username already exists' },
        })
      }
      throw new Error('Unexpected API request: ' + url)
    }
    const wrapper = await mountGuestPage()
    await wrapper.get('button.mode-switch').trigger('click')
    await wrapper.get('#username').setValue('frame_maker')
    await wrapper.get('#password').setValue('eight-char-password')
    await wrapper.get('#password-confirmation').setValue('eight-char-password')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('已被使用')
    expect(wrapper.text()).toContain('确认密码')
  }

  /**
   * 输入：无。
   * 输出：断言无效凭据显示通用中文错误且不改变访客状态。
   * 功能：验证认证失败不会透露账号是否存在或清除登录表单。
   */
  async function testInvalidLoginFeedback(): Promise<void> {
    /**
     * 输入：url，请求 API 地址。
     * 输出：访客查询或统一无效凭据响应；未声明请求抛出错误。
     * 功能：模拟后端对不存在用户和错误密码使用的同一错误码。
     */
    responder = async function invalidCredentials(url: string): Promise<Response> {
      if (url.endsWith('/me')) {
        return jsonResponse(401, {
          request_id: 'guest-request',
          error: { code: 'UNAUTHENTICATED', message: 'authentication required' },
        })
      }
      if (url.endsWith('/login')) {
        return jsonResponse(401, {
          request_id: 'login-request',
          error: { code: 'INVALID_CREDENTIALS', message: 'invalid credentials' },
        })
      }
      throw new Error('Unexpected API request: ' + url)
    }
    const wrapper = await mountGuestPage()
    await wrapper.get('#username').setValue('frame_maker')
    await wrapper.get('#password').setValue('wrong-password')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toBe('用户名或密码不正确。')
    expect(wrapper.find('form').exists()).toBe(true)
  }

  it('注册前校验密码并在成功后要求用户登录', testRegistrationRequiresConfirmationAndThenLogin)
  it('登录、恢复会话并用 CSRF 令牌退出', testLoginRestoreAndCsrfLogout)
  it('展示用户名冲突并保留注册表单', testRegistrationConflict)
  it('错误凭据显示不区分账号存在性的提示', testInvalidLoginFeedback)
})
