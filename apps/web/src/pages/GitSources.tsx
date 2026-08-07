import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useParams, useSearch } from '@tanstack/react-router'
import { useEffect, useMemo, useState } from 'react'
import { PageSkeleton } from '../components/ui/Skeleton'
import { ResourceTabs, TabPanel } from '../components/ui/tabs'
import { api } from '../lib/api'
import { Btn, Input, Modal } from './Servers'

function randomSourceName() {
  const words = ['calm', 'bright', 'swift', 'quiet', 'bold', 'clear', 'noble', 'keen']
  const a = words[Math.floor(Math.random() * words.length)]
  const b = words[Math.floor(Math.random() * words.length)]
  return `${a}-${b}-${Math.floor(Math.random() * 90 + 10)}`
}

export function GitSourcesPage() {
  const qc = useQueryClient()
  const nav = useNavigate()
  const sources = useQuery({ queryKey: ['git-sources'], queryFn: api.gitSources })
  const [showForm, setShowForm] = useState(false)
  const [name, setName] = useState(randomSourceName)
  const [organization, setOrganization] = useState('')
  const [showSelfHosted, setShowSelfHosted] = useState(false)
  const [htmlURL, setHtmlURL] = useState('https://github.com')
  const [apiURL, setApiURL] = useState('https://api.github.com')
  const [customUser, setCustomUser] = useState('git')
  const [customPort, setCustomPort] = useState('22')

  const create = useMutation({
    mutationFn: () =>
      api.createGitSource({
        name,
        organization: organization || undefined,
        html_url: htmlURL,
        api_url: apiURL,
        custom_user: customUser,
        custom_port: Number(customPort) || 22,
      }),
    onSuccess: (gs) => {
      void qc.invalidateQueries({ queryKey: ['git-sources'] })
      setShowForm(false)
      setName(randomSourceName())
      setOrganization('')
      void nav({ to: '/git-sources/$sourceId', params: { sourceId: gs.id } })
    },
  })

  if (sources.isLoading) return <PageSkeleton cards={2} />

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-xl font-semibold tracking-tight text-gray-900 dark:text-white">Sources</h1>
        </div>
        <Btn primary onClick={() => setShowForm(true)}>
          + Add
        </Btn>
      </div>

      {showForm && (
        <Modal title="New GitHub App" onClose={() => setShowForm(false)}>
          <form
            className="space-y-4"
            onSubmit={(e) => {
              e.preventDefault()
              create.mutate()
            }}
          >
            <p className="text-sm text-gray-500 dark:text-gray-400">
              Required for full GitHub integration (commit / pull request deployments).
            </p>
            <div className="grid gap-3 sm:grid-cols-2">
              <Input label="Name" value={name} onChange={setName} />
              <Input
                label="Organization (on GitHub)"
                value={organization}
                onChange={setOrganization}
                required={false}
              />
            </div>
            <p className="text-xs text-gray-500 dark:text-gray-400">
              Leave organization empty to use your personal GitHub account.
            </p>

            <button
              type="button"
              className="text-sm font-medium text-brand-600 hover:underline dark:text-brand-400"
              onClick={() => setShowSelfHosted((v) => !v)}
            >
              {showSelfHosted ? 'Hide' : 'Self-hosted / Enterprise GitHub'}
            </button>
            {showSelfHosted && (
              <div className="grid gap-3 sm:grid-cols-2">
                <Input
                  label="HTML Url"
                  value={htmlURL}
                  onChange={(v) => {
                    setHtmlURL(v)
                    if (
                      apiURL === 'https://api.github.com' ||
                      apiURL.startsWith(htmlURL) ||
                      apiURL.includes('/api/v3')
                    ) {
                      // derive later on server if left as github.com default
                      if (v !== 'https://github.com') setApiURL(v.replace(/\/$/, '') + '/api/v3')
                      else setApiURL('https://api.github.com')
                    }
                  }}
                />
                <Input label="API Url" value={apiURL} onChange={setApiURL} />
                <Input label="Custom Git User" value={customUser} onChange={setCustomUser} />
                <Input label="Custom Git Port" value={customPort} onChange={setCustomPort} />
              </div>
            )}

            {create.error && <p className="text-sm text-error-500">{create.error.message}</p>}
            <Btn primary type="submit" disabled={create.isPending}>
              {create.isPending ? 'Creating…' : 'Continue'}
            </Btn>
          </form>
        </Modal>
      )}

      <div className="grid gap-3 sm:grid-cols-2">
        {(sources.data?.git_sources || []).map((gs) => (
          <Link
            key={gs.id}
            to="/git-sources/$sourceId"
            params={{ sourceId: gs.id }}
            className="panel-card block p-5 transition hover:border-brand-300 dark:hover:border-brand-500/40"
          >
            <div className="font-medium text-gray-900 dark:text-white">{gs.name}</div>
            {!gs.configured ? (
              <p className="mt-1 text-sm text-error-500">Configuration is not finished.</p>
            ) : gs.organization ? (
              <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
                Organization: {gs.organization}
              </p>
            ) : !gs.installed ? (
              <p className="mt-1 text-sm text-amber-600 dark:text-amber-400">Not installed yet</p>
            ) : (
              <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
                Installed
                {gs.applications_count ? ` · ${gs.applications_count} app(s)` : ''}
              </p>
            )}
          </Link>
        ))}
        {!sources.data?.git_sources?.length && (
          <div className="panel-card col-span-full p-8 text-center text-sm text-gray-500 dark:text-gray-400">
            No sources found.
          </div>
        )}
      </div>
    </div>
  )
}

