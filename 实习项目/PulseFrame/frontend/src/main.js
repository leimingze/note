// 脚本功能：创建 PulseFrame Vue 应用并挂载到页面容器，加载全局样式。
// 启动命令：在 frontend 目录运行 npm run dev。
import { createApp } from 'vue'
import App from './App.vue'
import './style.css'

createApp(App).mount('#app')
