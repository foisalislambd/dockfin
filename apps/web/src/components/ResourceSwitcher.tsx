import { useMemo, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { ChevronsUpDown } from 'lucide-react'
import { ServiceLogo, logoForApplication, logoForDatabaseEngine, logoForServiceType } from './ServiceLogo'
import { api } from '../lib/api'

type Kind = 'application' | 'service' | 'database'

export function ResourceSwitcher({
  kind,
  currentId,
  environmentId,
  projectId,
}: {
  kind: Kind
  currentId: string
  environmentId?: string
  projectId?: string
}) {
  const nav = useNavigate()
  const [open, setOpen] = useState(false)
  const [q, setQ] = useState('')
  const apps = useQuery({
    queryKey: ['applications', environmentId],
    queryFn: () => api.applications(environmentId),
    enabled: open && kind === 'application' && !!environmentId,
  })
  const svcs = useQuery({
    queryKey: ['services', environmentId],
    queryFn: () => api.services(environmentId),
    enabled: open && kind === 'service' && !!environmentId,
  })
  const templates = useQuery({
    queryKey: ['templates'],
    queryFn: api.templates,
    staleTime: 30 * 60 * 1000,
    enabled: open && kind === 'service',
  })
  const dbs = useQuery({
    queryKey: ['databases', environmentId],
    queryFn: () => api.databases(environmentId),
    enabled: open && kind === 'database' && !!environmentId,
  })

  const items = useMemo(() => {
    const needle = q.trim().toLowerCase()
    const catalog = templates.data?.templates || []
    const list =
      kind === 'application'
        ? (apps.data?.applications || []).map((a) => ({
            id: a.id,
            name: a.name,
            logo: logoForApplication(a.build_pack, a.git_source_id),
          }))
        : kind === 'service'
          ? (svcs.data?.services || []).map((s) => ({
              id: s.id,
              name: s.name,
              logo: logoForServiceType(s.service_type, catalog),
            }))
          : (dbs.data?.databases || []).map((d) => ({
              id: d.id,
              name: d.name,
              logo: logoForDatabaseEngine(d.engine),
            }))
    return needle ? list.filter((x) => x.name.toLowerCase().includes(needle)) : list
  }, [apps.data, svcs.data, dbs.data, templates.data, kind, q])

  if (!environmentId || !projectId) return null

  const go = (id: string) => {
    setOpen(false)
    setQ('')
    if (id === currentId) return
    const keepNav = ((prev: Record<string, unknown>) => {
      const { deploy: _d, ...rest } = prev
      return rest
    }) as never
    if (kind === 'application') {
      void nav({
        to: '/projects/$projectId/environments/$envId/applications/$appId',
        params: { projectId, envId: environmentId, appId: id },
        search: keepNav,
      })
    } else if (kind === 'service') {
      void nav({
        to: '/projects/$projectId/environments/$envId/services/$svcId',
        params: { projectId, envId: environmentId, svcId: id },
        search: keepNav,
      })
    } else {
      void nav({
        to: '/projects/$projectId/environments/$envId/databases/$dbId',
        params: { projectId, envId: environmentId, dbId: id },
        search: keepNav,
      })
    }
  }

  return (
    <div className="relative">
      <button
        type="button"
        title="Switch resource"
        onClick={() => setOpen((v) => !v)}
        className="inline-flex h-7 items-center gap-1 rounded-md border border-gray-200 px-1.5 text-gray-500 hover:bg-gray-50 dark:border-gray-700 dark:hover:bg-white/5"
      >
        <ChevronsUpDown className="h-3.5 w-3.5" aria-hidden />
        <span className="sr-only">Switch {kind}</span>
      </button>
      {open && (
        <div className="absolute left-0 z-30 mt-1 w-64 rounded-lg border border-gray-200 bg-white p-2 shadow-lg dark:border-gray-800 dark:bg-gray-950">
          <input
            autoFocus
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder={`Search ${kind}s…`}
            className="panel-field mb-2 h-8 w-full rounded-md px-2 text-sm"
          />
          <ul className="max-h-56 overflow-auto text-sm">
            {items.map((item) => (
              <li key={item.id}>
                <button
                  type="button"
                  onClick={() => go(item.id)}
                  className={`block w-full rounded-md px-2 py-1.5 text-left ${
                    item.id === currentId
                      ? 'bg-brand-500/10 font-medium text-brand-700 dark:text-brand-300'
                      : 'text-gray-800 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-white/5'
                  }`}
                >
                  <span className="flex items-center gap-2">
                    <ServiceLogo src={item.logo} name={item.name} className="h-5 w-5" />
                    <span className="truncate">{item.name}</span>
                  </span>
                </button>
              </li>
            ))}
            {!items.length && (
              <li className="px-2 py-3 text-center text-xs text-gray-500">No matches</li>
            )}
          </ul>
        </div>
      )}
    </div>
  )
}
