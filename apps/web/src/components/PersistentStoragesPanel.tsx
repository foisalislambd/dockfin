import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { api, type AppVolume } from '../lib/api'
import { useToast } from './Toast'
import { TableSkeleton } from './ui/Skeleton'
import { Btn } from './ui/Button'
import { Input } from './ui/Input'

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
  /** When set, panel edits database file and directory mounts. */
  databaseId?: string
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
  databaseId,
  editable = false,
}: Props) {
  if (editable && applicationId) {
    return <EditableVolumesPanel resource="application" resourceId={applicationId} />
  }
  if (editable && databaseId) {
    return <EditableVolumesPanel resource="database" resourceId={databaseId} />
  }

  const list =
    volumes && volumes.length > 0 ? volumes : compose ? parseVolumesClient(compose) || [] : []

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-lg font-semibold tracking-tight text-gray-900 dark:text-white">Persistent storage</h2>
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

function EditableVolumesPanel({
  resource,
  resourceId,
}: {
  resource: 'application' | 'database'
  resourceId: string
}) {
  const qc = useQueryClient()
  const toast = useToast()
  const [name, setName] = useState('')
  const [mountPath, setMountPath] = useState('')
  const [hostPath, setHostPath] = useState('')
  const [isFile, setIsFile] = useState(false)
  const [content, setContent] = useState('')
  const queryKey = ['resource-volumes', resource, resourceId]

  const vols = useQuery({
    queryKey,
    queryFn: () => api.listResourceVolumes(resource, resourceId),
  })

  const clear = () => {
    setName('')
    setMountPath('')
    setHostPath('')
    setIsFile(false)
    setContent('')
  }

  const upsert = useMutation({
    mutationFn: () =>
      api.upsertResourceVolume(resource, resourceId, {
        name: name.trim(),
        mount_path: mountPath.trim(),
        host_path: hostPath.trim() || undefined,
        is_file: isFile,
        content: isFile ? content : undefined,
      }),
    onSuccess: () => {
      clear()
      void qc.invalidateQueries({ queryKey })
      toast.success(isFile ? 'File mount saved' : 'Volume saved')
    },
    onError: (e: Error) => toast.error(e.message || 'Failed to save mount'),
  })

  const remove = useMutation({
    mutationFn: (volumeId: string) => api.deleteResourceVolume(resource, resourceId, volumeId),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey })
      toast.success('Mount removed')
    },
    onError: (e: Error) => toast.error(e.message || 'Failed to delete mount'),
  })

  const load = async (volume: AppVolume) => {
    setName(volume.name)
    setMountPath(volume.mount_path)
    setHostPath(volume.host_path || '')
    setIsFile(Boolean(volume.is_file))
    setContent('')
    if (volume.is_file) {
      try {
        const full = await api.getResourceVolume(resource, resourceId, volume.id)
        setContent(full.content || '')
      } catch (e) {
        toast.error(e instanceof Error ? e.message : 'Failed to load file')
      }
    }
  }

  const list: AppVolume[] = vols.data?.volumes || []
  const when = resource === 'database' ? 'start' : 'deploy'

  return (
    <div className="space-y-4">
      <h2 className="text-lg font-semibold tracking-tight text-gray-900 dark:text-white">Persistent storage</h2>
      <p className="text-sm text-gray-500 dark:text-gray-400">
        Bind mounts are applied on the next {when}. A file is written before the container starts,
        so Docker does not turn a missing file into a directory. Leave the host path empty to store
        it in this resource’s own directory.
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
        <label className="flex items-center gap-2 text-sm sm:col-span-3">
          <input type="checkbox" checked={isFile} onChange={(e) => setIsFile(e.target.checked)} />
          <span>File mount. Directory mounts are created empty and keep their contents.</span>
        </label>
        {isFile && (
          <label className="block text-sm sm:col-span-3">
            <span className="mb-1 block text-gray-500 dark:text-gray-400">File content</span>
            <textarea
              value={content}
              onChange={(e) => setContent(e.target.value)}
              className="panel-field h-36 w-full rounded-lg px-3 py-2 font-mono text-xs"
              placeholder="Leave empty to keep an existing host file, or to create an empty one."
            />
          </label>
        )}
        <div className="sm:col-span-3">
          <Btn primary type="submit" disabled={upsert.isPending || !name.trim() || !mountPath.trim()}>
            {upsert.isPending ? 'Saving…' : 'Save mount'}
          </Btn>
        </div>
      </form>

      <div className="panel-card overflow-hidden">
        {vols.isLoading ? (
          <TableSkeleton rows={3} cols={4} />
        ) : (
        <table className="w-full text-left text-sm">
          <thead className="bg-gray-50 text-gray-500 dark:bg-white/5 dark:text-gray-400">
            <tr>
              <th className="px-3 py-2">Name</th>
              <th className="px-3 py-2">Mount path</th>
              <th className="px-3 py-2">Host path</th>
              <th className="px-3 py-2">Type</th>
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
                <td className="px-3 py-2 text-xs">{v.is_file ? 'File' : 'Directory'}</td>
                <td className="px-3 py-2 text-right">
                  <button
                    type="button"
                    className="mr-3 text-xs text-gray-500 hover:underline"
                    onClick={() => void load(v)}
                  >
                    Edit
                  </button>
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
                <td colSpan={5} className="px-4 py-8 text-center text-gray-500 dark:text-gray-400">
                  No mounts yet.
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
