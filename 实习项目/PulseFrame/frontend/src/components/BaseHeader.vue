<template>
  <header class="base-header">
    <div class="left">
      <button v-if="showBack" class="back" type="button" aria-label="返回" @click="onBack">
        ‹
      </button>
      <slot name="left" />
    </div>
    <div class="center"><slot /></div>
    <div class="right"><slot name="right" /></div>
  </header>
</template>

<script setup lang="ts">
// 顶部栏组件，移植自 douyin 前端，支持返回按钮与左右插槽。
defineOptions({ name: 'BaseHeader' })

const props = withDefaults(
  defineProps<{
    showBack?: boolean
    onBack?: () => void
  }>(),
  {
    showBack: false,
    onBack: undefined,
  },
)

import { useRouter } from 'vue-router'
const router = useRouter()

function onBack() {
  if (props.onBack) {
    props.onBack()
  } else {
    router.back()
  }
}
</script>

<style scoped>
.base-header {
  position: relative;
  display: flex;
  align-items: center;
  height: var(--home-header-height);
  border-bottom: 1px solid var(--line-color);
}
.left {
  position: absolute;
  left: 16px;
  display: flex;
  align-items: center;
}
.back {
  background: transparent;
  color: #fff;
  font-size: 24px;
  line-height: 1;
}
.center {
  flex: 1;
  text-align: center;
  font-size: 16px;
  font-weight: 600;
}
.right {
  position: absolute;
  right: 16px;
  display: flex;
  align-items: center;
}
</style>
