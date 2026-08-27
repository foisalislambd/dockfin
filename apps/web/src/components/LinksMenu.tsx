import { useEffect, useRef, useState } from 'react'
import { safeExternalHref } from '../lib/url'

export type ResourceLink = {
  label: string
  url: string
}

/** Links dropdown — open public URLs in a new tab. */
export function LinksMenu({ links, className = '' }: { links: ResourceLink[]; className?: string }) {
  const [open, setOpen] = useState(false)
  const root = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const onDoc = (e: MouseEvent) => {
      if (!root.current?.contains(e.target as Node)) setOpen(false)
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false)
    }
    document.addEventListener('mousedown', onDoc)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onDoc)
      document.removeEventListener('keydown', onKey)
    }
  }, [open])

  const items = links || []

  return (
    <div ref={root} className={`relative inline-block ${className}`}>
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="inline-flex h-8 items-center gap-1.5 rounded-md border border-gray-200 bg-white px-2.5 text-xs font-medium text-gray-800 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-100 dark:hover:bg-gray-800"
        aria-haspopup="menu"
        aria-expanded={open}
      >
        Links
        <svg className="h-3.5 w-3.5 opacity-60" viewBox="0 0 20 20" fill="currentColor" aria-hidden>
          <path
            fillRule="evenodd"
            d="M5.23 7.21a.75.75 0 011.06.02L10 11.17l3.71-3.94a.75.75 0 111.08 1.04l-4.25 4.5a.75.75 0 01-1.08 0l-4.25-4.5a.75.75 0 01.02-1.06z"
            clipRule="evenodd"
          />
        </svg>
      </button>
      {open && (
        <div
          role="menu"
          className="absolute right-0 z-40 mt-1.5 min-w-[16rem] max-w-[min(24rem,90vw)] overflow-hidden rounded-lg border border-gray-200 bg-white py-1 shadow-lg dark:border-gray-700 dark:bg-gray-900"
        >
          {items.length === 0 ? (
            <div className="px-3 py-2 text-xs text-gray-500 dark:text-gray-400">No links available</div>
          ) : (
            items.map((l) => {
              const href = safeExternalHref(l.url)
              const inner = (
                <>
                <ExternalIcon className="mt-0.5 h-3.5 w-3.5 shrink-0 text-brand-600 dark:text-brand-400" />
                <span className="min-w-0">
                  <span className="block text-xs font-medium text-gray-500 dark:text-gray-400">{l.label}</span>
                  <span className="block truncate font-mono text-xs">{l.url}</span>
                </span>
                </>
              )
              if (!href) {
                return (
                  <div key={l.url} role="menuitem" className="flex items-start gap-2 px-3 py-2 text-sm text-gray-800 dark:text-gray-100">
                    {inner}
                  </div>
                )
              }
              return (
              <a
                key={l.url}
                role="menuitem"
                href={href}
                target="_blank"
                rel="noreferrer"
                className="flex items-start gap-2 px-3 py-2 text-sm text-gray-800 hover:bg-gray-50 dark:text-gray-100 dark:hover:bg-gray-800"
                onClick={() => setOpen(false)}
              >
                {inner}
              </a>
              )
            })
          )}
        </div>
      )}
    </div>
  )
}

export function LinksPanel({ links }: { links: ResourceLink[] }) {
  const items = links || []
  const hasLocalhost = items.some(
    (l) =>
      l.url.includes('127.0.0.1') ||
      l.url.includes('localhost') ||
      l.url.includes('.0.0.0.0.'),
  )
  // https + sslip is rate-limited by Let's Encrypt.
  const hasSslipHttps = items.some(
    (l) => l.url.startsWith('https://') && l.url.includes('sslip'),
  )
  if (!items.length) {
    return (
      <div className="panel-card p-6 text-sm text-gray-500 dark:text-gray-400">
        No public links yet. Assign a free domain (sslip.io / nip.io) or custom FQDN, deploy the
        service, and start the proxy on the server.
      </div>
    )
  }
  return (
    <div className="space-y-2">
      <p className="text-sm text-gray-500 dark:text-gray-400">
        Open these URLs to access your deployed service (proxy must be running).
      </p>
      {hasLocalhost && (
        <div className="rounded-lg border border-amber-500/40 bg-amber-500/10 px-3 py-2 text-sm text-amber-800 dark:text-amber-200">
          This link uses <code>127.0.0.1</code> / localhost — your browser will try to open your own
          PC, not the server. Set the server&apos;s <strong>Public IP</strong> (Servers → Settings),
          run Validate, then Redeploy this service.
        </div>
      )}
      {hasSslipHttps && (
        <div className="rounded-lg border border-amber-500/40 bg-amber-500/10 px-3 py-2 text-sm text-amber-800 dark:text-amber-200">
          sslip.io with <strong>https</strong> is not recommended (Let&apos;s Encrypt rate-limits this
          public domain). Free domains use <code>http://</code> — for HTTPS, use your own domain.
        </div>
      )}
      <ul className="divide-y divide-gray-200 overflow-hidden rounded-lg border border-gray-200 dark:divide-gray-800 dark:border-gray-800">
        {items.map((l) => (
          <li
            key={l.url}
            className="flex flex-wrap items-center justify-between gap-3 bg-white px-4 py-3 dark:bg-gray-950"
          >
            <div className="min-w-0">
              <div className="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                {l.label}
              </div>
              <div className="mt-0.5 truncate font-mono text-sm text-gray-900 dark:text-white">{l.url}</div>
            </div>
            {safeExternalHref(l.url) ? (
            <a
              href={safeExternalHref(l.url)}
              target="_blank"
              rel="noreferrer"
              className="inline-flex h-8 shrink-0 items-center gap-1.5 rounded-md bg-brand-600 px-2.5 text-xs font-medium text-white hover:bg-brand-500"
            >
              Visit
              <ExternalIcon className="h-3.5 w-3.5" />
            </a>
            ) : null}
          </li>
        ))}
      </ul>
    </div>
  )
}

function ExternalIcon({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 20 20" fill="currentColor" aria-hidden>
      <path
        fillRule="evenodd"
        d="M4.25 5.5A.75.75 0 015 4.75h5.5a.75.75 0 010 1.5H6.56l7.22 7.22a.75.75 0 11-1.06 1.06L5.5 7.31v2.69a.75.75 0 01-1.5 0v-5.5z"
        clipRule="evenodd"
      />
      <path
        fillRule="evenodd"
        d="M10.75 4.75a.75.75 0 01.75-.75h4.5a.75.75 0 01.75.75v4.5a.75.75 0 01-1.5 0V6.56l-7.22 7.22a.75.75 0 11-1.06-1.06l7.22-7.22H11.5a.75.75 0 01-.75-.75z"
        clipRule="evenodd"
      />
    </svg>
  )
}