export function GitSourceDetailPage() {
  const { sourceId } = useParams({ strict: false }) as { sourceId: string }
  const search = useSearch({ strict: false }) as { installed?: string; registered?: string }
  const qc = useQueryClient()
  const nav = useNavigate()
  const source = useQuery({
    queryKey: ['git-source', sourceId],
    queryFn: () => api.getGitSource(sourceId),
  })
  const [tab, setTab] = useState<'general' | 'resources' | 'repositories'>('general')
  const [manualOpen, setManualOpen] = useState(false)
  const [previewPerms, setPreviewPerms] = useState(true)
  const [endpoint, setEndpoint] = useState('')
  const [toast, setToast] = useState('')
  const [error, setError] = useState('')

  const [form, setForm] = useState({
    name: '',
    organization: '',
    html_url: '',
    api_url: '',
    custom_user: 'git',
    custom_port: 22,
    app_id: '',
    installation_id: '',
    client_id: '',
    client_secret: '',
    webhook_secret: '',
    private_key: '',
  })

  useEffect(() => {
    if (search.installed === '1' || search.registered === '1') {
      void qc.invalidateQueries({ queryKey: ['git-source', sourceId] })
      void qc.invalidateQueries({ queryKey: ['git-sources'] })
      if (search.registered === '1') setToast('GitHub App registered. Install it on your account/org next.')
      if (search.installed === '1') setToast('GitHub App installation linked successfully.')
    }
  }, [search.installed, search.registered, sourceId, qc])

  useEffect(() => {
    const gs = source.data
    if (!gs) return
    setForm({
      name: gs.name || '',
      organization: gs.organization || '',
      html_url: gs.html_url || 'https://github.com',
      api_url: gs.api_url || 'https://api.github.com',
      custom_user: gs.custom_user || 'git',
      custom_port: gs.custom_port || 22,
      app_id: gs.app_id && gs.app_id !== '0' ? gs.app_id : '',
      installation_id: gs.installation_id && gs.installation_id !== '0' ? gs.installation_id : '',
      client_id: gs.client_id || '',
      client_secret: '',
      webhook_secret: '',
      private_key: '',
    })
  }, [source.data])

  const apps = useQuery({
    queryKey: ['git-source-apps', sourceId],
    queryFn: () => api.gitSourceApplications(sourceId),
    enabled: Boolean(source.data?.configured && source.data?.installed),
  })
  const repos = useQuery({
    queryKey: ['git-source-repos', sourceId],
    queryFn: () => api.gitSourceRepositories(sourceId),
    enabled: Boolean(source.data?.installed) && tab === 'repositories',
  })

  const registerManifest = useMutation({
    mutationFn: () =>
      api.gitSourceManifest(sourceId, {
        endpoint: endpoint || undefined,
        preview: previewPerms,
      }),
    onSuccess: (data) => {
      const formEl = document.createElement('form')
      formEl.method = 'post'
      formEl.action = data.action_url
      const input = document.createElement('input')
      input.type = 'hidden'
      input.name = 'manifest'
      input.value = JSON.stringify(data.manifest)
      formEl.appendChild(input)
      document.body.appendChild(formEl)
      formEl.submit()
    },
    onError: (e: Error) => setError(e.message),
  })

  const install = useMutation({
    mutationFn: () => api.gitSourceInstallURL(sourceId),
    onSuccess: (data) => {
      window.location.href = data.install_url
    },
    onError: (e: Error) => setError(e.message),
  })

  const save = useMutation({
    mutationFn: () => {
      const body: Record<string, unknown> = {
        name: form.name,
        organization: form.organization,
        html_url: form.html_url,
        api_url: form.api_url,
        custom_user: form.custom_user,
        custom_port: form.custom_port,
        app_id: form.app_id,
        installation_id: form.installation_id,
        client_id: form.client_id,
      }
      if (form.client_secret.trim()) body.client_secret = form.client_secret
      if (form.webhook_secret.trim()) body.webhook_secret = form.webhook_secret
      if (form.private_key.trim()) body.private_key = form.private_key
      return api.updateGitSource(sourceId, body)
    },
    onSuccess: () => {
      setToast('GitHub App updated.')
      setError('')
      setManualOpen(false)
      void qc.invalidateQueries({ queryKey: ['git-source', sourceId] })
      void qc.invalidateQueries({ queryKey: ['git-sources'] })
    },
    onError: (e: Error) => setError(e.message),
  })

  const remove = useMutation({
    mutationFn: () => api.deleteGitSource(sourceId),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['git-sources'] })
      void nav({ to: '/git-sources' })
    },
    onError: (e: Error) => setError(e.message),
  })

  const startManual = () => {
    setManualOpen(true)
    setForm((f) => ({
      ...f,
      app_id: f.app_id || '',
      installation_id: f.installation_id || '',
    }))
  }

  if (source.isLoading) return <PageSkeleton cards={2} />
  if (source.error || !source.data) {
    return <p className="text-error-500">{source.error?.message || 'Not found'}</p>
  }
  const gs = source.data
  const needsRegister = !gs.configured
  const needsInstall = gs.configured && !gs.installed

  const tabs = useMemo(() => {
    const t: { id: string; label: string }[] = [{ id: 'general', label: 'General' }]
    if (gs.installed) {
      t.push({ id: 'repositories', label: 'Repositories' })
      t.push({ id: 'resources', label: 'Resources' })
    }
    return t
  }, [gs.installed])

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <Link
            to="/git-sources"
            className="text-sm text-gray-500 hover:text-brand-600 dark:text-gray-400 dark:hover:text-brand-400"
          >
            ← Sources
          </Link>
          <h1 className="mt-2 text-xl font-semibold tracking-tight text-gray-900 dark:text-white">
            GitHub App
          </h1>
        </div>
        <div className="flex gap-2">
          {gs.installed && (
            <Btn primary type="button" disabled={save.isPending} onClick={() => save.mutate()}>
              {save.isPending ? 'Saving…' : 'Save'}
            </Btn>
          )}
          <Btn
            type="button"
            disabled={remove.isPending}
            onClick={() => {
              if (window.confirm(`Delete GitHub App “${gs.name}”?`)) remove.mutate()
            }}
          >
            Delete
          </Btn>
        </div>
      </div>

      {toast && <p className="text-sm text-success-600 dark:text-success-400">{toast}</p>}
      {error && <p className="text-sm text-error-500">{error}</p>}

      {needsRegister && !manualOpen && (
        <div className="grid gap-4 lg:grid-cols-2">
          <div className="panel-card flex flex-col gap-4 p-6">
            <div className="flex items-center justify-between">
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white">
                Automated Installation
              </h3>
              <span className="rounded bg-brand-50 px-2 py-1 text-xs font-bold tracking-wide text-brand-700 uppercase dark:bg-brand-500/20 dark:text-brand-300">
                Recommended
              </span>
            </div>
            <p className="text-sm text-gray-500 dark:text-gray-400">
              Register a GitHub App via GitHub&apos;s manifest flow. Permissions and webhooks are
              pre-configured.
            </p>
            <Input
              label="Webhook / public endpoint"
              value={endpoint}
              onChange={setEndpoint}
              required={false}
            />
            <p className="-mt-2 text-xs text-gray-500 dark:text-gray-400">
              Leave empty to use Settings → Domain (or DOCKFIN_PUBLIC_URL).
            </p>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={previewPerms}
                onChange={(e) => setPreviewPerms(e.target.checked)}
                className="accent-[var(--color-accent)]"
              />
              Preview Deployments (pull request comments)
            </label>
            <Btn
              primary
              type="button"
              disabled={registerManifest.isPending}
              onClick={() => registerManifest.mutate()}
            >
              {registerManifest.isPending ? 'Preparing…' : 'Register Now'}
            </Btn>
          </div>

          <div className="panel-card flex flex-col gap-4 p-6">
            <div className="flex items-center justify-between">
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white">
                Manual Installation
              </h3>
              <span className="rounded bg-gray-100 px-2 py-1 text-xs font-bold tracking-wide text-gray-500 uppercase dark:bg-gray-800 dark:text-gray-400">
                Advanced
              </span>
            </div>
            <p className="text-sm text-gray-500 dark:text-gray-400">
              Fill the GitHub App form manually. For GitHub Enterprise or custom permission setups.
            </p>
            <div className="mt-auto">
              <Btn type="button" onClick={startManual}>
                Continue
              </Btn>
            </div>
          </div>
        </div>
      )}

      {(manualOpen || (needsRegister && manualOpen) || gs.configured) && (
        <>
          {needsInstall && (
            <div className="rounded-lg border border-error-500/40 bg-error-500/5 p-4 text-sm text-error-600 dark:text-error-400">
              You must complete this step before you can use this source!
            </div>
          )}

          {needsInstall && (
            <div className="panel-card p-5">
              <Btn primary type="button" disabled={install.isPending} onClick={() => install.mutate()}>
                {install.isPending ? 'Preparing…' : 'Install Repositories on GitHub'}
              </Btn>
            </div>
          )}

          {gs.installed && (
            <ResourceTabs
              tabs={tabs}
              active={tab}
              onChange={(id) => setTab(id as typeof tab)}
            />
          )}

          {(manualOpen || gs.configured) && (
            <TabPanel>
              {(!gs.installed || tab === 'general') && (
                <div className="panel-card space-y-4 p-5">
                  <div className="grid gap-3 sm:grid-cols-2">
                    <Input label="App Name" value={form.name} onChange={(v) => setForm({ ...form, name: v })} />
                    <Input
                      label="Organization"
                      value={form.organization}
                      onChange={(v) => setForm({ ...form, organization: v })}
                      required={false}
                    />
                    <Input
                      label="HTML Url"
                      value={form.html_url}
                      onChange={(v) => setForm({ ...form, html_url: v })}
                    />
                    <Input
                      label="API Url"
                      value={form.api_url}
                      onChange={(v) => setForm({ ...form, api_url: v })}
                    />
                    <Input
                      label="User"
                      value={form.custom_user}
                      onChange={(v) => setForm({ ...form, custom_user: v })}
                    />
                    <Input
                      label="Port"
                      value={String(form.custom_port)}
                      onChange={(v) => setForm({ ...form, custom_port: Number(v) || 22 })}
                    />
                    <Input
                      label="App Id"
                      value={form.app_id}
                      onChange={(v) => setForm({ ...form, app_id: v })}
                    />
                    <Input
                      label="Installation Id"
                      value={form.installation_id}
                      onChange={(v) => setForm({ ...form, installation_id: v })}
                      required={false}
                    />
                    <Input
                      label="Client Id"
                      value={form.client_id}
                      onChange={(v) => setForm({ ...form, client_id: v })}
                      required={false}
                    />
                  </div>
                  <div className="grid gap-3 sm:grid-cols-2">
                    <label className="block text-sm">
                      <span className="mb-1 block text-gray-500 dark:text-gray-400">Client Secret</span>
                      <input
                        type="password"
                        value={form.client_secret}
                        onChange={(e) => setForm({ ...form, client_secret: e.target.value })}
                        className="panel-field w-full rounded-lg px-3 py-2"
                        placeholder="Leave blank to keep"
                      />
                    </label>
                    <label className="block text-sm">
                      <span className="mb-1 block text-gray-500 dark:text-gray-400">Webhook Secret</span>
                      <input
                        type="password"
                        value={form.webhook_secret}
                        onChange={(e) => setForm({ ...form, webhook_secret: e.target.value })}
                        className="panel-field w-full rounded-lg px-3 py-2"
                        placeholder="Leave blank to keep"
                      />
                    </label>
                  </div>
                  <label className="block text-sm">
                    <span className="mb-1 block text-gray-500 dark:text-gray-400">
                      Private Key (PEM)
                    </span>
                    <textarea
                      value={form.private_key}
                      onChange={(e) => setForm({ ...form, private_key: e.target.value })}
                      rows={5}
                      className="panel-field w-full rounded-lg px-3 py-2 font-mono text-xs"
                      placeholder={
                        gs.has_private_key
                          ? 'Leave blank to keep existing key'
                          : '-----BEGIN RSA PRIVATE KEY-----'
                      }
                    />
                  </label>
                  {(manualOpen || needsRegister) && (
                    <Btn primary type="button" disabled={save.isPending} onClick={() => save.mutate()}>
                      {save.isPending ? 'Saving…' : 'Save configuration'}
                    </Btn>
                  )}
                  {gs.configured && !gs.installed && (
                    <Btn type="button" disabled={install.isPending} onClick={() => install.mutate()}>
                      Install Repositories on GitHub
                    </Btn>
                  )}
                </div>
              )}

              {gs.installed && tab === 'repositories' && (
                <div className="panel-card overflow-hidden">
                  <div className="flex items-center justify-between border-b border-gray-200 px-3 py-2 dark:border-gray-800">
                    <span className="text-sm font-medium">Repositories</span>
                    <button
                      type="button"
                      className="text-xs text-brand-600 hover:underline dark:text-brand-400"
                      onClick={() => install.mutate()}
                    >
                      Update Repositories
                    </button>
                  </div>
                  <ul className="divide-y divide-gray-200 dark:divide-gray-800">
                    {(repos.data?.repositories || []).map((r, i) => {
                      const full = String(r.full_name || r.name || i)
                      const url = String(r.clone_url || r.html_url || '')
                      return (
                        <li key={full} className="px-3 py-2 text-sm">
                          <div className="font-medium text-gray-900 dark:text-white">{full}</div>
                          {url && (
                            <div className="font-mono text-xs text-gray-500 dark:text-gray-400">{url}</div>
                          )}
                        </li>
                      )
                    })}
                    {!repos.data?.repositories?.length && !repos.isLoading && (
                      <li className="px-4 py-8 text-center text-sm text-gray-500 dark:text-gray-400">
                        No repositories visible. Update installation permissions on GitHub.
                      </li>
                    )}
                  </ul>
                </div>
              )}

              {gs.installed && tab === 'resources' && (
                <div className="panel-card overflow-x-auto p-4">
                  {(apps.data?.applications || []).length === 0 ? (
                    <p className="py-4 text-sm text-gray-500 dark:text-gray-400">
                      No resources are currently using this GitHub App.
                    </p>
                  ) : (
                    <table className="min-w-full text-sm">
                      <thead>
                        <tr className="text-left text-xs uppercase text-gray-500">
                          <th className="px-3 py-2">Project</th>
                          <th className="px-3 py-2">Environment</th>
                          <th className="px-3 py-2">Name</th>
                          <th className="px-3 py-2">Type</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-gray-200 dark:divide-gray-800">
                        {(apps.data?.applications || []).map((app) => (
                          <tr key={app.id}>
                            <td className="px-3 py-3">{app.project_name}</td>
                            <td className="px-3 py-3">{app.environment_name}</td>
                            <td className="px-3 py-3">
                              <Link
                                to="/projects/$projectId/environments/$envId/applications/$appId"
                                params={{
                                  projectId: app.project_id,
                                  envId: app.environment_id,
                                  appId: app.id,
                                }}
                                className="text-brand-600 hover:underline dark:text-brand-400"
                              >
                                {app.name}
                              </Link>
                            </td>
                            <td className="px-3 py-3 capitalize">{app.build_pack}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  )}
                </div>
              )}
            </TabPanel>
          )}
        </>
      )}
    </div>
  )
}
