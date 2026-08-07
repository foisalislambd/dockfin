import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { api, type AppVolume } from '../lib/api'
import { useToast } from './Toast'
import { TableSkeleton } from './ui/Skeleton'
import { Btn, Input } from '../pages/Servers'

type ComposeVolume = {
  service: string
  name: string
  mount_path: string
  host_path?: string
  type: string
}

type Props = {
  compose?: string
  volumes?: ComposeVolume[]
  /** When set, panel is editable (non-compose applications). */
  applicationId?: string
  editable?: boolean
}

/** Client-side volume parse when API volumes are missing (older responses). */
function parseVolumesClient(compose: string): ComposeVolume[] {
  const out: ComposeVolume[] = []
  const lines = compose.split('\n')
  let inServices = false
  let currentSvc = ''
  let inVols = false
  for (const raw of lines) {
    const line = raw.replace(/\t/g, '  ')
    if (/^services:\s*$/.test(line)) {
      inServices = true
      continue
    }
    if (inServices && /^[a-zA-Z0-9]/.test(line) && !line.startsWith(' ')) {
      break
    }
    const svcMatch = line.match(/^  ([a-zA-Z0-9_-]+):\s*$/)
    if (inServices && svcMatch) {
      currentSvc = svcMatch[1]
      inVols = false
      continue
    }
    if (currentSvc && /^\s{4}volumes:\s*$/.test(line)) {
      inVols = true
      continue
    }
    if (inVols && /^\s{4}[a-zA-Z]/.test(line)) {
      inVols = false
      continue
    }
    if (inVols && /^\s{6}-\s+/.test(line)) {
      const item = line.replace(/^\s*-\s+/, '').replace(/^["']|["']$/g, '')
      const parts = item.split(':')
      if (parts.length >= 2) {
        const src = parts[0]
        const dest = parts[1]
        const isBind =
          src.startsWith('/') || src.startsWith('./') || src.startsWith('../') || src === '.'
        out.push({
          service: currentSvc,
          name: src,
          mount_path: dest,
          host_path: isBind ? src : undefined,
          type: isBind ? 'bind' : 'named',
        })
      } else {
        out.push({
          service: currentSvc,
          name: '',
          mount_path: parts[0] || '',
          type: 'anonymous',
        })
      }
    }
  }
  return out
}

export function PersistentStoragesPanel({
  compose = '',
  volumes,
  applicationId,
  editable = false,
}: Props) {
  if (editable && applicationId) {
    return <EditableVolumesPanel applicationId={applicationId} />
  }

  const list =
    volumes && volumes.length > 0 ? volumes : compose ? parseVolumesClient(compose) || [] : []

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-sm font-semibold text-gray-900 dark:text-white">Persistent Storages</h2>
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
          Volumes declared in the compose file. Named volumes are created automatically on deploy.
          Edit the compose YAML and use Load Compose to refresh this list.
        </p>
      </div>
      <div className="panel-card overflow-hidden">
        <table className="w-full text-left text-sm">
          <thead className="bg-gray-50 text-gray-500 dark:bg-white/5 dark:text-gray-400">
            <tr>
              <th className="px-3 py-2">Service</th>
              <th className="px-3 py-2">Name / Source</th>
              <th className="px-3 py-2">Mount path</th>
              <th className="px-3 py-2">Type</th>
            </tr>
          </thead>
          <tbody>
            {list.map((v, i) => (
              <tr key={`${v.service}-${v.name}-${i}`} className="border-t border-gray-200 dark:border-gray-800">
                <td className="px-3 py-2 font-mono text-xs">{v.service}</td>
                <td className="px-3 py-2 font-mono text-xs">{v.name || '—'}</td>
                <td className="px-3 py-2 font-mono text-xs">{v.mount_path}</td>
                <td className="px-3 py-2 capitalize">{v.type}</td>
              </tr>
            ))}
            {!list.length && (
              <tr>
                <td colSpan={4} className="px-4 py-8 text-center text-gray-500 dark:text-gray-400">
                  No volumes found in this compose file.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function EditableVolumesPanel({ applicationId }: { applicationId: string }) {
  const qc = useQueryClient()
  const toast = useToast()
  const [name, setName] = useState('')
  const [mountPath, setMountPath] = useState('')
  const [hostPath, setHostPath] = useState('')

  const vols = useQuery({
    queryKey: ['app-volumes', applicationId],
    queryFn: () => api.listAppVolumes(applicationId),
  })

  const upsert = useMutation({
    mutationFn: () =>
      api.upsertAppVolume(applicationId, {
        name: name.trim(),
        mount_path: mountPath.trim(),
        host_path: hostPath.trim() || undefined,
      }),
    onSuccess: () => {
      setName('')
      setMountPath('')
      setHostPath('')
      void qc.invalidateQueries({ queryKey: ['app-volumes', applicationId] })
      toast.success('Volume saved')
    },
    onError: (e: Error) => toast.error(e.message || 'Failed to save volume'),
  })

  const remove = useMutation({
    mutationFn: (volumeId: string) => api.deleteAppVolume(applicationId, volumeId),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['app-volumes', applicationId] })
      toast.success('Volume removed')
    },
    onError: (e: Error) => toast.error(e.message || 'Failed to delete volume'),
  })

  const list: AppVolume[] = vols.data?.volumes || []

  return (
    <div className="space-y-4">
      <p className="text-sm text-gray-500 dark:text-gray-400">
        Bind mounts applied on the next deploy as <code className="font-mono text-xs">-v host:mount</code>.
        Leave host path empty to use{' '}
        <code className="font-mono text-xs">/data/dockfin/applications/…/volumes/&lt;name&gt;</code>.
      </p>

      <form
        className="panel-card grid gap-3 p-4 sm:grid-cols-3"
        onSubmit={(e) => {
          e.preventDefault()
          if (!name.trim() || !mountPath.trim()) return
          upsert.mutate()
        }}
      >
        <Input label="Name" value={name} onChange={setName} required />
        <Input label="Mount path" value={mountPath} onChange={setMountPath} required />
        <Input label="Host path (optional)" value={hostPath} onChange={setHostPath} required={false} />
        <div className="sm:col-span-3">
          <Btn primary type="submit" disabled={upsert.isPending || !name.trim() || !mountPath.trim()}>
            {upsert.isPending ? 'Adding…' : 'Add volume'}
          </Btn>
        </div>
      </form>

      <div className="panel-card overflow-hidden">
        {vols.isLoading ? (
          <TableSkeleton rows={3} cols={3} />
        ) : (
        <table className="w-full text-left text-sm">
          <thead className="bg-gray-50 text-gray-500 dark:bg-white/5 dark:text-gray-400">
            <tr>
              <th className="px-3 py-2">Name</th>
              <th className="px-3 py-2">Mount path</th>
              <th className="px-3 py-2">Host path</th>
              <th className="px-3 py-2" />
            </tr>
          </thead>
          <tbody>
            {list.map((v) => (
              <tr key={v.id} className="border-t border-gray-200 dark:border-gray-800">
                <td className="px-3 py-2 font-mono text-xs">{v.name}</td>
                <td className="px-3 py-2 font-mono text-xs">{v.mount_path}</td>
                <td className="px-3 py-2 font-mono text-xs text-gray-500">
                  {v.host_path || '(default)'}
                </td>
                <td className="px-3 py-2 text-right">
                  <button
                    type="button"
                    className="text-xs text-error-500 hover:underline"
                    disabled={remove.isPending}
                    onClick={() => remove.mutate(v.id)}
                  >
                    Remove
                  </button>
                </td>
              </tr>
            ))}
            {!list.length && (
              <tr>
                <td colSpan={4} className="px-4 py-8 text-center text-gray-500 dark:text-gray-400">
                  No persistent volumes yet.
                </td>
              </tr>
            )}
          </tbody>
        </table>
        )}
      </div>
    </div>
  )
}
