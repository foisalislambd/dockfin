import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate, useParams, useSearch } from '@tanstack/react-router'
import { useEffect, useMemo, useRef, useState, type FormEvent } from 'react'
import { DomainsPanel, normalizeDomains } from '../components/DomainsPanel'
import { CodeEditor } from '../components/CodeEditor'
import { ServiceLogo } from '../components/ServiceLogo'
import { CreatePageShell, FormActions, FormInput, FormSelect } from '../components/ui/forms'
import { FormPageSkeleton } from '../components/ui/Skeleton'
import { api, fetchAllEnvironments, LAST_ENV_KEY } from '../lib/api'

const EMPTY_COMPOSE = `services:
  app:
    image: nginx:alpine
    # ports:
    #   - "80"
    # environment:
    #   - FOO=bar
`

export function CreateServicePage() {
  const nav = useNavigate()
  const qc = useQueryClient()
  const params = useParams({ strict: false }) as { projectId?: string; envId?: string }
  const search = useSearch({ strict: false }) as { environment_id?: string; empty_compose?: string }
  const emptyCompose = search.empty_compose === '1' || search.empty_compose === 'true'
  const prefillEnv = params.envId || search.environment_id || ''
  const templates = useQuery({ queryKey: ['templates'], queryFn: api.templates, enabled: !emptyCompose })
  const dests = useQuery({ queryKey: ['destinations'], queryFn: api.destinations })
  const envs = useQuery({ queryKey: ['all-environments'], queryFn: fetchAllEnvironments })
  const envTouched = useRef(false)
  const [searchQ, setSearchQ] = useState('')
  const [formError, setFormError] = useState('')
  const [composeRaw, setComposeRaw] = useState(EMPTY_COMPOSE)
  const [form, setForm] = useState({
    name: emptyCompose ? 'compose-stack' : '',
    environment_id: prefillEnv,
    destination_id: '',
    template: '',
    fqdn: '',
  })

  useEffect(() => {
    const saved = prefillEnv || localStorage.getItem(LAST_ENV_KEY) || ''
    const list = envs.data || []
    const pick = (saved && list.find((e) => e.id === saved)?.id) || list[0]?.id || ''
    const firstTpl = templates.data?.templates?.[0]?.type || ''
    setForm((f) => ({
      ...f,
      environment_id: envTouched.current ? f.environment_id : f.environment_id || prefillEnv || pick,
      destination_id: f.destination_id || dests.data?.destinations?.[0]?.id || '',
      template: emptyCompose ? '' : f.template || firstTpl,
    }))
  }, [envs.data, dests.data, templates.data, prefillEnv, emptyCompose])

  const filtered = useMemo(() => {
    const q = searchQ.trim().toLowerCase()
    const list = templates.data?.templates || []
    if (!q) return list.slice(0, 48)
    return list
      .filter(
        (t) =>
          t.name.toLowerCase().includes(q) ||
          t.type.toLowerCase().includes(q) ||
          (t.description || '').toLowerCase().includes(q) ||
          (t.category || '').toLowerCase().includes(q),
      )
      .slice(0, 48)
  }, [templates.data, searchQ])

  const selected = (templates.data?.templates || []).find((t) => t.type === form.template)

  const nested = Boolean(params.projectId && params.envId)
  const backEnvId = form.environment_id || params.envId || ''
  const backProjectId =
    (envs.data || []).find((e) => e.id === backEnvId)?.project_id || params.projectId || ''
  const backTo =
    nested && backProjectId && backEnvId
      ? `/projects/${backProjectId}/environments/${backEnvId}/new`
      : '/projects'
  const backLabel = nested ? 'Back to New Resource' : 'Back to projects'

  const create = useMutation({
    mutationFn: () => {
      const fqdn = form.fqdn ? normalizeDomains(form.fqdn) : undefined
      return api.createService(
        emptyCompose
          ? {
              name: form.name,
              environment_id: form.environment_id,
              destination_id: form.destination_id || undefined,
              docker_compose_raw: composeRaw,
              service_type: 'custom',
              fqdn,
            }
          : { ...form, fqdn },
      )
    },
    onSuccess: (svc) => {
      if (form.environment_id) localStorage.setItem(LAST_ENV_KEY, form.environment_id)
      void qc.invalidateQueries({ queryKey: ['services'] })
      const envMeta = (envs.data || []).find((e) => e.id === form.environment_id)
      const projectId = envMeta?.project_id || params.projectId
      const envId = form.environment_id || params.envId
      if (projectId && envId) {
        void nav({
          to: '/projects/$projectId/environments/$envId/services/$svcId',
          params: { projectId, envId, svcId: svc.id },
          search: { deploy: '1' },
        })
      } else {
        void nav({ to: '/projects' })
      }
    },
  })

  if ((!emptyCompose && templates.isLoading) || envs.isLoading || dests.isLoading) {
    return <FormPageSkeleton />
  }

  return (
    <CreatePageShell
      title={emptyCompose ? 'Empty Docker Compose' : 'New service'}
      backTo={backTo}
      backLabel={backLabel}
    >
      <form
        className="space-y-6"
        onSubmit={(e: FormEvent) => {
          e.preventDefault()
          setFormError('')
          if (emptyCompose) {
            if (!composeRaw.trim()) {
              setFormError('Paste a docker-compose.yaml.')
              return
            }
          } else if (!form.template) {
            setFormError('Select a service template before creating.')
            return
          }
          create.mutate()
        }}
      >
        <section className="space-y-4">
          <h2 className="text-sm font-semibold tracking-wide text-gray-500 uppercase dark:text-gray-400">
            Basics
          </h2>
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            <FormInput
              label="Name"
              value={form.name}
              onChange={(v) => setForm({ ...form, name: v })}
              placeholder={selected?.name || 'my-service'}
            />
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
            <FormSelect
              label="Destination"
              value={form.destination_id}
              onChange={(v) => setForm({ ...form, destination_id: v })}
              required={false}
              hint="Needed for free sslip.io / nip.io domain"
            >
              <option value="">Optional…</option>
              {(dests.data?.destinations || []).map((d) => (
                <option key={d.id} value={d.id}>
                  {d.name} ({d.network})
                </option>
              ))}
            </FormSelect>
          </div>
          <DomainsPanel
            value={form.fqdn}
            onChange={(v) => setForm({ ...form, fqdn: v })}
            serverId={
              (dests.data?.destinations || []).find((d) => d.id === form.destination_id)?.server_id ||
              undefined
            }
            destinationId={form.destination_id || undefined}
            resourceName={form.name || selected?.name || 'service'}
          />
        </section>

        {emptyCompose ? (
          <section className="space-y-3">
            <h2 className="text-sm font-semibold tracking-wide text-gray-500 uppercase dark:text-gray-400">
              Docker Compose
            </h2>
            <p className="text-sm text-gray-500 dark:text-gray-400">
              Paste a full compose file. Dockfin will prepare Traefik labels and magic domains on
              deploy.
            </p>
            <CodeEditor
              language="yaml"
              readOnly={false}
              height="28rem"
              ariaLabel="Docker Compose YAML"
              value={composeRaw}
              onChange={setComposeRaw}
            />
          </section>
        ) : (
          <section className="space-y-3">
            <div className="flex flex-wrap items-end justify-between gap-3">
              <h2 className="text-sm font-semibold tracking-wide text-gray-500 uppercase dark:text-gray-400">
                Template
              </h2>
              <input
                type="search"
                value={searchQ}
                onChange={(e) => setSearchQ(e.target.value)}
                placeholder="Search catalog…"
                className="panel-field h-8 w-full max-w-sm rounded-md px-3 text-sm"
              />
            </div>
            {selected && (
              <div className="flex items-start gap-3 rounded-xl border border-brand-200 bg-brand-50/70 px-4 py-3 text-sm dark:border-brand-500/30 dark:bg-brand-500/10">
                <ServiceLogo src={selected.logo} name={selected.name} className="h-10 w-10" />
                <div className="min-w-0">
                  <span className="font-semibold text-brand-700 dark:text-brand-300">
                    {selected.name}
                  </span>
                  <span className="text-gray-500 dark:text-gray-400"> — {selected.description}</span>
                </div>
              </div>
            )}
            <div className="grid max-h-[min(40rem,70vh)] gap-2 overflow-y-auto sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
              {filtered.map((t) => {
                const active = form.template === t.type
                return (
                  <button
                    key={t.type}
                    type="button"
                    onClick={() =>
                      setForm((f) => ({
                        ...f,
                        template: t.type,
                        name: f.name || t.name.toLowerCase().replace(/\s+/g, '-'),
                      }))
                    }
                    className={`flex items-start gap-3 rounded-lg border p-2.5 text-left transition ${
                      active
                        ? 'border-brand-500 bg-brand-50 ring-1 ring-brand-500/30 dark:bg-brand-500/10'
                        : 'border-gray-200 hover:border-gray-300 dark:border-gray-800'
                    }`}
                  >
                    <ServiceLogo src={t.logo} name={t.name} className="h-9 w-9" />
                    <div className="min-w-0 flex-1">
                      <div className="truncate text-sm font-medium text-gray-900 dark:text-white">
                        {t.name}
                      </div>
                      {t.category && (
                        <div className="mt-1 text-[10px] font-semibold tracking-wide text-gray-400 uppercase">
                          {t.category}
                        </div>
                      )}
                    </div>
                  </button>
                )
              })}
            </div>
          </section>
        )}

        {(formError || create.error) && (
          <p className="text-sm text-error-500" role="alert">
            {formError || create.error?.message}
          </p>
        )}
        <FormActions
          busy={create.isPending || (!emptyCompose && templates.isLoading)}
          submitLabel={emptyCompose ? 'Create compose service' : 'Create service'}
          cancelTo={backTo}
        />
      </form>
    </CreatePageShell>
  )
}
