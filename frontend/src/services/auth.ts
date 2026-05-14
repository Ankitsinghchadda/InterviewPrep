import { authApi } from './api'
import type { ApiEnvelope, User } from '@/types'

export async function fetchMe(): Promise<User | null> {
  try {
    const { data } = await authApi.get<ApiEnvelope<User>>('/me')
    return data.data ?? null
  } catch {
    return null
  }
}

export async function logout(): Promise<void> {
  try {
    await authApi.post('/logout')
  } catch {
    // ignore — cookies are cleared regardless of network outcome
  }
}

export function googleLoginURL(redirect?: string): string {
  const base = (import.meta.env.VITE_AUTH_BASE_URL ?? '/auth') + '/google/login'
  if (!redirect) return base
  return `${base}?redirect=${encodeURIComponent(redirect)}`
}
