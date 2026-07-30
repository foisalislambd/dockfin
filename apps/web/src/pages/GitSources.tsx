import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useParams, useSearch } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { api, type GitSource } from '../lib/api'
import { Btn, Input } from './Servers'

export function GitSourcesPage() {
  const qc = useQueryClient()
  const nav = useNavigate()
  const sources = useQuery({ queryKey: ['git-sources'], queryFn: api.gitSources })
  const [name, setName] = useState('')
  const [slug, setSlug] = useState('')
  const [appID, setAppID] = useState('')
  const [clientID, setClientID] = useState('')
  const [privateKey, setPrivateKey] = useState('')
  const [showForm, setShowForm] = useState(false)

  const create = useMutation({
    mutationFn: () =>
      api.createGitSource({
        name,
        slug: slug || name,
        app_id: appID,
        client_id: clientID,
        private_key: privateKey,
      }),
    onSuccess: (gs) => {
      void qc.invalidateQueries({ queryKey: ['git-sources'] })
      setShowForm(false)
      setName('')
      setSlug('')
      setAppID('')
      setClientID('')
      setPrivateKey('')
      void nav({ to: '/git-sources/$sourceId', params: { sourceId: gs.id } })
    },
  })

  const remove = useMutation({
    mutationFn: (id: string) => api.deleteGitSource(id),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['git-sources'] }),
  })

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-xl font-semibold tracking-tight text-gray-900 dark:text-white">
            Git Sources
          </h1>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            Connect a GitHub App to clone private repositories during deploys.
          </p>
        </div>
        <Btn primary onClick={() => setShowForm((v) => !v)}>
          {showForm ? 'Cancel' : 'Add GitHub App'}
        </Btn>
      </div>

      {showForm && (
        <form
          className="panel-card space-y-4 p-5"
          onSubmit={(e) => {
            e.preventDefault()
            create.mutate()
          }}
        >
          <div className="grid gap-4 sm:grid-cols-2">
            <Input label="Display name" value={name} onChange={setName} />
            <Input
              label="GitHub App slug"
              value={slug}
              onChange={setSlug}
              required={false}
            />
            <Input label="App ID" value={appID} onChange={setAppID} />
            <Input label="Client ID" value={clientID} onChange={setClientID} required={false} />
          </div>
          <label className="block text-sm">
            <span className="mb-1 block text-gray-500 dark:text-gray-400">Private key (PEM)</span>
            <textarea
              value={privateKey}
              onChange={(e) => setPrivateKey(e.target.value)}
              rows={6}
              required
              className="w-full rounded-lg border border-gray-200 bg-white px-3 py-2 font-mono text-xs dark:border-gray-800 dark:bg-gray-900"
            />
          </label>
          {create.error && <p className="text-sm text-error-500">{create.error.message}</p>}
          <Btn primary type="submit">
            {create.isPending ? 'Saving…' : 'Create'}
          </Btn>
        </form>
      )}

      <div className="grid gap-3 sm:grid-cols-2">
        {(sources.data?.git_sources || []).map((gs) => (
          <GitSourceCard key={gs.id} source={gs} onDelete={() => remove.mutate(gs.id)} />
        ))}
        {!sources.data?.git_sources?.length && !sources.isLoading && (
          <div className="panel-card col-span-full p-8 text-center text-sm text-gray-500">
            No GitHub Apps configured yet.
          </div>
        )}
      </div>
    </div>
  )
}

function GitSourceCard({ source, onDelete }: { source: GitSource; onDelete: () => void }) {
  return (
    <div className="panel-card flex flex-col gap-3 p-5">
      <div className="flex items-start justify-between gap-3">
        <div>
          <Link
            to="/git-sources/$sourceId"
            params={{ sourceId: source.id }}
            className="font-medium text-gray-900 hover:text-brand-600 dark:text-white dark:hover:text-brand-400"
          >
            {source.name}
          </Link>
          <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
            App ID {source.app_id}
            {source.installation_id ? ' · installed' : ' · not installed'}
          </p>
        </div>
        <button type="button" className="text-xs text-error-500" onClick={onDelete}>
          Delete
        </button>
      </div>
    </div>
  )
}

export function GitSourceDetailPage() {
  const { sourceId } = useParams({ strict: false }) as { sourceId: string }
  const search = useSearch({ strict: false }) as { installed?: string }
  const qc = useQueryClient()
  const source = useQuery({
    queryKey: ['git-source', sourceId],
    queryFn: () => api.getGitSource(sourceId),
  })
  const repos = useQuery({
    queryKey: ['git-source-repos', sourceId],
    queryFn: () => api.gitSourceRepositories(sourceId),
    enabled: Boolean(source.data?.installation_id),
  })

  useEffect(() => {
    if (search.installed === '1') {
      void qc.invalidateQueries({ queryKey: ['git-source', sourceId] })
      void qc.invalidateQueries({ queryKey: ['git-sources'] })
    }
  }, [search.installed, sourceId, qc])

  const install = useMutation({
    mutationFn: () => api.gitSourceInstallURL(sourceId),
    onSuccess: (data) => {
      window.location.href = data.install_url
    },
  })

  if (source.isLoading) return <p className="text-gray-500">Loading…</p>
  if (source.error || !source.data) {
    return <p className="text-error-500">{source.error?.message || 'Not found'}</p>
  }
  const gs = source.data

  return (
    <div className="space-y-6">
      <div>
        <Link
          to="/git-sources"
          className="text-sm text-gray-500 hover:text-brand-600 dark:text-gray-400 dark:hover:text-brand-400"
        >
          ← Git Sources
        </Link>
        <h1 className="mt-2 text-xl font-semibold tracking-tight text-gray-900 dark:text-white">
          {gs.name}
        </h1>
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {gs.installation_id
            ? `Installed (installation ${gs.installation_id})`
            : 'Not installed on any GitHub org/account yet'}
        </p>
      </div>

      {search.installed === '1' && (
        <p className="text-sm text-emerald-600 dark:text-emerald-400">
          GitHub App installation linked successfully.
        </p>
      )}

      <div className="panel-card space-y-4 p-5">
        <div className="grid gap-3 sm:grid-cols-2">
          <div>
            <div className="text-xs text-gray-500">App ID</div>
            <div className="font-mono text-sm">{gs.app_id}</div>
          </div>
          <div>
            <div className="text-xs text-gray-500">Provider</div>
            <div className="text-sm">{gs.provider}</div>
          </div>
        </div>
        {!gs.installation_id && (
          <div className="space-y-2">
            <p className="text-sm text-gray-500 dark:text-gray-400">
              Install the GitHub App on your org, then you will be redirected back here.
            </p>
            <Btn primary onClick={() => install.mutate()}>
              {install.isPending ? 'Preparing…' : 'Install on GitHub'}
            </Btn>
            {install.error && <p className="text-sm text-error-500">{install.error.message}</p>}
          </div>
        )}
      </div>

      {gs.installation_id && (
        <div className="panel-card overflow-hidden">
          <div className="border-b border-gray-200 px-3 py-2 text-sm font-medium dark:border-gray-800">
            Repositories
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
              <li className="px-4 py-8 text-center text-sm text-gray-500">No repositories visible.</li>
            )}
          </ul>
        </div>
      )}
    </div>
  )
}
