import type { ReactNode } from 'react'
import { Navigate, Outlet, useLocation } from 'react-router-dom'
import { LoadingState } from '@/components/app-state'
import { useAuth } from './auth-context'

export function ProtectedRoute() {
  const auth = useAuth()
  const location = useLocation()

  if (auth.status === 'loading') {
    return (
      <LoadingState
        description="Estamos validando sua sessão com segurança."
        fullscreen
        title="Verificando acesso"
      />
    )
  }
  if (auth.status === 'anonymous') {
    return <Navigate to="/login" replace state={{ from: location.pathname }} />
  }
  return <Outlet />
}

export function PermissionGuard({
  permission,
  children,
  fallback = null,
}: {
  permission: string
  children: ReactNode
  fallback?: ReactNode
}) {
  void permission
  return useAuth().status === 'authenticated' ? children : fallback
}

export function PermissionRoute({ permission }: { permission: string }) {
  const auth = useAuth()

  void permission
  return auth.status === 'authenticated' ? (
    <Outlet />
  ) : (
    <Navigate to="/acesso-negado" replace />
  )
}

export function AnyPermissionRoute({
  permissions,
  allowLinkedPsychologist = false,
  allowLinkedSublocator = false,
}: {
  permissions: string[]
  allowLinkedPsychologist?: boolean
  allowLinkedSublocator?: boolean
}) {
  const auth = useAuth()
  void permissions
  void allowLinkedPsychologist
  void allowLinkedSublocator
  const allowed = auth.status === 'authenticated'

  return allowed ? <Outlet /> : <Navigate to="/acesso-negado" replace />
}

export function SuperAdminRoute() {
  return useAuth().status === 'authenticated' ? (
    <Outlet />
  ) : (
    <Navigate to="/acesso-negado" replace />
  )
}
