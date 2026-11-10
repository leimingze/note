<template>
  <div class="home-page">
    <BaseHeader>
      <template #right>
        <button class="logout" @click="onLogout">退出</button>
      </template>
      PulseFrame
    </BaseHeader>
    <main class="feed-placeholder">
      <p class="feed-title">短视频 Feed</p>
      <p class="hint">功能建设中，即将支持视频浏览与播放。</p>
    </main>
    <nav class="tabbar">
      <button class="tab active">首页</button>
      <button class="tab" @click="goMe">我的</button>
    </nav>
  </div>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import BaseHeader from '@/components/BaseHeader.vue'

const auth = useAuthStore()
const router = useRouter()

async function onLogout() {
  await auth.logout()
  void router.push('/login')
}

function goMe() {
  void router.push('/me')
}
</script>

<style scoped>
.home-page {
  height: 100%;
  display: flex;
  flex-direction: column;
  background: var(--main-bg);
}
.logout {
  color: var(--second-text-color);
  background: transparent;
  font-size: 14px;
}
.feed-placeholder {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
}
.feed-title {
  font-size: 18px;
  margin: 0;
}
.hint {
  color: var(--second-text-color);
  font-size: 14px;
  margin: 0;
}
.tabbar {
  height: var(--footer-height);
  display: flex;
  align-items: center;
  justify-content: space-around;
  border-top: 1px solid var(--line-color);
}
.tab {
  background: transparent;
  color: var(--second-text-color);
  font-size: 15px;
}
.tab.active {
  color: #fff;
}
</style>
