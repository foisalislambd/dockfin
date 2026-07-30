import type { LucideIcon } from 'lucide-react'
import {
  Bell,
  Boxes,
  Database,
  FolderKanban,
  HardDrive,
  GitBranch,
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
    license: 'MIT',
    copyright: '© 2026 Goolify Contributors',
    loginDescription:
      'Deploy applications, databases, and one-click services on your own servers over SSH + Docker.',
    loginFeatures: [
      'Servers, projects, and deployments',
      'Databases and service catalog',
      'Works on desktop and mobile',
    ],
  },
}

export const MIT_LICENSE_TEXT = `MIT License

Copyright (c) 2026 Goolify Contributors

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.`

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
  { name: 'Git Sources', href: '/git-sources', icon: GitBranch },
  { name: 'Keys', href: '/security/private-keys', icon: KeyRound },
  { name: 'API Tokens', href: '/security/api-tokens', icon: Key },
  { name: 'Notifications', href: '/notifications', icon: Bell },
  { name: 'Settings', href: '/settings', icon: Settings },
]

export function isNavActive(pathname: string, href: string) {
  if (href === '/dashboard') return pathname === '/dashboard' || pathname === '/'
  if (href === '/projects') return pathname === '/projects' || pathname.startsWith('/projects/')
  if (href === '/servers') return pathname === '/servers' || pathname.startsWith('/servers/')
  if (href === '/git-sources') {
    return pathname === '/git-sources' || pathname.startsWith('/git-sources/')
  }
  if (href === '/security/private-keys') {
    return pathname === '/security/private-keys' || pathname.startsWith('/security/private-keys/')
  }
  if (href === '/security/api-tokens') {
    return pathname === '/security/api-tokens' || pathname.startsWith('/security/api-tokens/')
  }
  return pathname === href || pathname.startsWith(`${href}/`)
}
