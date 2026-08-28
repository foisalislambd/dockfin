import { useTheme } from 'next-themes'
import { useState, useSyncExternalStore } from 'react'
import { cn } from '../lib/cn'

type Props = {
  src?: string | null
  name: string
  className?: string
  imgClassName?: string
}

/**
 * Monochrome logos designed for light backgrounds (dark fills).
 * In dark mode we invert so they stay visible on dark tiles.
 */
const MONOCHROME_FOR_LIGHT = new Set([
  'github.svg',
  'mariadb.svg',
  'clickhouse.svg',
  'clickhouse-icon.svg',
  'dragonfly.svg',
  'keydb.svg',
  'ghost.svg',
  'umami.svg',
  'listmonk.svg',
  'classicpress.svg',
  'leantime.svg',
  'denoKV.svg',
  'denokv.svg',
  'opentelemetry.svg',
  'paperless.svg',
  'orangehrm.svg',
  'openpanel.svg',
  'external-link.svg',
  'internal-link.svg',
  'cloudbeaver.svg',
  'code-server.svg',
  'convex.svg',
  'ente-photos.svg',
  'excalidraw.svg',
  'jitsi.svg',
  'logto_dark.svg',
  'rybbit.svg',
  'sequin.svg',
  'signoz.svg',
  'syncthing.svg',
])

function logoBasename(src: string): string {
  const path = src.split('?')[0] || src
  const parts = path.split('/')
  return (parts[parts.length - 1] || '').toLowerCase()
}

/** Coolify `svgs/n8n.png` → public `/svgs/n8n.png`. */
export function catalogLogoUrl(logo?: string | null): string | undefined {
  const s = (logo || '').trim()
  if (!s) return undefined
  if (/^https?:\/\//i.test(s) || s.startsWith('/')) return s
  return `/svgs/${s.replace(/^svgs\//, '')}`
}

export function logoForServiceType(
  serviceType: string | undefined,
  templates: Array<{ type: string; logo?: string }>,
): string | undefined {
  if (!serviceType) return undefined
  if (serviceType === 'custom') return '/svgs/docker.svg'
  const t = templates.find((x) => x.type === serviceType)
  const fromTpl = catalogLogoUrl(t?.logo)
  if (fromTpl) return fromTpl
  if (templates.length === 0) return undefined
  const base = serviceType.split(/-with-|-and-/)[0] || serviceType
  return catalogLogoUrl(`${base}.svg`)
}

const DB_ENGINE_LOGOS: Record<string, string> = {
  postgresql: '/svgs/postgresql.svg',
  postgres: '/svgs/postgresql.svg',
  mysql: '/svgs/mysql.svg',
  mariadb: '/svgs/mariadb.svg',
  redis: '/svgs/redis.svg',
  keydb: '/svgs/keydb.svg',
  dragonfly: '/svgs/dragonfly.svg',
  mongodb: '/svgs/mongodb.svg',
  clickhouse: '/svgs/clickhouse.svg',
}

export function logoForDatabaseEngine(engine?: string): string | undefined {
  const k = (engine || '').toLowerCase().replace(/\s+/g, '')
  return DB_ENGINE_LOGOS[k] || catalogLogoUrl(k ? `${k}.svg` : '')
}

export function logoForApplication(buildPack?: string, gitSourceId?: string | null): string {
  const p = (buildPack || '').toLowerCase()
  if (p.includes('compose') || p.includes('docker')) return '/svgs/docker.svg'
  if (gitSourceId) return '/svgs/github.svg'
  return '/svgs/git.svg'
}

/** Service catalog / list logo with letter fallback when image is missing. */
export function ServiceLogo({ src, name, className, imgClassName }: Props) {
  const [failed, setFailed] = useState(false)
  const { resolvedTheme } = useTheme()
  const mounted = useSyncExternalStore(
    () => () => {},
    () => true,
    () => false,
  )
  const letter = (name || '?').trim().charAt(0).toUpperCase() || '?'
  const base = src ? logoBasename(src) : ''
  const monochrome = Boolean(base && MONOCHROME_FOR_LIGHT.has(base))
  const dark = mounted && resolvedTheme === 'dark'

  if (!src || failed) {
    return (
      <span
        className={cn(
          'inline-flex shrink-0 items-center justify-center rounded-lg bg-brand-50 text-sm font-semibold text-brand-600 dark:bg-brand-500/15 dark:text-brand-300',
          className,
        )}
        aria-hidden
      >
        {letter}
      </span>
    )
  }

  return (
    <span
      className={cn(
        'inline-flex shrink-0 items-center justify-center overflow-hidden rounded-lg bg-gray-50 dark:bg-white/10',
        className,
      )}
    >
      <img
        src={src}
        alt=""
        loading="lazy"
        decoding="async"
        className={cn(
          'h-full w-full object-contain p-1',
          // Dark fills for light UI; invert on dark tiles so logos stay visible.
          monochrome && dark && 'invert',
          imgClassName,
        )}
        onError={() => setFailed(true)}
      />
    </span>
  )
}
