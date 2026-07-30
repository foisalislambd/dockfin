import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate, useParams, useSearch } from '@tanstack/react-router'
import { useEffect, useRef, useState, type FormEvent } from 'react'
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

function useEnvPrefill() {
  const params = useParams({ strict: false }) as { projectId?: string; envId?: string }
  const search = useSearch({ strict: false }) as { environment_id?: string }
  return params.envId || search.environment_id || ''
}

export function CreateApplicationPage() {
  const nav = useNavigate()
  const qc = useQueryClient()
  const params = useParams({ strict: false }) as { projectId?: string; envId?: string }
  const prefillEnv = useEnvPrefill()
  const dests = useQuery({ queryKey: ['destinations'], queryFn: api.destinations })
  const envs = useQuery({ queryKey: ['all-environments'], queryFn: fetchAllEnvironments })
  const envTouched = useRef(false)
  const [form, setForm] = useState({
    name: '',
    environment_id: prefillEnv,
    destination_id: '',
    build_pack: 'dockerimage',
    docker_registry_image_name: 'nginx',
    docker_registry_image_tag: 'alpine',
    ports_exposes: '80',
    fqdn: '',
    git_repository: '',
  })

  useEffect(() => {
    const saved = prefillEnv || localStorage.getItem(LAST_ENV_KEY) || ''
    const list = envs.data || []
    const pick = (saved && list.find((e) => e.id === saved)?.id) || list[0]?.id || ''
    setForm((f) => ({
      ...f,
      environment_id: envTouched.current ? f.environment_id : f.environment_id || prefillEnv || pick,
      destination_id: f.destination_id || dests.data?.destinations?.[0]?.id || '',
    }))
  }, [envs.data, dests.data, prefillEnv])

  const nested = Boolean(params.projectId && params.envId)
  const backEnvId = form.environment_id || params.envId || ''
  const backProjectId =
    (envs.data || []).find((e) => e.id === backEnvId)?.project_id || params.projectId || ''
  const backTo =
    nested && backProjectId && backEnvId
      ? `/projects/${backProjectId}/environments/${backEnvId}`
      : nested && params.projectId && params.envId
        ? `/projects/${params.projectId}/environments/${params.envId}`
        : '/applications'
  const backLabel = nested ? 'Back to resources' : 'Back to applications'

  const create = useMutation({
    mutationFn: () => api.createApplication(form),
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
        void nav({ to: '/applications/$appId', params: { appId: app.id } })
      }
    },
  })

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    create.mutate()
  }

  return (
    <CreatePageShell title="New application" backTo={backTo} backLabel={backLabel}>
      <form className="space-y-6" onSubmit={onSubmit}>
        <section className="space-y-4">
          <h2 className="text-sm font-semibold tracking-wide text-gray-500 uppercase">Basics</h2>
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            <FormInput label="Name" value={form.name} onChange={(v) => setForm({ ...form, name: v })} placeholder="my-app" />
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
          <h2 className="text-sm font-semibold tracking-wide text-gray-500 uppercase">Build pack</h2>
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5">
            {BUILD_PACKS.map((bp) => (
              <ChoiceCard
                key={bp.id}
                active={form.build_pack === bp.id}
                title={bp.title}
                onClick={() => setForm({ ...form, build_pack: bp.id })}
              />
            ))}
          </div>
        </section>

        <section className="space-y-4">
          <h2 className="text-sm font-semibold tracking-wide text-gray-500 uppercase">Source</h2>
          {form.build_pack === 'dockerimage' ? (
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
          ) : (
            <FormInput
              label="Git repository"
              value={form.git_repository}
              onChange={(v) => setForm({ ...form, git_repository: v })}
              placeholder="https://github.com/org/repo.git"
            />
          )}
          <div className="grid gap-4 sm:grid-cols-2">
            <FormInput
              label="Port"
              value={form.ports_exposes}
              onChange={(v) => setForm({ ...form, ports_exposes: v })}
              placeholder="80"
            />
            <FormInput
              label="FQDN"
              value={form.fqdn}
              onChange={(v) => setForm({ ...form, fqdn: v })}
              required={false}
              placeholder="app.example.com"
              hint="optional"
            />
          </div>
        </section>

        {create.error && (
          <p className="text-sm text-error-500" role="alert">
            {create.error.message}
          </p>
        )}
        <FormActions busy={create.isPending} submitLabel="Create application" cancelTo={backTo} />
      </form>
    </CreatePageShell>
  )
}
