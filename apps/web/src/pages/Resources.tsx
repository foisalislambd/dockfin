import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { useState } from 'react'
import { api, LAST_ENV_KEY } from '../lib/api'
import { Btn, Header, Input, Modal } from './Servers'

export function ProjectsPage() {
  const qc = useQueryClient()
  const projects = useQuery({ queryKey: ['projects'], queryFn: api.projects })
  const [show, setShow] = useState(false)
  const [name, setName] = useState('')
  const create = useMutation({
    mutationFn: () => api.createProject(name),
    onSuccess: (data) => {
      localStorage.setItem(LAST_ENV_KEY, data.environment.id)
      void qc.invalidateQueries({ queryKey: ['projects'] })
      void qc.invalidateQueries({ queryKey: ['all-environments'] })
      setShow(false)
      setName('')
    },
  })

  return (
    <div className="space-y-6">
      <Header
        title="Projects"
        actions={
          <Btn primary onClick={() => setShow(true)}>
            New project
          </Btn>
        }
      />
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {(projects.data?.projects || []).map((p) => (
          <div key={p.id} className="panel-card p-5">
            <div className="font-medium text-gray-900 dark:text-white">{p.name}</div>
          </div>
        ))}
      </div>
      {show && (
        <Modal title="Create project" onClose={() => setShow(false)}>
          <form
            className="space-y-3"
            onSubmit={(e) => {
              e.preventDefault()
              create.mutate()
            }}
          >
            <Input label="Name" value={name} onChange={setName} />
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
  const dbs = useQuery({ queryKey: ['databases'], queryFn: api.databases })

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
          <div key={d.id} className="panel-card p-5">
            <div className="font-medium text-gray-900 dark:text-white">{d.name}</div>
            <div className="mt-1 font-mono text-xs text-gray-500 dark:text-gray-400">{d.engine}</div>
            <div className="mt-3 text-sm text-gray-600 dark:text-gray-300">{d.status}</div>
            <div className="mt-4 flex gap-2">
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
          </div>
        ))}
        {!dbs.data?.databases?.length && (
          <div className="panel-card col-span-full p-8 text-center text-sm text-gray-500">
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
  const services = useQuery({ queryKey: ['services'], queryFn: api.services })
  const templates = useQuery({ queryKey: ['templates'], queryFn: api.templates })
  const [deployError, setDeployError] = useState('')

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
            <div className="font-medium text-gray-900 dark:text-white">{s.name}</div>
            <div className="mt-1 text-sm text-gray-500 dark:text-gray-400">{s.service_type}</div>
            <div className="mt-3 text-sm text-gray-600 dark:text-gray-300">{s.status}</div>
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
          <div className="panel-card col-span-full p-8 text-center text-sm text-gray-500">
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
            <div key={t.type} className="panel-card px-3 py-3 text-sm">
              <div className="font-medium text-gray-900 dark:text-white">{t.name}</div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
