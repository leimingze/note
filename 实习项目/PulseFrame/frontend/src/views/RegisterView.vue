<template>
  <main class="auth-page">
    <h1 class="brand">PulseFrame</h1>
    <p class="subtitle">注册新账号</p>
    <form class="form" @submit.prevent="onSubmit">
      <input v-model="username" placeholder="用户名" autocomplete="username" />
      <input
        v-model="password"
        type="password"
        placeholder="密码（12-128 位）"
        autocomplete="new-password"
      />
      <p v-if="auth.error" class="error">{{ auth.error }}</p>
      <button class="primary" type="submit" :disabled="auth.loading">
        {{ auth.loading ? '注册中…' : '注册' }}
      </button>
      <a class="link" @click.prevent="goLogin">已有账号？去登录</a>
    </form>
  </main>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { validatePassword, validateUsername } from '@/utils/validate'

const auth = useAuthStore()
const router = useRouter()
const username = ref('')
const password = ref('')

async function onSubmit() {
  const usernameError = validateUsername(username.value)
  const passwordError = validatePassword(password.value)
  if (usernameError || passwordError) {
    auth.error = usernameError || passwordError || ''
    return
  }
  try {
    await auth.register({ username: username.value, password: password.value })
    auth.error = '注册成功，请登录'
    void router.push('/login')
  } catch {
    // 错误信息已写入 auth.error
  }
}

function goLogin() {
  void router.push('/login')
}
</script>

<style scoped>
.auth-page {
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 0 28px;
  background: var(--main-bg);
}
.brand {
  font-size: 30px;
  margin: 0 0 8px;
}
.subtitle {
  color: var(--second-text-color);
  margin: 0 0 28px;
}
.form {
  width: 100%;
  max-width: 340px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.form input {
  height: 48px;
  padding: 0 16px;
  border-radius: 10px;
  border: 1px solid var(--line-color);
  background: var(--active-main-bg);
  color: #fff;
  font-size: 15px;
  outline: none;
}
.primary {
  height: 48px;
  border-radius: 10px;
  background: var(--primary-btn-color);
  color: #fff;
  font-size: 16px;
}
.primary:disabled {
  background: var(--disable-primary-btn-color);
}
.error {
  color: var(--primary-btn-color);
  font-size: 13px;
  margin: 0;
}
.link {
  text-align: center;
  color: var(--second-text-color);
  font-size: 14px;
  margin-top: 6px;
  cursor: pointer;
}
</style>
