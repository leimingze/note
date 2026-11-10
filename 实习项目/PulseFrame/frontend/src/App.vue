<script>
import { onMounted, ref } from 'vue'
import { useAuth, restore, logout } from './auth'
import LoginView from './views/LoginView.vue'
import RegisterView from './views/RegisterView.vue'
import HomeView from './views/HomeView.vue'

export default {
  components: { LoginView, RegisterView, HomeView },
  setup() {
    const auth = useAuth()
    const view = ref('login')
    const notice = ref('')

    onMounted(restore)

    return { auth, view, notice, logout }
  },
}
</script>

<template>
  <main class="shell">
    <section class="panel intro" aria-labelledby="page-title">
      <header class="brand">
        <span class="brand-symbol" aria-hidden="true">P</span>
        <span>PulseFrame</span>
      </header>
      <div class="intro-body">
        <h1 id="page-title">PulseFrame</h1>
        <p>账号系统前端，基于 Vue 3 与 JavaScript。</p>
        <dl class="meta">
          <div>
            <dt>Web</dt>
            <dd>Vue 3 + JS</dd>
          </div>
          <div>
            <dt>API</dt>
            <dd>Go + Gin</dd>
          </div>
        </dl>
      </div>
    </section>

    <section class="panel stage">
      <p v-if="!auth.ready" class="status">加载中……</p>
      <LoginView
        v-else-if="!auth.signedIn && view === 'login'"
        :notice="notice"
        @to-register="view = 'register'"
      />
      <RegisterView
        v-else-if="!auth.signedIn && view === 'register'"
        @to-login="view = 'login'"
        @registered="notice = '注册成功，请登录'; view = 'login'"
      />
      <HomeView v-else @logout="logout" />
    </section>
  </main>
</template>
