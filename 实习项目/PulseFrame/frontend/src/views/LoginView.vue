<template>
  <main class="auth-page">
    <div class="auth-card">
      <h1 class="brand">PulseFrame</h1>
      <p class="subtitle">登录你的账号</p>
      <form @submit.prevent="onSubmit">
        <LoginInput v-model="username" placeholder="用户名" autocomplete="username" />
        <LoginInput
          v-model="password"
          type="password"
          placeholder="密码"
          autocomplete="current-password"
        />
        <div class="protocol">
          <Check v-model="agreed" />
          <span>我已阅读并同意《用户协议》与《隐私政策》</span>
        </div>
        <p v-if="auth.error" class="error">{{ auth.error }}</p>
        <BaseButton
          type="primary"
          block
          :loading="auth.loading"
          :disabled="!username || !password || auth.loading"
        >
          {{ auth.loading ? '登录中' : '登录' }}
        </BaseButton>
        <BaseButton type="text" block class="switch" @click="goRegister">
          没有账号？去注册
        </BaseButton>
      </form>
    </div>
  </main>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { validatePassword, validateUsername } from '@/utils/validate'
import BaseButton from '@/components/BaseButton.vue'
import LoginInput from '@/components/LoginInput.vue'
import Check from '@/components/Check.vue'

const auth = useAuthStore()
const router = useRouter()
const username = ref('')
const password = ref('')
const agreed = ref(false)

async function onSubmit() {
  const usernameError = validateUsername(username.value)
  const passwordError = validatePassword(password.value)
  if (usernameError || passwordError) {
    auth.error = usernameError || passwordError || ''
    return
  }
  try {
    await auth.login({ username: username.value, password: password.value })
    void router.push('/home')
  } catch {
    // 错误信息已写入 auth.error
  }
}

function goRegister() {
  void router.push('/register')
}
</script>

<style scoped>
.auth-page {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0 28px;
  background: var(--main-bg);
}
.auth-card {
  width: 100%;
  max-width: 360px;
}
.brand {
  margin: 0;
  font-size: 34px;
  text-align: center;
  letter-spacing: 1px;
}
.subtitle {
  margin: 8px 0 36px;
  color: var(--second-text-color);
  font-size: 14px;
  text-align: center;
}
.auth-card form {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.error {
  margin: 0;
  color: var(--primary-btn-color);
  font-size: 13px;
}
.protocol {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--second-text-color);
  font-size: 12px;
}
.switch {
  margin-top: 4px;
}
</style>
