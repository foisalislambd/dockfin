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
  /** Second key of the `g` then key go-to shortcut. */
  shortcut?: string
  description?: string
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
      { name: 'Dashboard', href: '/dashboard', icon: LayoutDashboard, shortcut: 'o', description: 'Health and first-run setup' },
      { name: 'Projects', href: '/projects', icon: FolderKanban, shortcut: 'p', description: 'Apps, databases, and services' },
      { name: 'Terminal', href: '/terminal', icon: SquareTerminal, shortcut: 't', description: 'Shell on a server' },
    ],
  },
  {
    id: 'infrastructure',
    label: 'Infrastructure',
    items: [
      { name: 'Servers', href: '/servers', icon: Server, shortcut: 'h', description: 'SSH hosts and proxies' },
      { name: 'Sources', href: '/git-sources', icon: GitBranch, shortcut: 'c', description: 'GitHub and Git providers' },
      { name: 'Destinations', href: '/destinations', icon: Waypoints, shortcut: 'd', description: 'Docker networks' },
      { name: 'Storages', href: '/storages', icon: Box, shortcut: 'r', description: 'S3 backup destinations' },
      { name: 'Shared vars', href: '/shared-variables', icon: Share2, shortcut: 'v', description: 'Environment variables shared across resources' },
    ],
  },
  {
    id: 'manage',
    label: 'Manage',
    items: [
      { name: 'Team', href: '/team', icon: Users, shortcut: 'u', description: 'Members and invitations' },
      { name: 'Audit', href: '/audit', icon: ClipboardList, shortcut: 'a', description: 'Who changed what' },
      { name: 'Notifications', href: '/notifications', icon: Bell, shortcut: 'n', description: 'Email, Slack, and webhooks' },
      { name: 'Keys & Tokens', href: '/security', icon: KeyRound, shortcut: 'k', description: 'SSH keys, API tokens, cloud tokens' },
      { name: 'Tags', href: '/tags', icon: Tags, shortcut: 'l', description: 'Labels across resources' },
      { name: 'Settings', href: '/settings', icon: Settings, shortcut: 's', description: 'Instance, OAuth, backups, registries' },
    ],
  },
]

export const navItems: NavItem[] = navGroups.flatMap((g) => g.items)

/** `g` then this key jumps to the page. */
export function goKeyHref(key: string): string | null {
  const hit = navItems.find((item) => item.shortcut === key.toLowerCase())
  return hit?.href ?? null
}

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
