import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { api } from '../api'
import router from '../router'

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string | null>(localStorage.getItem('token'))
  const expiresAt = ref<string | null>(localStorage.getItem('expiresAt'))

  const isLoggedIn = computed(() => {
    if (!token.value) return false
    if (expiresAt.value) {
      return new Date(expiresAt.value) > new Date()
    }
    return !!token.value
  })

  async function login(username: string, password: string) {
    const response = await api.post('/api/v1/auth/login', { username, password })
    const data = response.data

    token.value = data.token
    expiresAt.value = data.expires_at ?? null

    localStorage.setItem('token', data.token)
    if (data.expires_at) {
      localStorage.setItem('expiresAt', data.expires_at)
    }

    const redirect = (router.currentRoute.value.query.redirect as string) || '/dashboard'
    router.push(redirect)
  }

  function logout() {
    token.value = null
    expiresAt.value = null

    localStorage.removeItem('token')
    localStorage.removeItem('expiresAt')

    router.push('/login')
  }

  return { token, expiresAt, isLoggedIn, login, logout }
})
