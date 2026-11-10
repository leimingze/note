// 脚本功能：配置 Vue 编译、@ 路径别名与开发代理，把 /api 请求转发到本地后端。
// 启动命令：在 frontend 目录运行 npm run dev。
import { fileURLToPath, URL } from 'node:url'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    proxy: {
      '/api': 'http://127.0.0.1:8080',
    },
  },
})
