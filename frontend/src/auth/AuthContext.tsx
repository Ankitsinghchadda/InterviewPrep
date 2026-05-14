import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import type { User } from '@/types'
import { fetchMe, logout as apiLogout } from '@/services/auth'
import { setOnAuthFailure } from '@/services/api'

type Status = 'loading' | 'authenticated' | 'unauthenticated'

interface AuthState {
  status: Status
  user: User | null
  refresh: () => Promise<void>
  logout: () => Promise<void>
}

const AuthCtx = createContext<AuthState | undefined>(undefined)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [status, setStatus] = useState<Status>('loading')
  const mounted = useRef(true)

  const load = useCallback(async () => {
    const me = await fetchMe()
    if (!mounted.current) return
    setUser(me)
    setStatus(me ? 'authenticated' : 'unauthenticated')
  }, [])

  const logout = useCallback(async () => {
    await apiLogout()
    if (!mounted.current) return
    setUser(null)
    setStatus('unauthenticated')
  }, [])

  useEffect(() => {
    mounted.current = true
    void load()
    setOnAuthFailure(() => {
      setUser(null)
      setStatus('unauthenticated')
    })
    return () => {
      mounted.current = false
      setOnAuthFailure(null)
    }
  }, [load])

  const value = useMemo<AuthState>(
    () => ({ status, user, refresh: load, logout }),
    [status, user, load, logout],
  )

  return <AuthCtx.Provider value={value}>{children}</AuthCtx.Provider>
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthCtx)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
