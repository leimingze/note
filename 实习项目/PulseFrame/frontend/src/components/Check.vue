<template>
  <button class="check" type="button" :class="{ checked: modelValue }" @click="toggle">
    <svg viewBox="0 0 16 16" width="16" height="16" aria-hidden="true">
      <circle
        cx="8"
        cy="8"
        r="7"
        fill="none"
        :stroke="modelValue ? 'var(--primary-btn-color)' : 'var(--second-text-color)'"
        stroke-width="1.5"
      />
      <path
        v-if="modelValue"
        d="M5 8.2 7.2 10.4 11 6.4"
        fill="none"
        stroke="var(--primary-btn-color)"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
    </svg>
  </button>
</template>

<script setup lang="ts">
// 勾选框组件，移植自 douyin 前端，用 SVG 渲染选中状态。
defineOptions({ name: 'Check' })

const props = withDefaults(
  defineProps<{
    modelValue?: boolean
  }>(),
  {
    modelValue: false,
  },
)

const emit = defineEmits<{ 'update:modelValue': [boolean] }>()

function toggle() {
  emit('update:modelValue', !props.modelValue)
}
</script>

<style scoped>
.check {
  display: grid;
  place-items: center;
  width: 18px;
  height: 18px;
  padding: 0;
  background: transparent;
}
</style>
