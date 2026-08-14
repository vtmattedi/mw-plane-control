import { useQueryClient } from '@tanstack/react-query'
import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react'

import {
  beginLogin,
  ControlApiError,
  currentIdentity,
  endSession,
  type ControlIdentity,
} from '@/lib/control-api'

import { AuthContext, type AuthContextValue } from './auth-context'

export function AuthProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient()

  const [status, setStatus] = useState<AuthContextValue['status']>('loading')

  const [user, setUser] = useState<ControlIdentity | null>(null)

  useEffect(() => {
    let active = true

    void currentIdentity()
      .then((restoredUser) => {
        if (!active) return

        setUser(restoredUser)
        setStatus('authenticated')
      })
      .catch((error: unknown) => {
        if (!active) return

        setUser(null)
        setStatus('anonymous')

        if (!(error instanceof ControlApiError && error.status === 401)) {
          console.error('Failed to restore admin session', error)
        }
      })

    return () => {
      active = false
    }
  }, [])

  const login = useCallback(() => beginLogin(), [])

  const logout = useCallback(async () => {
    try {
      await endSession()
    } finally {
      setUser(null)
      setStatus('anonymous')
      queryClient.clear()
    }
  }, [queryClient])

  const value = useMemo<AuthContextValue>(
    () => ({
      status,
      user,
      login,
      logout,
    }),
    [status, user, login, logout],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}
