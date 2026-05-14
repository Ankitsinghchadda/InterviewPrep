import axios, { AxiosError, type AxiosRequestConfig } from 'axios'

const apiBase = import.meta.env.VITE_API_BASE_URL ?? '/api/v1'
const authBase = import.meta.env.VITE_AUTH_BASE_URL ?? '/auth'

export const api = axios.create({
  baseURL: apiBase,
  headers: { 'Content-Type': 'application/json' },
  withCredentials: true,
})

export const authApi = axios.create({
  baseURL: authBase,
  headers: { 'Content-Type': 'application/json' },
  withCredentials: true,
})

type RetryConfig = AxiosRequestConfig & { _retry?: boolean }

let refreshPromise: Promise<void> | null = null

// Single-flight refresh: while one /auth/refresh is in progress, all other 401s wait on it.
async function performRefresh(): Promise<void> {
  if (!refreshPromise) {
    refreshPromise = authApi
      .post('/refresh')
      .then(() => {
        /* cookie rotated server-side */
      })
      .finally(() => {
        refreshPromise = null
      })
  }
  return refreshPromise
}

let onAuthFailure: (() => void) | null = null

export function setOnAuthFailure(cb: (() => void) | null) {
  onAuthFailure = cb
}

api.interceptors.response.use(
  (r) => r,
  async (error: AxiosError) => {
    const original = error.config as RetryConfig | undefined
    if (!original || error.response?.status !== 401 || original._retry) {
      if (error.response?.status === 401) onAuthFailure?.()
      return Promise.reject(error)
    }
    // Don't try to refresh the refresh endpoint itself.
    if (original.url?.includes('/auth/refresh')) {
      onAuthFailure?.()
      return Promise.reject(error)
    }
    original._retry = true
    try {
      await performRefresh()
      return api(original)
    } catch (refreshErr) {
      onAuthFailure?.()
      return Promise.reject(refreshErr)
    }
  },
)
