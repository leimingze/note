<template>
  <div class="me-page">
    <header class="me-header">
      <button class="back" @click="goHome">‹</button>
      <span>我的</span>
    </header>
    <main class="me-body">
      <div class="avatar">P</div>
      <p class="nickname">PulseFrame 用户</p>
      <p class="desc">账号功能已接入，个人资料与作品展示建设中。</p>
      <button class="logout" @click="onLogout">退出登录</button>
    </main>
  </div>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const router = useRouter()

async function onLogout() {
  await auth.logout()
  void router.push('/login')
}

function goHome() {
  void router.push('/home')
}
</script>

<style scoped>
.me-page {
  height: 100%;
  display: flex;
  flex-direction: column;
  background: var(--main-bg);
}
.me-header {
  height: var(--common-header-height, 50px);
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 16px;
  border-bottom: 1px solid var(--line-color);
}
.back {
  background: transparent;
  color: #fff;
  font-size: 22px;
}
.me-body {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 0 28px;
}
.avatar {
  width: 72px;
  height: 72px;
  border-radius: 50%;
  background: var(--primary-btn-color);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 32px;
}
.nickname {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
}
.desc {
  margin: 0;
  color: var(--second-text-color);
  font-size: 14px;
  text-align: center;
}
.logout {
  margin-top: 12px;
  padding: 10px 22px;
  border-radius: 8px;
  background: var(--active-main-bg);
  color: var(--primary-btn-color);
  font-size: 14px;
}
</style>
