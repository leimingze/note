<template>
  <div class="me-page">
    <BaseHeader show-back :on-back="goHome">我的</BaseHeader>
    <main class="me-body">
      <div class="avatar">P</div>
      <p class="nickname">PulseFrame 用户</p>
      <p class="desc">账号功能已接入，个人资料与作品展示建设中。</p>
      <button class="logout" @click="showConfirm = true">退出登录</button>
    </main>
    <ConfirmDialog
      v-model:visible="showConfirm"
      title="退出登录"
      message="确定要退出当前账号吗？"
      ok-text="退出"
      @ok="onLogout"
    />
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import BaseHeader from '@/components/BaseHeader.vue'
import ConfirmDialog from '@/components/dialog/ConfirmDialog.vue'

const auth = useAuthStore()
const router = useRouter()
const showConfirm = ref(false)

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
