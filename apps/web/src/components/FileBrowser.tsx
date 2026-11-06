import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { api, type ContainerFile } from '../lib/api'
import { Btn } from './ui/Button'

type Kind = 'applications' | 'services' | 'databases'

export function FileBrowser({ kind, id }: { kind: Kind; id: string }) {
  const qc = useQueryClient()
  const containers = useQuery({
    queryKey: ['containers', kind, id],
    queryFn: () =>
      kind === 'applications'
        ? api.applicationContainers(id)
        : kind === 'services'
          ? api.serviceContainers(id)
          : Promise.resolve({ containers: [] as string[] }),
    enabled: kind !== 'databases',
  })
  const names = containers.data?.containers || []
  const [container, setContainer] = useState('')
  const active = container || names[0] || ''
  const [path, setPath] = useState('/')
  const [draft, setDraft] = useState('')
  const [openFile, setOpenFile] = useState<string | null>(null)
  const [mkdirName, setMkdirName] = useState('')
  const [error, setError] = useState('')

  const listing = useQuery({
    queryKey: ['files', kind, id, active, path],
    queryFn: () => api.containerFiles(kind, id, path, active || undefined),
    enabled: kind === 'databases' || !!active,
  })

  const open = useMutation({
    mutationFn: (file: string) => api.readContainerFile(kind, id, file, active || undefined),
    onSuccess: (res) => {
      setOpenFile(res.path)
      setDraft(res.content)
      setError('')
    },
    onError: (e: Error) => setError(e.message),
  })

  const save = useMutation({
    mutationFn: () =>
      api.writeContainerFile(kind, id, { path: openFile || path, container: active || undefined, content: draft }),
    onSuccess: () => {
      setError('')
      void qc.invalidateQueries({ queryKey: ['files', kind, id] })
    },
    onError: (e: Error) => setError(e.message),
  })

  const mkdir = useMutation({
    mutationFn: () => {
      const next = path === '/' ? `/${mkdirName}` : `${path.replace(/\/$/, '')}/${mkdirName}`
      return api.mkdirContainer(kind, id, { path: next, container: active || undefined })
    },
    onSuccess: () => {
      setMkdirName('')
      void qc.invalidateQueries({ queryKey: ['files', kind, id] })
    },
    onError: (e: Error) => setError(e.message),
  })

  const remove = useMutation({
    mutationFn: (file: ContainerFile) =>
      api.deleteContainerFile(kind, id, { path: file.path, container: active || undefined, confirm: file.name }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['files', kind, id] }),
    onError: (e: Error) => setError(e.message),
  })

  const entries = listing.data?.entries || []
  const parent = path === '/' ? '/' : path.split('/').slice(0, -1).join('/') || '/'

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-end gap-3">
        {kind !== 'databases' && (
          <label className="text-sm">
            <span className="mb-1 block text-gray-500 dark:text-gray-400">Container</span>
            <select
              className="panel-field rounded-lg px-3 py-2"
              value={active}
              onChange={(e) => {
                setContainer(e.target.value)
                setOpenFile(null)
              }}
            >
              {names.length === 0 && <option value="">No containers</option>}
              {names.map((n) => (
                <option key={n} value={n}>
                  {n}
                </option>
              ))}
            </select>
          </label>
        )}
        <div className="min-w-[220px] flex-1 text-sm">
          <span className="mb-1 block text-gray-500 dark:text-gray-400">Path</span>
          <div className="flex gap-2">
            <input
              className="panel-field w-full rounded-lg px-3 py-2 font-mono text-xs"
              value={path}
              onChange={(e) => setPath(e.target.value || '/')}
            />
            <Btn onClick={() => void listing.refetch()}>Refresh</Btn>
          </div>
        </div>
      </div>
      {error && <p className="text-sm text-error-500">{error}</p>}
      {listing.isLoading && <p className="text-sm text-gray-500">Loading files…</p>}
      {listing.error && <p className="text-sm text-error-500">{(listing.error as Error).message}</p>}
      <div className="panel-card overflow-hidden">
        <table className="panel-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Size</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {path !== '/' && (
              <tr>
                <td colSpan={3}>
                  <button type="button" className="text-sm text-brand-600" onClick={() => setPath(parent)}>
                    ..
                  </button>
                </td>
              </tr>
            )}
            {entries.map((e) => (
              <tr key={e.path}>
                <td>
                  <button
                    type="button"
                    className="text-left text-sm font-medium text-gray-900 hover:underline dark:text-white"
                    onClick={() => (e.is_dir ? setPath(e.path) : open.mutate(e.path))}
                  >
                    {e.is_dir ? `${e.name}/` : e.name}
                  </button>
                </td>
                <td className="text-xs text-gray-500">{e.is_dir ? '—' : e.size}</td>
                <td>
                  {!e.is_dir && (
                    <button
                      type="button"
                      className="text-xs text-error-500"
                      onClick={() => {
                        if (confirm(`Delete ${e.name}?`)) remove.mutate(e)
                      }}
                    >
                      Delete
                    </button>
                  )}
                </td>
              </tr>
            ))}
            {!listing.isLoading && !entries.length && (
              <tr>
                <td colSpan={3} className="px-4 py-8 text-center text-sm text-gray-500">
                  This directory is empty, or the container is not running.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
      <form
        className="flex flex-wrap items-end gap-2"
        onSubmit={(e) => {
          e.preventDefault()
          if (mkdirName.trim()) mkdir.mutate()
        }}
      >
        <label className="text-sm">
          <span className="mb-1 block text-gray-500 dark:text-gray-400">New folder</span>
          <input
            className="panel-field rounded-lg px-3 py-2"
            value={mkdirName}
            onChange={(e) => setMkdirName(e.target.value)}
            placeholder="logs"
          />
        </label>
        <Btn type="submit" disabled={mkdir.isPending || !mkdirName.trim()}>
          Create
        </Btn>
      </form>
      {openFile && (
        <div className="panel-card space-y-3 p-4">
          <div className="flex items-center justify-between gap-3">
            <h3 className="font-mono text-xs text-gray-700 dark:text-gray-200">{openFile}</h3>
            <Btn primary disabled={save.isPending} onClick={() => save.mutate()}>
              {save.isPending ? 'Saving…' : 'Save file'}
            </Btn>
          </div>
          <textarea
            className="panel-field min-h-48 w-full rounded-lg px-3 py-2 font-mono text-xs"
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
          />
          <p className="text-xs text-gray-500">Text files up to 512 KiB. Saving replaces the file inside the container.</p>
        </div>
      )}
    </div>
  )
}
