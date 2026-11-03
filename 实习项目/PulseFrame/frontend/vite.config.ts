// 脚本功能：配置 Vue 编译和 Vitest 测试环境；例如运行 npm test 时，用 jsdom 提供组件测试所需的页面环境。
// 启动命令：在 frontend 目录运行 npm run dev 或 npm test。
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vitest/config'

export default defineConfig({
  // 让开发服务和测试工具都能读取 .vue 单文件组件。
  plugins: [vue()],
  test: {
    // 在 Node.js 中提供 document 等浏览器对象，供组件挂载和页面断言使用。
    environment: 'jsdom',
  },
})
