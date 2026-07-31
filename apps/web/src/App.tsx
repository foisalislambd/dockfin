import {
  Navigate,
  Outlet,
  RouterProvider,
  createRootRoute,
  createRoute,
  createRouter,
  redirect,
} from '@tanstack/react-router'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { AuthProvider, useAuth } from './lib/auth'
import { ThemeProvider } from './components/theme/ThemeProvider'
import { AppShell } from './components/AppShell'
import { AuthSkeleton } from './components/ui/Skeleton'
import { LoginPage, RegisterPage } from './pages/Auth'
import { DashboardPage } from './pages/Dashboard'
import { ServersPage } from './pages/Servers'
import { ProjectsPage } from './pages/Resources'
import { CreateApplicationPage } from './pages/CreateApplication'
import { CreateDatabasePage } from './pages/CreateDatabase'
import { CreateServicePage } from './pages/CreateService'
import { ApplicationDetailPage } from './pages/ApplicationDetail'
import { NotificationsPage } from './pages/Notifications'
import { ProjectShowPage } from './pages/ProjectShow'
import { ProjectEditPage, EnvironmentEditPage } from './pages/ProjectEdit'
import { EnvironmentResourcesPage } from './pages/EnvironmentResources'
import { NewResourcePage } from './pages/NewResource'
import { DatabaseDetailPage, ServerDetailPage } from './pages/ResourceDetails'
import { ServiceDetailPage } from './pages/ServiceDetail'
import { DeploymentShowPage } from './pages/DeploymentShow'
import { SettingsPage } from './pages/Settings'
import { SecurityPage } from './pages/Security'
import {
  SharedVariablesPage,
  StoragesPage,
  TeamPage,
} from './pages/OpsPages'
import { GitSourceDetailPage, GitSourcesPage } from './pages/GitSources'
import './index.css'

const queryClient = new QueryClient()

function RootComponent() {
  return (
    <ThemeProvider>
      <AuthProvider>
        <Outlet />
      </AuthProvider>
    </ThemeProvider>
  )
}

function RequireAuth() {
  const { user, loading } = useAuth()
  if (loading) {
    return <AuthSkeleton />
  }
  if (!user) {
    return <Navigate to="/login" />
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
    throw redirect({ to: '/login' })
  },
})

const dashboardRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/dashboard',
  component: DashboardPage,
})

const serversRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/servers',
  component: ServersPage,
})

const serverDetailRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/servers/$serverId',
  component: ServerDetailPage,
})

const projectsRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/projects',
  component: ProjectsPage,
})

const projectShowRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/projects/$projectId',
  component: ProjectShowPage,
})

const projectEditRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/projects/$projectId/edit',
  component: ProjectEditPage,
})

const envResourcesRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/projects/$projectId/environments/$envId',
  component: EnvironmentResourcesPage,
})

const envEditRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/projects/$projectId/environments/$envId/edit',
  component: EnvironmentEditPage,
})

const newResourceRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/projects/$projectId/environments/$envId/new',
  component: NewResourcePage,
})

const nestedCreateAppRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/projects/$projectId/environments/$envId/applications/new',
  validateSearch: (s: Record<string, unknown>) => ({
    build_pack: typeof s.build_pack === 'string' ? s.build_pack : undefined,
    environment_id: typeof s.environment_id === 'string' ? s.environment_id : undefined,
    source_type: typeof s.source_type === 'string' ? s.source_type : undefined,
  }),
  component: CreateApplicationPage,
})

const nestedAppDetailRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/projects/$projectId/environments/$envId/applications/$appId',
  component: ApplicationDetailPage,
})

const nestedDeploymentRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/projects/$projectId/environments/$envId/applications/$appId/deployments/$deploymentId',
  component: DeploymentShowPage,
})

const nestedCreateDbRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/projects/$projectId/environments/$envId/databases/new',
  validateSearch: (s: Record<string, unknown>) => ({
    engine: typeof s.engine === 'string' ? s.engine : undefined,
    environment_id: typeof s.environment_id === 'string' ? s.environment_id : undefined,
  }),
  component: CreateDatabasePage,
})

const nestedDbDetailRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/projects/$projectId/environments/$envId/databases/$dbId',
  component: DatabaseDetailPage,
})

const nestedCreateSvcRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/projects/$projectId/environments/$envId/services/new',
  component: CreateServicePage,
})

const nestedSvcDetailRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/projects/$projectId/environments/$envId/services/$svcId',
  component: ServiceDetailPage,
})

const applicationsRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/applications',
  beforeLoad: () => {
    throw redirect({ to: '/projects' })
  },
})

const createApplicationRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/applications/new',
  beforeLoad: () => {
    throw redirect({ to: '/projects' })
  },
})

const applicationDetailRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/applications/$appId',
  component: ApplicationDetailPage,
})

const deploymentShowRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/applications/$appId/deployments/$deploymentId',
  component: DeploymentShowPage,
})

const databasesRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/databases',
  beforeLoad: () => {
    throw redirect({ to: '/projects' })
  },
})

const createDatabaseRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/databases/new',
  beforeLoad: () => {
    throw redirect({ to: '/projects' })
  },
})

const databaseDetailRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/databases/$dbId',
  component: DatabaseDetailPage,
})

const servicesRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/services',
  beforeLoad: () => {
    throw redirect({ to: '/projects' })
  },
})

const createServiceRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/services/new',
  beforeLoad: () => {
    throw redirect({ to: '/projects' })
  },
})

const serviceDetailRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/services/$svcId',
  component: ServiceDetailPage,
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

const storagesRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/storages',
  component: StoragesPage,
})

const teamRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/team',
  component: TeamPage,
})

const sharedVariablesRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/shared-variables',
  component: SharedVariablesPage,
})

const securityRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/security',
  validateSearch: (search: Record<string, unknown>): { tab?: string } => ({
    tab: typeof search.tab === 'string' ? search.tab : undefined,
  }),
  component: SecurityPage,
})

const privateKeysRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/security/private-keys',
  beforeLoad: () => {
    throw redirect({ to: '/security', search: { tab: 'private-keys' } })
  },
})

const apiTokensRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/security/api-tokens',
  beforeLoad: () => {
    throw redirect({ to: '/security', search: { tab: 'api-tokens' } })
  },
})

const gitSourcesRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/git-sources',
  component: GitSourcesPage,
})

const gitSourceDetailRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/git-sources/$sourceId',
  component: GitSourceDetailPage,
})

const routeTree = rootRoute.addChildren([
  indexRoute,
  loginRoute,
  registerRoute,
  appRoute.addChildren([
    dashboardRoute,
    serversRoute,
    serverDetailRoute,
    projectsRoute,
    projectShowRoute,
    projectEditRoute,
    envResourcesRoute,
    envEditRoute,
    newResourceRoute,
    nestedCreateAppRoute,
    nestedAppDetailRoute,
    nestedDeploymentRoute,
    nestedCreateDbRoute,
    nestedDbDetailRoute,
    nestedCreateSvcRoute,
    nestedSvcDetailRoute,
    applicationsRoute,
    createApplicationRoute,
    applicationDetailRoute,
    deploymentShowRoute,
    databasesRoute,
    createDatabaseRoute,
    databaseDetailRoute,
    servicesRoute,
    createServiceRoute,
    serviceDetailRoute,
    notificationsRoute,
    settingsRoute,
    storagesRoute,
    teamRoute,
    sharedVariablesRoute,
    securityRoute,
    privateKeysRoute,
    gitSourcesRoute,
    gitSourceDetailRoute,
    apiTokensRoute,
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
