# PulseFrame Web

Vue 3 与 TypeScript 网页工程基座。当前页面展示项目状态和视觉素材，不包含账号接口或业务表单。

需要 Node.js 20.19 及以上，或 22.12 及以上版本。

## 开发

~~~sh
npm install
npm run dev
~~~

默认页面地址为 http://127.0.0.1:5173。当前网页不请求后端接口，因此 Vite 不配置 API 代理。

## 检查

~~~sh
npm test
npm run typecheck
npm run build
~~~

素材来源和许可见 ASSETS.md。
