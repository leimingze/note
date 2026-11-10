<template>
  <button
    class="base-button"
    :class="[`btn-${type}`, { block, disabled }]"
    :disabled="disabled"
    :type="submit ? 'submit' : 'button'"
    @click="onClick"
  >
    <span v-if="loading" class="spinner" aria-hidden="true"></span>
    <slot />
  </button>
</template>

<script setup lang="ts">
// 通用按钮组件，移植自 douyin 前端，提供主色、轮廓与文字三种形态。
defineOptions({ name: 'BaseButton' })

withDefaults(
  defineProps<{
    type?: 'primary' | 'ghost' | 'text'
    block?: boolean
    submit?: boolean
    disabled?: boolean
    loading?: boolean
  }>(),
  {
    type: 'primary',
    block: false,
    submit: false,
    disabled: false,
    loading: false,
  },
)

const emit = defineEmits<{ click: [] }>()

function onClick() {
  emit('click')
}
</script>

<style scoped>
.base-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  height: 50px;
  padding: 0 24px;
  border-radius: 12px;
  font-size: 16px;
  font-weight: 600;
  transition: opacity 0.15s ease;
}
.base-button.block {
  width: 100%;
}
.btn-primary {
  background: var(--primary-btn-color);
  color: #fff;
}
.btn-primary.disabled {
  background: var(--disable-primary-btn-color);
}
.btn-ghost {
  background: var(--active-main-bg);
  border: 1px solid var(--line-color);
  color: #fff;
}
.btn-text {
  background: transparent;
  color: var(--second-text-color);
}
.spinner {
  width: 16px;
  height: 16px;
  border: 2px solid rgba(255, 255, 255, 0.4);
  border-top-color: #fff;
  border-radius: 50%;
  animation: spin 0.7s linear infinite;
}
@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
