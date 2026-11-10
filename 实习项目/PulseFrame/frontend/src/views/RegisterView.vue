<script>
import { reactive, ref } from 'vue'
import { validUsername, validPassword } from '../validate'
import { register } from '../auth'

export default {
  emits: ['to-login', 'registered'],
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
        error.value = '密码至少 12 个字符'
        return
      }
      error.value = ''
      busy.value = true
      const ok = await register(form)
      busy.value = false
      if (ok) emit('registered')
    }

    return { form, error, busy, submit, emit }
  },
}
</script>

<template>
  <form class="auth-form" @submit.prevent="submit">
    <h2>注册</h2>
    <p v-if="error" class="error">{{ error }}</p>
    <label>
      用户名
      <input v-model="form.username" maxlength="24" autocomplete="username" />
    </label>
    <label>
      密码
      <input v-model="form.password" type="password" maxlength="128" autocomplete="new-password" />
    </label>
    <button type="submit" :disabled="busy">{{ busy ? '注册中……' : '注册' }}</button>
    <p class="switch">
      已有账号？<a href="#" @click.prevent="emit('to-login')">去登录</a>
    </p>
  </form>
</template>
