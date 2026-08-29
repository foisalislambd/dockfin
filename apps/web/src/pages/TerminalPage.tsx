import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { Monitor, Server, TerminalSquare } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { ServerTerminal } from '../components/Terminal'
import { PageSkeleton } from '../components/ui/Skeleton'
import { LAST_TERM_SERVER_KEY, api } from '../lib/api'

export function TerminalPickerPage() {
  const servers = useQuery({ queryKey: ['servers'], queryFn: api.servers })
  const usable = useMemo(
    () => (servers.data?.servers || []).filter((s) => s.is_usable),
    [servers.data],
  )
  const [serverId, setServerId] = useState('')

  useEffect(() => {
    if (!usable.length) {
      setServerId('')
      return
    }
    setServerId((cur) => {
      if (cur && usable.some((s) => s.id === cur)) return cur
      const saved = localStorage.getItem(LAST_TERM_SERVER_KEY) || ''
      if (saved && usable.some((s) => s.id === saved)) return saved
      return usable[0].id
    })
  }, [usable])

  const server = usable.find((s) => s.id === serverId)

  const pick = (id: string) => {
    setServerId(id)
    localStorage.setItem(LAST_TERM_SERVER_KEY, id)
  }

  if (servers.isLoading) return <PageSkeleton cards={1} />

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col gap-3">
      <div className="flex min-w-0 flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex min-w-0 items-center gap-2.5">
          <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-brand-500/10 text-brand-600 dark:text-brand-400">
            <TerminalSquare className="h-5 w-5" />
          </span>
          <div className="min-w-0">
            <h1 className="text-lg font-semibold tracking-tight text-gray-900 dark:text-white">
              Terminal
            </h1>
            <p className="truncate text-xs text-gray-500 dark:text-gray-400">
              SSH session on a connected server
            </p>
          </div>
        </div>
        {server ? (
          <Link
            to="/servers/$serverId"
            params={{ serverId: server.id }}
            className="hidden text-xs text-brand-600 hover:underline sm:inline dark:text-brand-400"
          >
            Server details
          </Link>
        ) : null}
      </div>

      {!usable.length ? (
        <div className="flex flex-1 flex-col items-center justify-center rounded-xl border border-dashed border-gray-200 px-6 py-16 text-center dark:border-gray-800">
          <Server className="mb-3 h-8 w-8 text-gray-400" />
          <p className="text-sm font-medium text-gray-900 dark:text-white">No usable servers</p>
          <p className="mt-1 max-w-sm text-sm text-gray-500 dark:text-gray-400">
            Add a server with a working Docker connection to open a shell.
          </p>
          <Link
            to="/servers"
            className="mt-4 inline-flex h-8 items-center rounded-md bg-brand-500 px-3 text-xs font-medium text-white hover:bg-brand-600"
          >
            Add a server
          </Link>
        </div>
      ) : (
        <>
          <div className="flex gap-2 overflow-x-auto pb-0.5 [-ms-overflow-style:none] [scrollbar-width:none] sm:flex-wrap sm:overflow-visible [&::-webkit-scrollbar]:hidden">
            {usable.map((s) => {
              const active = s.id === serverId
              return (
                <button
                  key={s.id}
                  type="button"
                  onClick={() => pick(s.id)}
                  className={`inline-flex max-w-[85vw] shrink-0 items-center gap-2 rounded-lg border px-3 py-2 text-left transition sm:max-w-xs ${
                    active
                      ? 'border-brand-500 bg-brand-50 dark:border-brand-400 dark:bg-brand-500/10'
                      : 'border-gray-200 hover:bg-gray-50 dark:border-gray-800 dark:hover:bg-white/5'
                  }`}
                >
                  <Monitor
                    className={`h-4 w-4 shrink-0 ${active ? 'text-brand-600 dark:text-brand-400' : 'text-gray-400'}`}
                  />
                  <span className="min-w-0">
                    <span className="block truncate text-sm font-medium text-gray-900 dark:text-white">
                      {s.name}
                    </span>
                    <span className="block truncate font-mono text-[11px] text-gray-500 dark:text-gray-400">
                      {s.user_name}@{s.ip}
                    </span>
                  </span>
                </button>
              )
            })}
          </div>

          {server ? (
            <div className="flex min-h-0 min-w-0 flex-1 flex-col">
              <ServerTerminal key={server.id} serverId={server.id} fill />
            </div>
          ) : null}

          {server ? (
            <Link
              to="/servers/$serverId"
              params={{ serverId: server.id }}
              className="text-xs text-brand-600 hover:underline sm:hidden dark:text-brand-400"
            >
              Open server details
            </Link>
          ) : null}
        </>
      )}
    </div>
  )
}
