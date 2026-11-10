<script>
import { reactive, ref } from 'vue'
import { validUsername, validPassword } from '../validate'
import { login } from '../auth'

export default {
  props: {
    notice: { type: String, default: '' },
  },
  emits: ['to-register'],
  setup(_, { emit }) {
    const form = reactive({ username: '', password: '' })
    const error = ref('')
    const busy = ref(false)

    async function submit() {
      if (!validUsername(form.username)) {
        error.value = '用户名为 3-24 位字母、数字或下划线'
        return
      }
      if (!validPassword(form.password)) {
        error.value = '密码至少 6 个字符'
        return
      }
      error.value = ''
      busy.value = true
      await login(form)
      busy.value = false
    }

    return { form, error, busy, submit, emit }
  },
}
</script>

<template>
  <form class="auth-form" @submit.prevent="submit">
    <h2>登录</h2>
    <p v-if="notice" class="notice">{{ notice }}</p>
    <p v-if="error" class="error">{{ error }}</p>
    <label>
      用户名
      <input v-model="form.username" maxlength="24" autocomplete="username" />
    </label>
    <label>
      密码
      <input v-model="form.password" type="password" maxlength="128" autocomplete="current-password" />
    </label>
    <button type="submit" :disabled="busy">{{ busy ? '登录中……' : '登录' }}</button>
    <p class="switch">
      还没有账号？<a href="#" @click.prevent="emit('to-register')">去注册</a>
    </p>
  </form>
</template>
