<template>
  <div class="login-input">
    <input
      :type="type"
      :value="modelValue"
      :placeholder="placeholder"
      :maxlength="maxlength"
      :autocomplete="autocomplete"
      :autofocus="autofocus"
      @input="onInput"
    />
    <button
      v-if="modelValue && clearable"
      class="clear"
      type="button"
      aria-label="清空输入"
      @click="onClear"
    >
      <svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true">
        <path
          d="M8 7.05 11.95 3.1a.5.5 0 0 1 .7.7L8.7 7.75l3.95 3.95a.5.5 0 0 1-.7.7L8 8.45 4.05 12.4a.5.5 0 0 1-.7-.7L7.3 7.75 3.35 3.8a.5.5 0 0 1 .7-.7L8 7.05Z"
          fill="currentColor"
        />
      </svg>
    </button>
  </div>
</template>

<script setup lang="ts">
// 输入框组件，移植自 douyin 前端，带清空按钮并统一深色输入样式。
defineOptions({ name: 'LoginInput' })

withDefaults(
  defineProps<{
    modelValue: string
    placeholder?: string
    type?: 'text' | 'password'
    maxlength?: number
    autocomplete?: string
    autofocus?: boolean
    clearable?: boolean
  }>(),
  {
    placeholder: '',
    type: 'text',
    autocomplete: undefined,
    autofocus: false,
    clearable: true,
  },
)

const emit = defineEmits<{ 'update:modelValue': [string] }>()

function onInput(e: Event) {
  emit('update:modelValue', (e.target as HTMLInputElement).value)
}

function onClear() {
  emit('update:modelValue', '')
}
</script>

<style scoped>
.login-input {
  position: relative;
}
.login-input input {
  width: 100%;
  height: 50px;
  padding: 0 44px 0 16px;
  border: 1px solid var(--line-color);
  border-radius: 12px;
  background: var(--active-main-bg);
  color: #fff;
  font-size: 15px;
  outline: none;
  caret-color: var(--primary-btn-color);
  transition: border-color 0.15s ease;
}
.login-input input::placeholder {
  color: var(--second-text-color);
}
.login-input input:focus {
  border-color: var(--primary-btn-color);
}
.clear {
  position: absolute;
  top: 50%;
  right: 14px;
  transform: translateY(-50%);
  display: grid;
  place-items: center;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.12);
  color: var(--second-text-color);
}
</style>
