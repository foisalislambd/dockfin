import type { LucideIcon } from 'lucide-react'
import {
  Bell,
  Box,
  ClipboardList,
  FolderKanban,
  GitBranch,
  KeyRound,
  LayoutDashboard,
  Server,
  Settings,
  Share2,
  SquareTerminal,
  Tags,
  Users,
  Waypoints,
} from 'lucide-react'

export const appConfig = {
  brand: {
    name: 'Dockfin',
    tagline: 'Self-hosted PaaS',
    letter: 'D',
    /** Full logo (icon + wordmark) — dark surfaces */
    logo: '/brand/dockfin-logo.png',
    /** Full logo for light surfaces */
    logoLight: '/brand/dockfin-logo-light.png',
    /** Icon mark — dark surfaces */
    mark: '/brand/dockfin-mark.png',
    /** Icon mark for light surfaces */
    markLight: '/brand/dockfin-mark-light.png',
    license: 'MIT',
    copyright: '© 2026 Dockfin Contributors',
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

Copyright (c) 2026 Dockfin Contributors

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

export type NavGroup = {
  id: string
  label: string
  items: NavItem[]
}

export const navGroups: NavGroup[] = [
  {
    id: 'workspace',
    label: 'Workspace',
    items: [
      { name: 'Dashboard', href: '/dashboard', icon: LayoutDashboard },
      { name: 'Projects', href: '/projects', icon: FolderKanban },
      { name: 'Terminal', href: '/terminal', icon: SquareTerminal },
    ],
  },
  {
    id: 'infrastructure',
    label: 'Infrastructure',
    items: [
      { name: 'Servers', href: '/servers', icon: Server },
      { name: 'Sources', href: '/git-sources', icon: GitBranch },
      { name: 'Destinations', href: '/destinations', icon: Waypoints },
      { name: 'Storages', href: '/storages', icon: Box },
      { name: 'Shared vars', href: '/shared-variables', icon: Share2 },
    ],
  },
  {
    id: 'manage',
    label: 'Manage',
    items: [
      { name: 'Team', href: '/team', icon: Users },
      { name: 'Audit', href: '/audit', icon: ClipboardList },
      { name: 'Notifications', href: '/notifications', icon: Bell },
      { name: 'Keys & Tokens', href: '/security', icon: KeyRound },
      { name: 'Tags', href: '/tags', icon: Tags },
      { name: 'Settings', href: '/settings', icon: Settings },
    ],
  },
]

export const navItems: NavItem[] = navGroups.flatMap((g) => g.items)

export function isNavActive(pathname: string, href: string) {
  if (href === '/dashboard') return pathname === '/dashboard' || pathname === '/'
  if (href === '/projects') return pathname === '/projects' || pathname.startsWith('/projects/')
  if (href === '/servers') return pathname === '/servers' || pathname.startsWith('/servers/')
  if (href === '/git-sources') {
    return pathname === '/git-sources' || pathname.startsWith('/git-sources/')
  }
  if (href === '/security') {
    return pathname === '/security' || pathname.startsWith('/security/')
  }
  return pathname === href || pathname.startsWith(`${href}/`)
}
