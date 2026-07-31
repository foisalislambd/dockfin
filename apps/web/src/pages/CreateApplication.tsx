import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useParams, useSearch } from '@tanstack/react-router'
import { useEffect, useMemo, useRef, useState, type FormEvent } from 'react'
import {
  ChoiceCard,
  CreatePageShell,
  FormActions,
  FormInput,
  FormSelect,
} from '../components/ui/forms'
import { api, fetchAllEnvironments, LAST_ENV_KEY } from '../lib/api'

const BUILD_PACKS = [
  { id: 'dockerimage', title: 'Docker Image', description: 'Pull and run a public or private image.' },
  { id: 'dockerfile', title: 'Dockerfile', description: 'Build from a Git repo Dockerfile.' },
  { id: 'dockercompose', title: 'Compose', description: 'Deploy a docker-compose stack from Git.' },
  { id: 'nixpacks', title: 'Nixpacks', description: 'Auto-detect and build from source.' },
  { id: 'static', title: 'Static', description: 'Static site / SPA build output.' },
]

type SourceType = 'public' | 'private-gh-app' | 'private-deploy-key' | ''

export function CreateApplicationPage() {
  const nav = useNavigate()
  const qc = useQueryClient()
  const params = useParams({ strict: false }) as { projectId?: string; envId?: string }
  const search = useSearch({ strict: false }) as {
    environment_id?: string
    build_pack?: string
    source_type?: SourceType
  }
  const prefillEnv = params.envId || search.environment_id || ''
  const prefillPack = search.build_pack || ''
  const sourceType = (search.source_type || '') as SourceType

  const dests = useQuery({ queryKey: ['destinations'], queryFn: api.destinations })
  const envs = useQuery({ queryKey: ['all-environments'], queryFn: fetchAllEnvironments })
  const gitSources = useQuery({
    queryKey: ['git-sources'],
    queryFn: api.gitSources,
    enabled: sourceType === 'private-gh-app',
  })
  const keys = useQuery({
    queryKey: ['keys'],
    queryFn: api.keys,
    enabled: sourceType === 'private-deploy-key',
  })

  const installedSources = useMemo(
    () => (gitSources.data?.git_sources || []).filter((g) => g.installed && g.configured),
    [gitSources.data],
  )

  const envTouched = useRef(false)
  const [form, setForm] = useState({
    name: '',
    environment_id: prefillEnv,
    destination_id: '',
    build_pack: prefillPack || 'dockerimage',
    docker_registry_image_name: 'nginx',
    docker_registry_image_tag: 'alpine',
    ports_exposes: '80',
    fqdn: '',
    git_repository: '',
    git_branch: 'main',
    git_source_id: '',
    private_key_id: '',
  })

  const [repoOwner, setRepoOwner] = useState('')
  const [repoName, setRepoName] = useState('')

  const repos = useQuery({
    queryKey: ['git-source-repos', form.git_source_id],
    queryFn: () => api.gitSourceRepositories(form.git_source_id),
    enabled: Boolean(form.git_source_id) && sourceType === 'private-gh-app',
  })

  const branches = useQuery({
    queryKey: ['git-source-branches', form.git_source_id, repoOwner, repoName],
    queryFn: () => api.gitSourceBranches(form.git_source_id, repoOwner, repoName),
    enabled:
      Boolean(form.git_source_id && repoOwner && repoName) && sourceType === 'private-gh-app',
  })

  useEffect(() => {
    const saved = prefillEnv || localStorage.getItem(LAST_ENV_KEY) || ''
    const list = envs.data || []
    const pick = (saved && list.find((e) => e.id === saved)?.id) || list[0]?.id || ''
    setForm((f) => ({
      ...f,
      environment_id: envTouched.current ? f.environment_id : f.environment_id || prefillEnv || pick,
      destination_id: f.destination_id || dests.data?.destinations?.[0]?.id || '',
      build_pack: prefillPack || f.build_pack || (sourceType ? 'nixpacks' : f.build_pack),
    }))
  }, [envs.data, dests.data, prefillEnv, prefillPack, sourceType])

  useEffect(() => {
    if (sourceType === 'private-gh-app' && !form.git_source_id && installedSources[0]) {
      setForm((f) => ({ ...f, git_source_id: installedSources[0].id }))
    }
  }, [sourceType, installedSources, form.git_source_id])

  const nested = Boolean(params.projectId && params.envId)
  const backEnvId = form.environment_id || params.envId || ''
  const backProjectId =
    (envs.data || []).find((e) => e.id === backEnvId)?.project_id || params.projectId || ''
  const backTo =
    nested && backProjectId && backEnvId
      ? `/projects/${backProjectId}/environments/${backEnvId}/new`
      : '/projects'
  const backLabel = nested ? 'Back to New Resource' : 'Back to projects'

  const needsGit = form.build_pack !== 'dockerimage'

  const create = useMutation({
    mutationFn: () => {
      const body: Record<string, unknown> = { ...form }
      if (!body.git_source_id) delete body.git_source_id
      if (!body.private_key_id) delete body.private_key_id
      return api.createApplication(body)
    },
    onSuccess: (app) => {
      if (form.environment_id) localStorage.setItem(LAST_ENV_KEY, form.environment_id)
      void qc.invalidateQueries({ queryKey: ['applications'] })
      const envMeta = (envs.data || []).find((e) => e.id === form.environment_id)
      const projectId = envMeta?.project_id || params.projectId
      const envId = form.environment_id || params.envId
      if (projectId && envId) {
        void nav({
          to: '/projects/$projectId/environments/$envId/applications/$appId',
          params: { projectId, envId, appId: app.id },
        })
      } else {
        void nav({ to: '/projects' })
      }
    },
  })

  const [formError, setFormError] = useState('')

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    setFormError('')
    if (sourceType === 'private-gh-app' && (!form.git_source_id || !form.git_repository)) {
      setFormError('Select a GitHub App and repository.')
      return
    }
    if (sourceType === 'private-deploy-key' && (!form.private_key_id || !form.git_repository)) {
      setFormError('Select a deploy key and enter the repository URL.')
      return
    }
    if (needsGit && sourceType !== 'private-gh-app' && sourceType !== 'private-deploy-key' && !form.git_repository && form.build_pack !== 'dockerimage') {
      setFormError('Git repository is required.')
      return
    }
    create.mutate()
  }

  const title =
    sourceType === 'private-gh-app'
      ? 'Private Repository (GitHub App)'
      : sourceType === 'private-deploy-key'
        ? 'Private Repository (Deploy Key)'
        : sourceType === 'public'
          ? 'Public Repository'
          : 'New application'

  return (
    <CreatePageShell title={title} backTo={backTo} backLabel={backLabel}>
      <form className="space-y-6" onSubmit={onSubmit}>
        <section className="space-y-4">
          <h2 className="text-sm font-semibold tracking-wide text-gray-500 uppercase dark:text-gray-400">
            Basics
          </h2>
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            <FormInput
              label="Name"
              value={form.name}
              onChange={(v) => setForm({ ...form, name: v })}
              placeholder="my-app"
            />
            <FormSelect
              label="Environment"
              value={form.environment_id}
              onChange={(v) => {
                envTouched.current = true
                setForm({ ...form, environment_id: v })
              }}
              hint={!envs.data?.length ? 'Create a project first' : undefined}
            >
              <option value="">Select…</option>
              {(envs.data || []).map((e) => (
                <option key={e.id} value={e.id}>
                  {e.project_name} / {e.name}
                </option>
              ))}
            </FormSelect>
            <FormSelect
              label="Destination"
              value={form.destination_id}
              onChange={(v) => setForm({ ...form, destination_id: v })}
            >
              <option value="">Select…</option>
              {(dests.data?.destinations || []).map((d) => (
                <option key={d.id} value={d.id}>
                  {d.name} ({d.network})
                </option>
              ))}
            </FormSelect>
          </div>
        </section>

        <section className="space-y-3">
          <h2 className="text-sm font-semibold tracking-wide text-gray-500 uppercase dark:text-gray-400">
            Build pack
          </h2>
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5">
            {BUILD_PACKS.filter((bp) => (needsGit || sourceType ? bp.id !== 'dockerimage' || !sourceType : true)).map(
              (bp) => (
                <ChoiceCard
                  key={bp.id}
                  active={form.build_pack === bp.id}
                  title={bp.title}
                  onClick={() => setForm({ ...form, build_pack: bp.id })}
                />
              ),
            )}
          </div>
        </section>

        <section className="space-y-4">
          <h2 className="text-sm font-semibold tracking-wide text-gray-500 uppercase dark:text-gray-400">
            Source
          </h2>

          {form.build_pack === 'dockerimage' && !sourceType ? (
            <div className="grid gap-4 sm:grid-cols-2">
              <FormInput
                label="Image"
                value={form.docker_registry_image_name}
                onChange={(v) => setForm({ ...form, docker_registry_image_name: v })}
                placeholder="nginx"
              />
              <FormInput
                label="Tag"
                value={form.docker_registry_image_tag}
                onChange={(v) => setForm({ ...form, docker_registry_image_tag: v })}
                placeholder="alpine"
              />
            </div>
          ) : sourceType === 'private-gh-app' ? (
            <div className="space-y-4">
              <FormSelect
                label="GitHub App"
                value={form.git_source_id}
                onChange={(v) => {
                  setForm({ ...form, git_source_id: v, git_repository: '', git_branch: 'main' })
                  setRepoOwner('')
                  setRepoName('')
                }}
                hint={
                  !installedSources.length
                    ? 'Connect and install a GitHub App under Sources first.'
                    : undefined
                }
              >
                <option value="">Select…</option>
                {installedSources.map((gs) => (
                  <option key={gs.id} value={gs.id}>
                    {gs.name}
                  </option>
                ))}
              </FormSelect>
              {!installedSources.length && (
                <Link to="/git-sources" className="text-sm text-brand-600 hover:underline dark:text-brand-400">
                  + Add GitHub App
                </Link>
              )}
              <FormSelect
                label="Repository"
                value={form.git_repository}
                onChange={(v) => {
                  const full = v.replace(/\.git$/, '')
                  const [owner, name] = full.includes('/')
                    ? full.replace(/^https?:\/\/[^/]+\//, '').split('/')
                    : ['', '']
                  setRepoOwner(owner || '')
                  setRepoName((name || '').replace(/\.git$/, ''))
                  setForm({ ...form, git_repository: v, git_branch: 'main' })
                }}
              >
                <option value="">Select…</option>
                {(repos.data?.repositories || []).map((r) => {
                  const full = String(r.full_name || '')
                  const clone = String(r.clone_url || r.html_url || full)
                  return (
                    <option key={full} value={clone || full}>
                      {full}
                    </option>
                  )
                })}
              </FormSelect>
              <FormSelect
                label="Branch"
                value={form.git_branch}
                onChange={(v) => setForm({ ...form, git_branch: v })}
              >
                <option value="main">main</option>
                {(branches.data?.branches || [])
                  .filter((b) => b !== 'main')
                  .map((b) => (
                    <option key={b} value={b}>
                      {b}
                    </option>
                  ))}
              </FormSelect>
            </div>
          ) : sourceType === 'private-deploy-key' ? (
            <div className="space-y-4">
              <FormSelect
                label="Deploy Key"
                value={form.private_key_id}
                onChange={(v) => setForm({ ...form, private_key_id: v })}
                hint="Add an SSH key under Keys & Tokens, then add its public key as a deploy key on the repo."
              >
                <option value="">Select…</option>
                {(keys.data?.private_keys || []).map((k) => (
                  <option key={k.id} value={k.id}>
                    {k.name}
                  </option>
                ))}
              </FormSelect>
              <FormInput
                label="Git repository"
                value={form.git_repository}
                onChange={(v) => setForm({ ...form, git_repository: v })}
                placeholder="git@github.com:org/repo.git"
              />
              <FormInput
                label="Branch"
                value={form.git_branch}
                onChange={(v) => setForm({ ...form, git_branch: v })}
                placeholder="main"
              />
            </div>
          ) : (
            <div className="space-y-4">
              <FormInput
                label="Git repository"
                value={form.git_repository}
                onChange={(v) => setForm({ ...form, git_repository: v })}
                placeholder="https://github.com/org/repo.git"
              />
              <FormInput
                label="Branch"
                value={form.git_branch}
                onChange={(v) => setForm({ ...form, git_branch: v })}
                placeholder="main"
              />
            </div>
          )}

          <div className="grid gap-4 sm:grid-cols-2">
            <FormInput
              label="Port"
              value={form.ports_exposes}
              onChange={(v) => setForm({ ...form, ports_exposes: v })}
              placeholder="80"
            />
            <div className="space-y-2">
              <FormInput
                label="FQDN"
                value={form.fqdn}
                onChange={(v) => setForm({ ...form, fqdn: v })}
                required={false}
                placeholder="Leave empty for free sslip.io"
                hint="Empty = auto free domain (sslip.io / nip.io)"
              />
              <button
                type="button"
                className="text-xs font-medium text-brand-600 hover:underline dark:text-brand-400"
                disabled={!form.name || !form.destination_id}
                onClick={() => {
                  void api
                    .generateDomain({
                      name: form.name || 'app',
                      destination_id: form.destination_id || undefined,
                    })
                    .then((d) => setForm((f) => ({ ...f, fqdn: d.fqdn })))
                    .catch(() => undefined)
                }}
              >
                Generate free domain
              </button>
            </div>
          </div>
        </section>

        {(formError || create.error) && (
          <p className="text-sm text-error-500" role="alert">
            {formError || create.error?.message}
          </p>
        )}
        <FormActions busy={create.isPending} submitLabel="Create application" cancelTo={backTo} />
      </form>
    </CreatePageShell>
  )
}
