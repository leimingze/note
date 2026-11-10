// 脚本功能：配置 Vue 编译与开发代理；例如把 /api 请求转发到本地后端。
// 启动命令：在 frontend 目录运行 npm run dev。
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [vue()],
  server: {
    proxy: {
      // 把 /api 请求转发到本地后端，避免开发期跨域。
      '/api': 'http://127.0.0.1:8080',
    },
  },
})
