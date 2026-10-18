import { lazy, Suspense } from 'react'
import { Navigate, RouterProvider, createBrowserRouter } from 'react-router'
import { AppShell } from '@/components/layout/AppShell'
import { RequireAuth } from '@/features/auth/AuthGuard'
import { LoginPage } from '@/features/auth/LoginPage'
import { TableSkeleton } from '@/components/data-display/TableSkeleton'

const DashboardPage = lazy(() => import('@/features/dashboard/DashboardPage'))
const SourcesPage = lazy(() => import('@/features/sources/SourcesPage'))
const MappingsPage = lazy(() => import('@/features/mappings/MappingsPage'))
const RulesPage = lazy(() => import('@/features/rules/RulesPage'))
const TemplatesPage = lazy(() => import('@/features/templates/TemplatesPage'))
const DestinationsPage = lazy(() => import('@/features/destinations/DestinationsPage'))
const EventsPage = lazy(() => import('@/features/events/EventsPage'))
const DeliveriesPage = lazy(() => import('@/features/deliveries/DeliveriesPage'))
const AuditPage = lazy(() => import('@/features/audit/AuditPage'))
const PlaygroundPage = lazy(() => import('@/features/playground/PlaygroundPage'))
const SettingsPage = lazy(() => import('@/features/settings/SettingsPage'))

function PageFallback() {
  return <TableSkeleton rows={8} cols={5} />
}

const router = createBrowserRouter([
  {
    path: '/login',
    element: <LoginPage />,
  },
  {
    path: '/',
    element: (
      <RequireAuth>
        <AppShell />
      </RequireAuth>
    ),
    children: [
      { index: true, element: <Navigate to="/dashboard" replace /> },
      {
        path: 'dashboard',
        element: (
          <Suspense fallback={<PageFallback />}>
            <DashboardPage />
          </Suspense>
        ),
      },
      {
        path: 'sources',
        element: (
          <Suspense fallback={<PageFallback />}>
            <SourcesPage />
          </Suspense>
        ),
      },
      {
        path: 'mappings',
        element: (
          <Suspense fallback={<PageFallback />}>
            <MappingsPage />
          </Suspense>
        ),
      },
      {
        path: 'rules',
        element: (
          <Suspense fallback={<PageFallback />}>
            <RulesPage />
          </Suspense>
        ),
      },
      {
        path: 'templates',
        element: (
          <Suspense fallback={<PageFallback />}>
            <TemplatesPage />
          </Suspense>
        ),
      },
      {
        path: 'destinations',
        element: (
          <Suspense fallback={<PageFallback />}>
            <DestinationsPage />
          </Suspense>
        ),
      },
      {
        path: 'events',
        element: (
          <Suspense fallback={<PageFallback />}>
            <EventsPage />
          </Suspense>
        ),
      },
      {
        path: 'deliveries',
        element: (
          <Suspense fallback={<PageFallback />}>
            <DeliveriesPage />
          </Suspense>
        ),
      },
      {
        path: 'audit',
        element: (
          <Suspense fallback={<PageFallback />}>
            <AuditPage />
          </Suspense>
        ),
      },
      {
        path: 'playground',
        element: (
          <Suspense fallback={<PageFallback />}>
            <PlaygroundPage />
          </Suspense>
        ),
      },
      {
        path: 'settings',
        element: (
          <Suspense fallback={<PageFallback />}>
            <SettingsPage />
          </Suspense>
        ),
      },
    ],
  },
  {
    path: '*',
    element: <Navigate to="/dashboard" replace />,
  },
])

export function AppRouter() {
  return <RouterProvider router={router} />
}
