import { defineStore } from 'pinia'
import { api, setAccessToken } from '@/api/request'

interface LoginResponse {
  accessToken: string
  expiresIn: number
}

export const useAuthStore = defineStore('auth', {
  state: () => ({
    signedIn: false,
    ready: false,
    loading: false,
    error: '',
  }),
  actions: {
    async restore() {
      try {
        const data = await api.post<LoginResponse>('/refresh')
        setAccessToken(data.accessToken)
        this.signedIn = true
      } catch {
        this.signedIn = false
      } finally {
        this.ready = true
      }
    },
    async login(body: { username: string; password: string }) {
      this.loading = true
      this.error = ''
      try {
        const data = await api.post<LoginResponse>('/login', body)
        setAccessToken(data.accessToken)
        this.signedIn = true
      } catch (err) {
        this.error = (err as Error).message
        throw err
      } finally {
        this.loading = false
      }
    },
    async register(body: { username: string; password: string }) {
      this.loading = true
      this.error = ''
      try {
        await api.post('/register', body)
      } catch (err) {
        this.error = (err as Error).message
        throw err
      } finally {
        this.loading = false
      }
    },
    async logout() {
      try {
        await api.post('/logout')
      } finally {
        setAccessToken(null)
        this.signedIn = false
      }
    },
  },
})
