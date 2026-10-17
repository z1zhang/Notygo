import type { ReactNode } from 'react'
import { Navigate, useLocation } from 'react-router'
import { hasValidSession } from '@/lib/api-client'

export function RequireAuth({ children }: { children: ReactNode }) {
  const location = useLocation()

  if (!hasValidSession()) {
    return <Navigate to="/login" replace state={{ redirect: location.pathname + location.search }} />
  }
  return children
}
