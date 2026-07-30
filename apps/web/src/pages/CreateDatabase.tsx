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

const ENGINES = [
  { id: 'postgresql', title: 'PostgreSQL', description: 'Relational DB — default for most apps.' },
  { id: 'mysql', title: 'MySQL', description: 'Classic relational database.' },
  { id: 'mariadb', title: 'MariaDB', description: 'MySQL-compatible community engine.' },
  { id: 'mongodb', title: 'MongoDB', description: 'Document store.' },
  { id: 'redis', title: 'Redis', description: 'In-memory cache / queue.' },
  { id: 'keydb', title: 'KeyDB', description: 'High-performance Redis fork.' },
  { id: 'dragonfly', title: 'Dragonfly', description: 'Modern Redis-compatible store.' },
  { id: 'clickhouse', title: 'ClickHouse', description: 'Columnar analytics database.' },
]

export function CreateDatabasePage() {
  const nav = useNavigate()
  const qc = useQueryClient()
  const params = useParams({ strict: false }) as { projectId?: string; envId?: string }
  const search = useSearch({ strict: false }) as { environment_id?: string }
  const prefillEnv = params.envId || search.environment_id || ''
  const dests = useQuery({ queryKey: ['destinations'], queryFn: api.destinations })
  const envs = useQuery({ queryKey: ['all-environments'], queryFn: fetchAllEnvironments })
  const envTouched = useRef(false)
  const [password, setPassword] = useState<string | null>(null)
  const [createdId, setCreatedId] = useState<string | null>(null)
  const [form, setForm] = useState({
    name: '',
    engine: 'postgresql',
    environment_id: prefillEnv,
    destination_id: '',
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
        : '/databases'
  const backLabel = nested ? 'Back to resources' : 'Back to databases'

  const goToDetail = (id: string, environmentId = form.environment_id) => {
    const envMeta = (envs.data || []).find((e) => e.id === environmentId)
    const projectId = envMeta?.project_id || params.projectId
    const envId = environmentId || params.envId
    if (projectId && envId && id) {
      void nav({
        to: '/projects/$projectId/environments/$envId/databases/$dbId',
        params: { projectId, envId, dbId: id },
      })
    } else if (id) {
      void nav({ to: '/databases/$dbId', params: { dbId: id } })
    }
  }

  const create = useMutation({
    mutationFn: () => api.createDatabase(form),
    onSuccess: (data) => {
      if (form.environment_id) localStorage.setItem(LAST_ENV_KEY, form.environment_id)
      void qc.invalidateQueries({ queryKey: ['databases'] })
      setCreatedId(data.database.id)
      if (data.password) {
        setPassword(data.password)
      } else {
        goToDetail(data.database.id, data.database.environment_id || form.environment_id)
      }
    },
  })

  if (password && createdId) {
    return (
      <CreatePageShell title="Database created" backTo={backTo} backLabel={backLabel}>
        <div className="space-y-4">
          <div className="rounded-xl border border-amber-200 bg-amber-50 p-4 dark:border-amber-500/30 dark:bg-amber-500/10">
            <p className="text-sm font-medium text-amber-800 dark:text-amber-200">Generated password</p>
            <code className="mt-2 block break-all font-mono text-sm text-gray-900 dark:text-white">{password}</code>
          </div>
          <button
            type="button"
            onClick={() => goToDetail(createdId)}
            className="inline-flex h-8 items-center justify-center rounded-md bg-brand-500 px-3 text-xs font-medium text-white hover:bg-brand-600"
          >
            Done
          </button>
        </div>
      </CreatePageShell>
    )
  }

  return (
    <CreatePageShell title="New database" backTo={backTo} backLabel={backLabel}>
      <form
        className="space-y-6"
        onSubmit={(e: FormEvent) => {
          e.preventDefault()
          create.mutate()
        }}
      >
        <section className="space-y-4">
          <h2 className="text-sm font-semibold tracking-wide text-gray-500 uppercase">Basics</h2>
          <FormInput label="Name" value={form.name} onChange={(v) => setForm({ ...form, name: v })} placeholder="app-db" />
          <div className="grid gap-4 sm:grid-cols-2">
            <FormSelect
              label="Environment"
              value={form.environment_id}
              onChange={(v) => {
                envTouched.current = true
                setForm({ ...form, environment_id: v })
              }}
            >
              <option value="">Select…</option>
              {(envs.data || []).map((e) => (
                <option key={e.id} value={e.id}>
                  {e.project_name} / {e.name}
                </option>
              ))}
            </FormSelect>
            <FormSelect label="Destination" value={form.destination_id} onChange={(v) => setForm({ ...form, destination_id: v })}>
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
          <h2 className="text-sm font-semibold tracking-wide text-gray-500 uppercase">Engine</h2>
          <div className="grid gap-3 sm:grid-cols-2">
            {ENGINES.map((eng) => (
              <ChoiceCard
                key={eng.id}
                active={form.engine === eng.id}
                title={eng.title}
                onClick={() => setForm({ ...form, engine: eng.id })}
              />
            ))}
          </div>
        </section>

        {create.error && (
          <p className="text-sm text-error-500" role="alert">
            {create.error.message}
          </p>
        )}
        <FormActions busy={create.isPending} submitLabel="Create database" cancelTo={backTo} />
      </form>
    </CreatePageShell>
  )
}
