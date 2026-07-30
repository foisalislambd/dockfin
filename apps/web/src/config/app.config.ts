import type { LucideIcon } from 'lucide-react'
import {
  Bell,
  Boxes,
  Database,
  FolderKanban,
  HardDrive,
  KeyRound,
  LayoutDashboard,
  Rocket,
  Server,
  Settings,
  Users,
  Variable,
  Key,
} from 'lucide-react'

export const appConfig = {
  brand: {
    name: 'Goolify',
    tagline: 'Self-hosted PaaS',
    letter: 'G',
    loginDescription:
      'Deploy applications, databases, and one-click services on your own servers over SSH + Docker.',
    loginFeatures: [
      'Servers, projects, and deployments',
      'Databases and service catalog',
      'Works on desktop and mobile',
    ],
  },
}

export type NavItem = {
  name: string
  href: string
  icon: LucideIcon
}

export const navItems: NavItem[] = [
  { name: 'Dashboard', href: '/dashboard', icon: LayoutDashboard },
  { name: 'Projects', href: '/projects', icon: FolderKanban },
  { name: 'Servers', href: '/servers', icon: Server },
  { name: 'Applications', href: '/applications', icon: Rocket },
  { name: 'Databases', href: '/databases', icon: Database },
  { name: 'Services', href: '/services', icon: Boxes },
  { name: 'Storages', href: '/storages', icon: HardDrive },
  { name: 'Shared vars', href: '/shared-variables', icon: Variable },
  { name: 'Team', href: '/team', icon: Users },
  { name: 'Keys', href: '/security/private-keys', icon: KeyRound },
  { name: 'API Tokens', href: '/security/api-tokens', icon: Key },
  { name: 'Notifications', href: '/notifications', icon: Bell },
  { name: 'Settings', href: '/settings', icon: Settings },
]

export function isNavActive(pathname: string, href: string) {
  if (href === '/dashboard') return pathname === '/dashboard' || pathname === '/'
  if (href === '/projects') return pathname === '/projects' || pathname.startsWith('/projects/')
  if (href === '/servers') return pathname === '/servers' || pathname.startsWith('/servers/')
  if (href === '/security/private-keys') {
    return pathname === '/security/private-keys' || pathname.startsWith('/security/private-keys/')
  }
  if (href === '/security/api-tokens') {
    return pathname === '/security/api-tokens' || pathname.startsWith('/security/api-tokens/')
  }
  return pathname === href || pathname.startsWith(`${href}/`)
}
