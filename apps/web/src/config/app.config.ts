import type { LucideIcon } from 'lucide-react'
import {
  Bell,
  Boxes,
  Database,
  FolderKanban,
  LayoutDashboard,
  Rocket,
  Server,
  Settings,
  Sparkles,
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
  { name: 'Onboarding', href: '/onboarding', icon: Sparkles },
  { name: 'Servers', href: '/servers', icon: Server },
  { name: 'Projects', href: '/projects', icon: FolderKanban },
  { name: 'Applications', href: '/applications', icon: Rocket },
  { name: 'Databases', href: '/databases', icon: Database },
  { name: 'Services', href: '/services', icon: Boxes },
  { name: 'Notifications', href: '/notifications', icon: Bell },
  { name: 'Settings', href: '/settings', icon: Settings },
]

export function isNavActive(pathname: string, href: string) {
  if (href === '/dashboard') return pathname === '/dashboard' || pathname === '/'
  return pathname === href || pathname.startsWith(`${href}/`)
}
