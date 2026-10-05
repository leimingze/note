// 功能：配置 Vue 编译和 Vitest 测试环境。
// 启动命令：在 frontend 目录运行 npm run dev 或 npm test。
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vitest/config'

export default defineConfig({
  plugins: [vue()],
  test: {
    environment: 'jsdom',
  },
})
