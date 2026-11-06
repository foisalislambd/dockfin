import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Fragment, useState } from 'react'
import { api } from '../lib/api'
import { Header } from '../components/ui/Header'
import { Btn } from '../components/ui/Button'
import { PageSkeleton } from '../components/ui/Skeleton'

export function VulnerabilitiesPage() {
  const qc = useQueryClient()
  const policy = useQuery({ queryKey: ['image-scan-policy'], queryFn: api.imageScanPolicy })
  const scans = useQuery({ queryKey: ['image-scans'], queryFn: api.imageScans })
  const apps = useQuery({ queryKey: ['applications'], queryFn: () => api.applications() })
  const [enabled, setEnabled] = useState<boolean | null>(null)
  const [gate, setGate] = useState('')
  const [error, setError] = useState('')
  const [open, setOpen] = useState<string | null>(null)
  const [confirm, setConfirm] = useState('')
  const [tag, setTag] = useState('')

  const save = useMutation({
    mutationFn: () => api.setImageScanPolicy(enabled ?? policy.data?.enabled ?? true, gate || policy.data?.gate || 'off'),
    onSuccess: () => {
      setError('')
      void qc.invalidateQueries({ queryKey: ['image-scan-policy'] })
    },
    onError: (e: Error) => setError(e.message),
  })
  const run = useMutation({
    mutationFn: (appId: string) => api.scanApplication(appId),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['image-scans'] }),
    onError: (e: Error) => setError(e.message),
  })
  const ignore = useMutation({
    mutationFn: (id: string) => api.ignoreVuln(id, 'dismissed from the vulnerabilities page'),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['image-scans'] }),
  })
  const candidates = useMutation({
    mutationFn: (scanId: string) => api.imageCandidates(scanId),
    onSuccess: (res) => {
      setError('')
      setTag(res.patch || res.line || '')
      void qc.invalidateQueries({ queryKey: ['image-scans'] })
    },
    onError: (e: Error) => setError(e.message),
  })
  const upgrade = useMutation({
    mutationFn: (input: { appId: string; confirm: string; tag?: string }) => api.imageUpgrade(input.appId, input.confirm, input.tag),
    onSuccess: () => {
      setError('')
      setConfirm('')
    },
    onError: (e: Error) => setError(e.message),
  })

  const on = enabled ?? policy.data?.enabled ?? true
  const mode = gate || policy.data?.gate || 'off'

  return (
    <div className="space-y-4">
      <Header
        title="Vulnerabilities"
        subtitle="Syft builds a bill of materials and Grype matches it on the server that has the image. A finished deploy scans automatically."
      />
      {policy.isLoading || scans.isLoading ? <PageSkeleton cards={2} /> : null}
      {error ? <p className="text-sm text-error-500">{error}</p> : null}
      <div className="panel-card space-y-3 p-4">
        <h2 className="text-sm font-semibold text-gray-900 dark:text-white">Instance policy</h2>
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={on} onChange={(e) => setEnabled(e.target.checked)} />
          Scan images after deploy and on demand
        </label>
        <label className="block text-sm">
          <span className="mb-1 block text-gray-500">Deploy gate</span>
          <select className="panel-field rounded-lg px-3 py-2" value={mode} onChange={(e) => setGate(e.target.value)}>
            <option value="off">Off — record findings, do not stop the deploy</option>
            <option value="warn">Warn — write the counts into the deploy log</option>
            <option value="block">Block — fail the deploy on a critical finding or a scanner error</option>
          </select>
        </label>
        <Btn primary disabled={save.isPending} onClick={() => save.mutate()}>
          {save.isPending ? 'Saving…' : 'Save policy'}
        </Btn>
      </div>
      <div className="panel-card space-y-3 p-4">
        <h2 className="text-sm font-semibold text-gray-900 dark:text-white">Scan now</h2>
        <div className="flex flex-wrap gap-2">
          {(apps.data?.applications || []).map((app) => (
            <Btn key={app.id} disabled={run.isPending} onClick={() => run.mutate(app.id)}>
              {app.name}
            </Btn>
          ))}
          {!apps.isLoading && !(apps.data?.applications || []).length ? (
            <p className="text-sm text-gray-500">No applications on this team.</p>
          ) : null}
        </div>
      </div>
      <div className="panel-card overflow-hidden">
        <table className="w-full text-left text-sm">
          <thead className="border-b border-gray-200 text-xs text-gray-500 dark:border-gray-800">
            <tr>
              <th className="px-4 py-3 font-medium">Image</th>
              <th className="px-4 py-3 font-medium">Status</th>
              <th className="px-4 py-3 font-medium">Critical</th>
              <th className="px-4 py-3 font-medium">High</th>
              <th className="px-4 py-3 font-medium">Medium</th>
              <th className="px-4 py-3 font-medium">Low</th>
              <th className="px-4 py-3 font-medium">When</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-200 dark:divide-gray-800">
            {(scans.data?.scans || []).map((s) => (
              <Fragment key={s.id}>
                <tr className="cursor-pointer" onClick={() => setOpen(open === s.id ? null : s.id)}>
                  <td className="px-4 py-3">
                    <div className="font-medium text-gray-900 dark:text-white">{s.image_ref}</div>
                    <div className="text-xs text-gray-500">{s.resource_type}</div>
                  </td>
                  <td className="px-4 py-3">{s.status}</td>
                  <td className="px-4 py-3">{s.critical}</td>
                  <td className="px-4 py-3">{s.high}</td>
                  <td className="px-4 py-3">{s.medium}</td>
                  <td className="px-4 py-3">{s.low}</td>
                  <td className="px-4 py-3 text-xs text-gray-500">{new Date(s.created_at).toLocaleString()}</td>
                </tr>
                {open === s.id ? (
                  <tr>
                    <td colSpan={7} className="bg-gray-50 px-4 py-3 dark:bg-white/5">
                      {s.error ? <p className="text-sm text-error-500">{s.error}</p> : null}
                      {(s.findings || []).length === 0 && !s.error ? (
                        <p className="text-sm text-gray-500">No findings stored.</p>
                      ) : null}
                      <div className="mb-3 space-y-2">
                        <p className="text-sm text-gray-600 dark:text-gray-300">
                          Check the registry for a newer tag on the same line. The tag is not scanned until you deploy it, so this does not claim the CVE is fixed.
                        </p>
                        <Btn disabled={candidates.isPending} onClick={() => candidates.mutate(s.id)}>
                          {candidates.isPending ? 'Checking…' : 'Check newer tags'}
                        </Btn>
                        {s.candidates?.note ? <p className="text-sm text-gray-500">{s.candidates.note}</p> : null}
                        {s.candidates?.current_tag ? (
                          <p className="text-sm">
                            Current tag {s.candidates.current_tag}
                            {s.candidates.patch ? ` · patch ${s.candidates.patch}` : ''}
                            {s.candidates.line ? ` · line ${s.candidates.line}` : ''}
                            {s.candidates.major ? ` · major ${s.candidates.major} (not applied)` : ''}
                            {s.candidates.floating ? ' · this tag floats' : ''}
                          </p>
                        ) : null}
                        {s.candidates?.digests
                          ? Object.entries(s.candidates.digests).map(([name, digest]) => (
                              <p key={name} className="font-mono text-xs text-gray-500">
                                {name} {digest}
                              </p>
                            ))
                          : null}
                        {s.resource_type === 'application' ? (
                          <div className="flex flex-wrap items-end gap-2">
                            <label className="block text-sm">
                              <span className="mb-1 block text-gray-500">Tag</span>
                              <input className="panel-field w-40 rounded-lg px-3 py-2" value={tag} onChange={(e) => setTag(e.target.value)} />
                            </label>
                            <label className="block text-sm">
                              <span className="mb-1 block text-gray-500">Type UPGRADE or REBUILD</span>
                              <input className="panel-field w-40 rounded-lg px-3 py-2" value={confirm} onChange={(e) => setConfirm(e.target.value)} />
                            </label>
                            <Btn
                              primary
                              disabled={upgrade.isPending || confirm !== 'UPGRADE'}
                              onClick={() => upgrade.mutate({ appId: s.resource_id, confirm: 'UPGRADE', tag })}
                            >
                              Upgrade tag
                            </Btn>
                            <Btn disabled={upgrade.isPending || confirm !== 'REBUILD'} onClick={() => upgrade.mutate({ appId: s.resource_id, confirm: 'REBUILD' })}>
                              Rebuild current image
                            </Btn>
                          </div>
                        ) : (
                          <p className="text-sm text-gray-500">Change a service image in its compose file, then scan again.</p>
                        )}
                      </div>
                      <ul className="space-y-1">
                        {(s.findings || []).map((f) => (
                          <li key={f.id + f.package} className="flex flex-wrap items-center gap-2 text-sm">
                            <span className="font-mono text-xs">{f.id}</span>
                            <span>{f.severity}</span>
                            <span>
                              {f.package} {f.version}
                            </span>
                            {f.fixed_version ? <span className="text-gray-500">fix {f.fixed_version}</span> : null}
                            <button type="button" className="text-xs text-brand-600 hover:underline" onClick={() => ignore.mutate(f.id)}>
                              Ignore
                            </button>
                          </li>
                        ))}
                      </ul>
                    </td>
                  </tr>
                ) : null}
              </Fragment>
            ))}
            {!scans.isLoading && !(scans.data?.scans || []).length ? (
              <tr>
                <td colSpan={7} className="px-4 py-8 text-center text-gray-500">
                  No scans yet. Deploy an application or scan one above. The server needs Docker so it can run Syft and Grype.
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>
    </div>
  )
}
