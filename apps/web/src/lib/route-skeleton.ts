/** Which full-page skeleton to show while a lazy route chunk (or auth) loads. */

export type RouteSkeletonKind =
  | 'form'
  | 'choice'
  | 'app'
  | 'app-settings'
  | 'service'
  | 'service-settings'
  | 'resource'
  | 'simple'
  | 'env'
  | 'project'
  | 'dashboard'
  | 'tabbed'
  | 'list'
  | 'list-3'
  | 'list-1'

export function tabFromSearch(search: unknown): string {
  if (typeof search === 'string') {
    const q = search.startsWith('?') ? search.slice(1) : search
    return new URLSearchParams(q).get('tab') || ''
  }
  if (search && typeof search === 'object' && 'tab' in search) {
    const t = (search as { tab?: unknown }).tab
    return typeof t === 'string' ? t : ''
  }
  return ''
}

function pathOnly(pathname: string) {
  return pathname.replace(/\/+$/, '') || '/'
}

/**
 * Ordered like App.tsx routes: more specific paths first.
 * Application/service details must never fall through to the project-list skeleton.
 */
export function routeSkeletonKind(pathname: string, search?: unknown): RouteSkeletonKind {
  const p = pathOnly(pathname)
  const tab = tabFromSearch(search)

  if (p === '/notifications' || p === '/settings' || p === '/security' || p.startsWith('/security/')) {
    return 'tabbed'
  }

  if (
    /\/(applications|databases|services)\/new$/.test(p) ||
    /\/servers\/new$/.test(p) ||
    /\/storages\/new$/.test(p) ||
    /\/edit$/.test(p)
  ) {
    return 'form'
  }

  if (/\/environments\/[^/]+\/new$/.test(p)) return 'choice'

  if (/\/deployments\/[^/]+$/.test(p)) return 'simple'

  if (/\/applications\/[^/]+$/.test(p)) {
    return tab === 'configuration' ? 'app-settings' : 'app'
  }
  if (/\/services\/[^/]+$/.test(p)) {
    return tab === 'configuration' ? 'service-settings' : 'service'
  }

  if (/\/(databases|servers|git-sources)\/[^/]+$/.test(p)) return 'resource'

  if (/\/shared-variables$/.test(p)) return 'list'

  if (/\/projects\/[^/]+\/environments\/[^/]+$/.test(p)) return 'env'
  if (/\/projects\/[^/]+$/.test(p)) return 'project'

  if (p === '/dashboard') return 'dashboard'
  if (p === '/projects') return 'list'
  if (p === '/storages') return 'list-3'
  if (p === '/terminal' || p === '/audit') return 'list-1'

  return 'list'
}
