import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import { useMemo, useState } from 'react'
import { ServiceLogo } from '../components/ServiceLogo'
import { api, LAST_ENV_KEY } from '../lib/api'
import { Btn, Header, Input, Modal } from './Servers'

export function ProjectsPage() {
  const qc = useQueryClient()
  const nav = useNavigate()
  const projects = useQuery({ queryKey: ['projects'], queryFn: api.projects })
  const [show, setShow] = useState(false)
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const create = useMutation({
    mutationFn: () => api.createProject(name, description),
    onSuccess: (data) => {
      localStorage.setItem(LAST_ENV_KEY, data.environment.id)
      void qc.invalidateQueries({ queryKey: ['projects'] })
      void qc.invalidateQueries({ queryKey: ['all-environments'] })
      setShow(false)
      setName('')
      setDescription('')
      void nav({ to: '/projects/$projectId', params: { projectId: data.project.id } })
    },
  })

  return (
    <div className="space-y-6">
      <Header
        title="Projects"
        actions={
          <Btn primary onClick={() => setShow(true)}>
            + Add
          </Btn>
        }
      />
      <p className="text-sm text-gray-500 dark:text-gray-400">All your projects are here.</p>

      {(projects.data?.projects || []).length > 0 ? (
        <div className="grid gap-4 xl:grid-cols-2">
          {(projects.data?.projects || []).map((p) => (
            <div key={p.id} className="panel-card relative flex items-center gap-4 p-5">
              <Link
                to="/projects/$projectId"
                params={{ projectId: p.id }}
                className="absolute inset-0"
                aria-label={`Open ${p.name}`}
              />
              <div className="relative z-10 flex min-w-0 flex-1 flex-col">
                <div className="font-semibold text-gray-900 dark:text-white">{p.name}</div>
                {p.description && (
                  <div className="mt-1 truncate text-sm text-gray-500 dark:text-gray-400">{p.description}</div>
                )}
              </div>
              <div className="relative z-10 flex shrink-0 items-center gap-3 text-xs font-semibold">
                <Link
                  to="/projects/$projectId"
                  params={{ projectId: p.id }}
                  className="hover:underline text-brand-600 dark:text-brand-400"
                >
                  Open
                </Link>
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div className="panel-card p-8 text-sm text-gray-500 dark:text-gray-400">
          <p className="font-medium text-amber-600 dark:text-amber-400">No projects found.</p>
          <p className="mt-2">
            <button type="button" className="font-medium text-brand-600 dark:text-brand-400" onClick={() => setShow(true)}>
              Add
            </button>{' '}
            your first project or go to{' '}
            <Link to="/onboarding" className="underline dark:text-white">
              onboarding
            </Link>
            .
          </p>
        </div>
      )}

      {show && (
        <Modal title="New Project" onClose={() => setShow(false)}>
          <form
            className="space-y-3"
            onSubmit={(e) => {
              e.preventDefault()
              create.mutate()
            }}
          >
            <Input label="Name" value={name} onChange={setName} />
            <Input label="Description" value={description} onChange={setDescription} required={false} />
            {create.error && <p className="text-sm text-error-500">{create.error.message}</p>}
            <Btn primary type="submit">
              Create
            </Btn>
          </form>
        </Modal>
      )}
    </div>
  )
}

export function ApplicationsPage() {
  const qc = useQueryClient()
  const apps = useQuery({ queryKey: ['applications'], queryFn: () => api.applications() })

  return (
    <div className="space-y-6">
      <Header
        title="Applications"
        actions={
          <Link
            to="/applications/new"
            className="inline-flex h-8 items-center rounded-md bg-brand-500 px-2.5 text-xs font-medium text-white transition hover:bg-brand-600"
          >
            New application
          </Link>
        }
      />
      <div className="panel-card overflow-hidden">
        <table className="w-full text-left text-sm">
          <thead className="bg-gray-50 text-gray-500 dark:bg-white/5 dark:text-gray-400">
            <tr>
              <th className="px-3 py-2">Name</th>
              <th className="px-3 py-2">Build</th>
              <th className="px-3 py-2">Status</th>
              <th className="px-3 py-2">Domain</th>
              <th className="px-3 py-2">Actions</th>
            </tr>
          </thead>
          <tbody>
            {(apps.data?.applications || []).map((a) => (
              <tr key={a.id} className="border-t border-gray-200 dark:border-gray-800">
                <td className="px-3 py-2">
                  <Link
                    to="/applications/$appId"
                    params={{ appId: a.id }}
                    className="font-medium text-brand-600 hover:underline dark:text-brand-400"
                  >
                    {a.name}
                  </Link>
                </td>
                <td className="px-3 py-2 font-mono text-xs">{a.build_pack}</td>
                <td className="px-3 py-2">{a.status}</td>
                <td className="px-3 py-2">{a.fqdn || '—'}</td>
                <td className="space-x-3 px-3 py-2">
                  <Link
                    to="/applications/$appId"
                    params={{ appId: a.id }}
                    className="text-gray-500 hover:text-brand-600 dark:text-gray-400 dark:hover:text-brand-400"
                  >
                    Open
                  </Link>
                  <button
                    type="button"
                    className="text-brand-600 dark:text-brand-400"
                    onClick={() =>
                      void api.deployApplication(a.id).then(() => qc.invalidateQueries({ queryKey: ['applications'] }))
                    }
                  >
                    Deploy
                  </button>
                </td>
              </tr>
            ))}
            {!apps.data?.applications?.length && (
              <tr>
                <td colSpan={5} className="px-4 py-10 text-center text-gray-500 dark:text-gray-400">
                  No applications yet.{' '}
                  <Link to="/applications/new" className="font-medium text-brand-600 dark:text-brand-400">
                    Create one
                  </Link>
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}

export function DatabasesPage() {
  const qc = useQueryClient()
  const dbs = useQuery({ queryKey: ['databases'], queryFn: () => api.databases() })

  return (
    <div className="space-y-6">
      <Header
        title="Databases"
        actions={
          <Link
            to="/databases/new"
            className="inline-flex h-8 items-center rounded-md bg-brand-500 px-2.5 text-xs font-medium text-white transition hover:bg-brand-600"
          >
            New database
          </Link>
        }
      />

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {(dbs.data?.databases || []).map((d) => (
          <Link
            key={d.id}
            to="/databases/$dbId"
            params={{ dbId: d.id }}
            className="panel-card block p-5 transition hover:border-brand-300 dark:hover:border-brand-500/40"
          >
            <div className="font-medium text-gray-900 dark:text-white">{d.name}</div>
            <div className="mt-1 font-mono text-xs text-gray-500 dark:text-gray-400">{d.engine}</div>
            <div className="mt-3 text-sm text-gray-600 dark:text-gray-300">{d.status}</div>
            <div className="mt-4 flex gap-2" onClick={(e) => e.preventDefault()}>
              <Btn
                onClick={() =>
                  void api.startDatabase(d.id).then(() => qc.invalidateQueries({ queryKey: ['databases'] }))
                }
              >
                Start
              </Btn>
              <Btn
                onClick={() =>
                  void api.stopDatabase(d.id).then(() => qc.invalidateQueries({ queryKey: ['databases'] }))
                }
              >
                Stop
              </Btn>
            </div>
          </Link>
        ))}
        {!dbs.data?.databases?.length && (
          <div className="panel-card col-span-full p-8 text-center text-sm text-gray-500 dark:text-gray-400">
            No databases yet.{' '}
            <Link to="/databases/new" className="font-medium text-brand-600 dark:text-brand-400">
              Create one
            </Link>
          </div>
        )}
      </div>
    </div>
  )
}

export function ServicesPage() {
  const qc = useQueryClient()
  const services = useQuery({ queryKey: ['services'], queryFn: () => api.services() })
  const templates = useQuery({ queryKey: ['templates'], queryFn: api.templates })
  const [deployError, setDeployError] = useState('')
  const logoByType = useMemo(() => {
    const m = new Map<string, string>()
    for (const t of templates.data?.templates || []) {
      if (t.logo) m.set(t.type, t.logo)
    }
    return m
  }, [templates.data])

  return (
    <div className="space-y-6">
      <Header
        title="Services"
        actions={
          <Link
            to="/services/new"
            className="inline-flex h-8 items-center rounded-md bg-brand-500 px-2.5 text-xs font-medium text-white transition hover:bg-brand-600"
          >
            New service
          </Link>
        }
      />
      {deployError && <p className="text-sm text-error-500">{deployError}</p>}
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {(services.data?.services || []).map((s) => (
          <div key={s.id} className="panel-card p-5">
            <div className="flex items-start gap-3">
              <ServiceLogo
                src={logoByType.get(s.service_type)}
                name={s.name}
                className="h-10 w-10"
              />
              <div className="min-w-0 flex-1">
                <Link
                  to="/services/$svcId"
                  params={{ svcId: s.id }}
                  className="font-medium text-gray-900 hover:text-brand-600 dark:text-white dark:hover:text-brand-400"
                >
                  {s.name}
                </Link>
                <div className="mt-1 text-sm text-gray-500 dark:text-gray-400">{s.service_type}</div>
                <div className="mt-2 text-sm text-gray-600 dark:text-gray-300">{s.status}</div>
              </div>
            </div>
            <div className="mt-4">
              <Btn
                primary
                onClick={() => {
                  setDeployError('')
                  void api
                    .deployService(s.id)
                    .then(() => qc.invalidateQueries({ queryKey: ['services'] }))
                    .catch((e: Error) => setDeployError(e.message))
                }}
              >
                Deploy
              </Btn>
            </div>
          </div>
        ))}
        {!services.data?.services?.length && (
          <div className="panel-card col-span-full p-8 text-center text-sm text-gray-500 dark:text-gray-400">
            No services yet.{' '}
            <Link to="/services/new" className="font-medium text-brand-600 dark:text-brand-400">
              Create one
            </Link>
          </div>
        )}
      </div>
      <div>
        <div className="mb-3 flex items-center justify-between gap-3">
          <h2 className="text-lg font-medium text-gray-900 dark:text-white">Catalog preview</h2>
          <Link to="/services/new" className="text-sm font-medium text-brand-600 dark:text-brand-400">
            Browse all →
          </Link>
        </div>
        <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
          {(templates.data?.templates || []).slice(0, 8).map((t) => (
            <div key={t.type} className="panel-card flex items-center gap-3 px-3 py-3 text-sm">
              <ServiceLogo src={t.logo} name={t.name} className="h-8 w-8" />
              <div className="truncate font-medium text-gray-900 dark:text-white">{t.name}</div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
