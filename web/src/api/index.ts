import axios from 'axios'
import { ElMessage } from 'element-plus'
import i18n from '../locales'
import router from '../router'

const api = axios.create({
  baseURL: '',
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
})

// Request interceptor: attach Bearer token
api.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

// Response interceptor: handle 401 globally
api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('token')
      localStorage.removeItem('expiresAt')
      router.push('/login')
      ElMessage.error(i18n.global.t('login.sessionExpired'))
    }
    return Promise.reject(error)
  }
)

export { api }
