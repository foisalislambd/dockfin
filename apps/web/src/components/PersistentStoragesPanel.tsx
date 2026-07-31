type Props = {
  compose?: string
  volumes?: Array<{
    service: string
    name: string
    mount_path: string
    host_path?: string
    type: string
  }>
}

/** Client-side volume parse when API volumes are missing (older responses). */
function parseVolumesClient(compose: string): Props['volumes'] {
  const out: NonNullable<Props['volumes']> = []
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
        const isBind = src.startsWith('/') || src.startsWith('./') || src.startsWith('../') || src === '.'
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

export function PersistentStoragesPanel({ compose = '', volumes }: Props) {
  const list =
    volumes && volumes.length > 0 ? volumes : compose ? parseVolumesClient(compose) || [] : []

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-sm font-semibold text-gray-900 dark:text-white">Persistent Storages</h2>
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
          Volumes declared in the compose file. Named volumes are created automatically on deploy.
        </p>
      </div>
      <div className="panel-card overflow-hidden">
        <table className="w-full text-left text-sm">
          <thead className="bg-gray-50 text-gray-500 dark:bg-white/5 dark:text-gray-400">
            <tr>
              <th className="px-3 py-2">Service</th>
              <th className="px-3 py-2">Name / source</th>
              <th className="px-3 py-2">Mount path</th>
              <th className="px-3 py-2">Type</th>
            </tr>
          </thead>
          <tbody>
            {list.map((v, i) => (
              <tr key={`${v.service}-${v.mount_path}-${i}`} className="border-t border-gray-200 dark:border-gray-800">
                <td className="px-3 py-2 font-mono text-xs">{v.service}</td>
                <td className="px-3 py-2 font-mono text-xs">{v.name || '—'}</td>
                <td className="px-3 py-2 font-mono text-xs">{v.mount_path}</td>
                <td className="px-3 py-2 capitalize text-gray-500 dark:text-gray-400">{v.type}</td>
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
