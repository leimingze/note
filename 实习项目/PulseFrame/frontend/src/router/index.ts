import { createRouter, createWebHistory } from 'vue-router'
import routes from './routes'
import { useAuthStore } from '@/stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  if (!auth.ready) {
    await auth.restore()
  }
  const requiresAuth = !['/login', '/register'].includes(to.path)
  if (requiresAuth && !auth.signedIn) {
    return '/login'
  }
  if (!requiresAuth && auth.signedIn) {
    return '/home'
  }
  return true
})

export default router
