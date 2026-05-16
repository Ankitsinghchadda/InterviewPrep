import axios, { AxiosError, type AxiosRequestConfig } from 'axios'
import type { PaywallReason } from '@/types'

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
let onPaywall: ((reason: PaywallReason) => void) | null = null

export function setOnAuthFailure(cb: (() => void) | null) {
  onAuthFailure = cb
}

// setOnPaywall registers a global handler for 402/403 paywall responses.
// The Layout mounts the PaywallModal and registers this once on mount.
// Returning the reason lets the modal show kind-specific copy.
export function setOnPaywall(cb: ((reason: PaywallReason) => void) | null) {
  onPaywall = cb
}

// isPaywallReason narrows an unknown error body to PaywallReason. Both
// shapes are emitted by the server (see internal/handlers/billing_record.go).
function isPaywallReason(v: unknown): v is PaywallReason {
  if (!v || typeof v !== 'object') return false
  const e = (v as { error?: unknown }).error
  return e === 'quota_exceeded' || e === 'plan_required'
}

api.interceptors.response.use(
  (r) => r,
  async (error: AxiosError) => {
    // Paywall: 402 (quota exceeded) or 403 (plan required). Fire the
    // global handler BEFORE rejecting so the modal opens even when the
    // caller chooses to swallow the error.
    const status = error.response?.status
    if ((status === 402 || status === 403) && isPaywallReason(error.response?.data)) {
      onPaywall?.(error.response!.data as PaywallReason)
      return Promise.reject(error)
    }

    const original = error.config as RetryConfig | undefined
    if (!original || status !== 401 || original._retry) {
      if (status === 401) onAuthFailure?.()
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
