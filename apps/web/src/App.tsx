import {
  Outlet,
  RouterProvider,
  createRootRoute,
  createRoute,
  createRouter,
  redirect,
} from '@tanstack/react-router'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { AuthProvider, useAuth } from './lib/auth'
import { AppShell } from './components/AppShell'
import { LoginPage, RegisterPage } from './pages/Auth'
import { DashboardPage } from './pages/Dashboard'
import { ServersPage } from './pages/Servers'
import { ApplicationsPage, DatabasesPage, ProjectsPage, ServicesPage } from './pages/Resources'
import { ApplicationDetailPage } from './pages/ApplicationDetail'
import { OnboardingPage } from './pages/Onboarding'
import { NotificationsPage } from './pages/Notifications'
import { SettingsPage } from './pages/Settings'
import './index.css'

const queryClient = new QueryClient()

function RootComponent() {
  return (
    <AuthProvider>
      <Outlet />
    </AuthProvider>
  )
}

function RequireAuth() {
  const { user, loading } = useAuth()
  if (loading) {
    return (
      <div className="grid min-h-screen place-items-center text-[var(--color-muted)]">Loading…</div>
    )
  }
  if (!user) {
    throw redirect({ to: '/login' })
  }
  return <AppShell />
}

const rootRoute = createRootRoute({ component: RootComponent })

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/login',
  component: LoginPage,
})

const registerRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/register',
  component: RegisterPage,
})

const appRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: 'app',
  component: RequireAuth,
})

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  beforeLoad: () => {
    throw redirect({ to: '/dashboard' })
  },
})

const dashboardRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/dashboard',
  component: DashboardPage,
})

const onboardingRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/onboarding',
  component: OnboardingPage,
})

const serversRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/servers',
  component: ServersPage,
})

const projectsRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/projects',
  component: ProjectsPage,
})

const applicationsRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/applications',
  component: ApplicationsPage,
})

const applicationDetailRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/applications/$appId',
  component: ApplicationDetailPage,
})

const databasesRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/databases',
  component: DatabasesPage,
})

const servicesRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/services',
  component: ServicesPage,
})

const notificationsRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/notifications',
  component: NotificationsPage,
})

const settingsRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/settings',
  component: SettingsPage,
})

const routeTree = rootRoute.addChildren([
  indexRoute,
  loginRoute,
  registerRoute,
  appRoute.addChildren([
    dashboardRoute,
    onboardingRoute,
    serversRoute,
    projectsRoute,
    applicationsRoute,
    applicationDetailRoute,
    databasesRoute,
    servicesRoute,
    notificationsRoute,
    settingsRoute,
  ]),
])

const router = createRouter({ routeTree })

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
}
