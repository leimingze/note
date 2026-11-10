<template>
  <div class="loading" :class="{ inline: !full, full }">
    <span class="dot blue"></span>
    <span class="dot red"></span>
  </div>
</template>

<script setup lang="ts">
// 加载动画组件，移植自 douyin 前端，支持行内与全屏两种形态。
defineOptions({ name: 'Loading' })

withDefaults(
  defineProps<{
    full?: boolean
  }>(),
  {
    full: true,
  },
)
</script>

<style scoped>
.loading {
  display: flex;
  justify-content: center;
  align-items: center;
}
.loading.inline {
  width: 100%;
  height: 40px;
}
.loading.full {
  position: fixed;
  inset: 0;
  z-index: 999;
  background: var(--main-bg);
}
.dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}
.dot.blue {
  background: cadetblue;
  animation: shift-blue 0.4s ease-in-out infinite alternate;
}
.dot.red {
  background: var(--primary-btn-color);
  animation: shift-red 0.4s ease-in-out infinite alternate;
}
@keyframes shift-blue {
  to {
    transform: translate(10px, 0) scale(1.2);
  }
}
@keyframes shift-red {
  to {
    transform: translate(-10px, 0) scale(1.2);
  }
}
</style>
