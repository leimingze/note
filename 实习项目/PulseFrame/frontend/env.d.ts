/// <reference types="vite/client" />

// 让 TypeScript 认识 .vue 单文件组件，避免 import 视图时报找不到模块。
declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<{}, {}, unknown>
  export default component
}
