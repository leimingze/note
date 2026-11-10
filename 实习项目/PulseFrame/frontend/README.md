# PulseFrame Web

Vue 3 + JavaScript 前端，当前实现账号模块（注册、登录、会话刷新、退出），并保留清晰的扩展结构。

需要 Node.js 18 及以上版本。

## 开发

```sh
npm install
npm run dev
```

开发服务默认监听 http://127.0.0.1:5173 ，并将 `/api` 请求代理到本地后端 `http://127.0.0.1:8080`，避免开发期跨域。

## 检查

```sh
npm test
npm run build
```

## 目录结构

- `src/api.js`：统一请求封装，自动附加令牌，401 时自动刷新会话并重试。
- `src/auth.js`：登录状态与注册、登录、恢复、退出操作。
- `src/validate.js`：与后端一致的输入校验规则。
- `src/views/`：页面视图（LoginView、RegisterView、HomeView）。
- `src/App.vue`：应用骨架与视图切换。

## 接口对应

- `POST /api/register`：注册，成功返回 201。
- `POST /api/login`：登录，返回 `accessToken`，并下发 HttpOnly 刷新 Cookie。
- `POST /api/refresh`：用 Cookie 刷新访问令牌。
- `POST /api/logout`：退出并清除 Cookie。

刷新令牌以 HttpOnly Cookie 保存且标记 `Secure`，脚本无法读取；在本地 HTTP 开发环境下，浏览器会拒绝保存该 Cookie。要完整验证“登录后自动续期”，需要在后端前配置 HTTPS，或临时调整后端 Cookie 的 `Secure` 与 `AllowedOrigin` 配置。
