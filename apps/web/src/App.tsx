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
import { ApplicationsPage, DatabasesPage, ProjectsPage, ServicesPage } from './pages/Resources'
import { CreateApplicationPage } from './pages/CreateApplication'
import { CreateDatabasePage } from './pages/CreateDatabase'
import { CreateServicePage } from './pages/CreateService'
import { ApplicationDetailPage } from './pages/ApplicationDetail'
import { OnboardingPage } from './pages/Onboarding'
import { NotificationsPage } from './pages/Notifications'
import { ProjectShowPage } from './pages/ProjectShow'
import { EnvironmentResourcesPage } from './pages/EnvironmentResources'
import { NewResourcePage } from './pages/NewResource'
import { DatabaseDetailPage, ServerDetailPage, ServiceDetailPage } from './pages/ResourceDetails'
import { DeploymentShowPage } from './pages/DeploymentShow'
import {
  PrivateKeysPage,
  ApiTokensPage,
  SettingsPage,
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

const envResourcesRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/projects/$projectId/environments/$envId',
  component: EnvironmentResourcesPage,
})

const newResourceRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/projects/$projectId/environments/$envId/new',
  component: NewResourcePage,
})

const nestedCreateAppRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/projects/$projectId/environments/$envId/applications/new',
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
  component: ApplicationsPage,
})

const createApplicationRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/applications/new',
  component: CreateApplicationPage,
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
  component: DatabasesPage,
})

const createDatabaseRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/databases/new',
  component: CreateDatabasePage,
})

const databaseDetailRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/databases/$dbId',
  component: DatabaseDetailPage,
})

const servicesRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/services',
  component: ServicesPage,
})

const createServiceRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/services/new',
  component: CreateServicePage,
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

const privateKeysRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/security/private-keys',
  component: PrivateKeysPage,
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

const apiTokensRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/security/api-tokens',
  component: ApiTokensPage,
})

const routeTree = rootRoute.addChildren([
  indexRoute,
  loginRoute,
  registerRoute,
  appRoute.addChildren([
    dashboardRoute,
    onboardingRoute,
    serversRoute,
    serverDetailRoute,
    projectsRoute,
    projectShowRoute,
    envResourcesRoute,
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
