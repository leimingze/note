// 功能：配置 Vue 编译、认证 API 同源代理和 Vitest 测试环境。
// 启动命令：在 frontend 目录运行 npm run dev 或 npm test。
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vitest/config'

export default defineConfig({
  plugins: [vue()],
  server: {
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8081',
        changeOrigin: false,
      },
    },
  },
  test: {
    environment: 'jsdom',
    clearMocks: true,
    restoreMocks: true,
  },
})
