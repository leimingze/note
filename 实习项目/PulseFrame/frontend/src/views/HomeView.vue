<script>
export default {
  emits: ['logout'],
  setup(_, { emit }) {
    return { emit }
  },
}
</script>

<template>
  <section class="home">
    <h2>已登录</h2>
    <p>访问令牌由浏览器安全保存，会话刷新在后台自动完成。</p>
    <button type="button" @click="emit('logout')">退出登录</button>
  </section>
</template>
