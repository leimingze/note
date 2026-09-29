# PulseFrame Web

Vue 3 与 TypeScript 网页客户端。账号页面调用真实后端认证 API，不包含模拟登录或内置测试账号。

需要 Node.js 20.19 及以上，或 22.12 及以上版本。

## 开发

启动命令：

```sh
npm install
npm run dev
```

默认页面地址为 http://127.0.0.1:5173。Vite 将 /api 代理到专用本地地址 http://127.0.0.1:8081，避免请求落到其他本机服务。启动后端时设置 HTTP_ADDR=127.0.0.1:8081，并将 WEB_ORIGIN 设置为网页的准确来源；例如本次端口为 5174 时使用 http://127.0.0.1:5174。还需按 ../backend/README.md 配置 MySQL 与 Redis。依赖或 API 不可用时，页面会显示请求错误，不会假装登录成功。

若 5173 已被占用，可运行 npm run dev -- --port 5174 --strictPort，并将后端 WEB_ORIGIN 设置为 http://127.0.0.1:5174。不要在 8080 有其他本机服务时将 API 代理回该端口。

## 检查

检查命令：

```sh
npm test
npm run typecheck
npm run build
```

组件和 API 测试使用合成账号及受控 HTTP 响应，不会写入任何共享数据库。后端 HTTP、Redis 临时实例和 SQL mock 测试见 ../backend；完整测试不等于真实隔离 MySQL/Redis 集成验收。

素材来源和许可见 ASSETS.md。
