// 脚本功能：创建 PulseFrame Vue 网页应用并加载全局样式；例如打开首页后，把 App 组件显示在 #app 元素中。
// 启动命令：在 frontend 目录运行 npm run dev。
import { createApp } from 'vue'
import App from './App.vue'
import './style.css'

// 页面已经提供 id 为 app 的容器，在这里把 Vue 根组件挂载到该位置。
createApp(App).mount('#app')
