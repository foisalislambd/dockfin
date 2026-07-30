import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useParams } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { Meta, ResourceTabs, TabPanel } from '../components/ui/tabs'
import { api } from '../lib/api'
import { Btn, Input } from './Servers'

const APP_TABS = [
  { id: 'configuration', label: 'Configuration' },
  { id: 'environment', label: 'Environment Variables' },
  { id: 'deployments', label: 'Deployments' },
  { id: 'tasks', label: 'Scheduled Tasks' },
  { id: 'webhooks', label: 'Webhooks' },
  { id: 'rollback', label: 'Rollback' },
  { id: 'danger', label: 'Danger' },
]

export function ApplicationDetailPage() {
  const { appId, projectId, envId } = useParams({ strict: false }) as {
    appId: string
    projectId?: string
    envId?: string
  }
  const nav = useNavigate()
  const qc = useQueryClient()
  const nested = Boolean(projectId && envId)

  const app = useQuery({ queryKey: ['application', appId], queryFn: () => api.application(appId) })
  const dests = useQuery({ queryKey: ['destinations'], queryFn: api.destinations })
  const deps = useQuery({
    queryKey: ['deployments', appId],
    queryFn: () => api.deployments(appId),
    refetchInterval: (q) => {
      const list = q.state.data?.deployments || []
      const busy = list.some((d) => d.status === 'queued' || d.status === 'in_progress')
      return busy ? 3000 : false
    },
  })
  const envVars = useQuery({
    queryKey: ['env-vars', appId],
    queryFn: () => api.envVars('application', appId, true),
  })

  const [tab, setTab] = useState('configuration')
  const [cfg, setCfg] = useState({
    name: '',
    description: '',
    fqdn: '',
    git_repository: '',
    git_branch: '',
    ports_exposes: '',
    docker_registry_image_name: '',
    docker_registry_image_tag: '',
    destination_id: '',
  })
  const [envKey, setEnvKey] = useState('')
  const [envValue, setEnvValue] = useState('')
  const [webhookSecret, setWebhookSecret] = useState<string | null>(null)
  const [taskName, setTaskName] = useState('')
  const [taskCommand, setTaskCommand] = useState('')
  const [taskFrequency, setTaskFrequency] = useState('0 * * * *')

  const tasks = useQuery({
    queryKey: ['scheduled-tasks', appId],
    queryFn: () => api.scheduledTasks({ resource_type: 'application', resource_id: appId }),
  })
  const createTask = useMutation({
    mutationFn: () =>
      api.createScheduledTask({
        resource_type: 'application',
        resource_id: appId,
        name: taskName,
        command: taskCommand,
        frequency: taskFrequency,
      }),
    onSuccess: () => {
      setTaskName('')
      setTaskCommand('')
      void qc.invalidateQueries({ queryKey: ['scheduled-tasks', appId] })
    },
  })

  // Reset local UI state whenever the route resource changes (component may be reused).
  useEffect(() => {
    setTab('configuration')
    setEnvKey('')
    setEnvValue('')
    setWebhookSecret(null)
    setTaskName('')
    setTaskCommand('')
    setTaskFrequency('0 * * * *')
    setCfg({
      name: '',
      description: '',
      fqdn: '',
      git_repository: '',
      git_branch: '',
      ports_exposes: '',
      docker_registry_image_name: '',
      docker_registry_image_tag: '',
      destination_id: '',
    })
  }, [appId])

  useEffect(() => {
    if (!app.data || app.data.id !== appId) return
    setCfg({
      name: app.data.name || '',
      description: app.data.description || '',
      fqdn: app.data.fqdn || '',
      git_repository: app.data.git_repository || '',
      git_branch: app.data.git_branch || 'main',
      ports_exposes: app.data.ports_exposes || '80',
      docker_registry_image_name: app.data.docker_registry_image_name || '',
      docker_registry_image_tag: app.data.docker_registry_image_tag || '',
      destination_id: app.data.destination_id || '',
    })
  }, [app.data, appId])

  const activeDep = (deps.data?.deployments || []).find(
    (d) => d.status === 'queued' || d.status === 'in_progress',
  )

  const save = useMutation({
    mutationFn: () => api.updateApplication(appId, cfg),
    onSuccess: (updated) => {
      void qc.invalidateQueries({ queryKey: ['application', appId] })
      setCfg({
        name: updated.name || '',
        description: updated.description || '',
        fqdn: updated.fqdn || '',
        git_repository: updated.git_repository || '',
        git_branch: updated.git_branch || 'main',
        ports_exposes: updated.ports_exposes || '80',
        docker_registry_image_name: updated.docker_registry_image_name || '',
        docker_registry_image_tag: updated.docker_registry_image_tag || '',
        destination_id: updated.destination_id || '',
      })
    },
  })

  const deploy = useMutation({
    mutationFn: (vars: { force?: boolean } = {}) => api.deployApplication(appId, Boolean(vars.force)),
    onSuccess: (dep) => {
      void qc.invalidateQueries({ queryKey: ['deployments', appId] })
      void qc.invalidateQueries({ queryKey: ['application', appId] })
      if (nested && projectId && envId) {
        void nav({
          to: '/projects/$projectId/environments/$envId/applications/$appId/deployments/$deploymentId',
          params: { projectId, envId, appId, deploymentId: dep.id },
        })
      } else {
        void nav({
          to: '/applications/$appId/deployments/$deploymentId',
          params: { appId, deploymentId: dep.id },
        })
      }
    },
  })

  const remove = useMutation({
    mutationFn: () => api.deleteApplication(appId),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['applications'] })
      if (nested && projectId && envId) {
        void nav({ to: '/projects/$projectId/environments/$envId', params: { projectId, envId } })
      } else {
        void nav({ to: '/applications' })
      }
    },
  })

  const cancel = useMutation({
    mutationFn: (id: string) => api.cancelDeployment(id),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['deployments', appId] }),
  })

  const rollback = useMutation({
    mutationFn: () => api.rollbackApplication(appId),
    onSuccess: (dep) => {
      void qc.invalidateQueries({ queryKey: ['deployments', appId] })
      if (nested && projectId && envId) {
        void nav({
          to: '/projects/$projectId/environments/$envId/applications/$appId/deployments/$deploymentId',
          params: { projectId, envId, appId, deploymentId: dep.id },
        })
      } else {
        void nav({
          to: '/applications/$appId/deployments/$deploymentId',
          params: { appId, deploymentId: dep.id },
        })
      }
    },
  })

  const addEnv = useMutation({
    mutationFn: () =>
      api.upsertEnvVar({
        resource_type: 'application',
        resource_id: appId,
        key: envKey,
        value: envValue,
      }),
    onSuccess: () => {
      setEnvKey('')
      setEnvValue('')
      void qc.invalidateQueries({ queryKey: ['env-vars', appId] })
    },
  })

  const delEnv = useMutation({
    mutationFn: (id: string) => api.deleteEnvVar(id),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['env-vars', appId] }),
  })

  const webhook = useMutation({
    mutationFn: () => api.setWebhookSecret(appId),
    onSuccess: (data) => setWebhookSecret(data.secret),
  })

  if (app.isLoading) {
    return <p className="text-gray-500 dark:text-gray-400">Loading…</p>
  }

  const backLink =
    nested && projectId && envId ? (
      <Link
        to="/projects/$projectId/environments/$envId"
        params={{ projectId, envId }}
        className="text-sm text-gray-500 hover:text-brand-600 dark:text-gray-400 dark:hover:text-brand-400"
      >
        ← Resources
      </Link>
    ) : (
      <Link
        to="/applications"
        className="text-sm text-gray-500 hover:text-brand-600 dark:text-gray-400 dark:hover:text-brand-400"
      >
        ← Applications
      </Link>
    )

  if (app.error || !app.data) {
    return (
      <div className="space-y-4">
        <p className="text-error-500">{app.error?.message || 'Application not found'}</p>
        {backLink}
      </div>
    )
  }

  const a = app.data
  const webhookUrl =
    typeof window !== 'undefined' ? `${window.location.origin}/api/v1/webhooks/git/${appId}` : `/api/v1/webhooks/git/${appId}`

  const openDeployment = (deploymentId: string) => {
    if (nested && projectId && envId) {
      void nav({
        to: '/projects/$projectId/environments/$envId/applications/$appId/deployments/$deploymentId',
        params: { projectId, envId, appId, deploymentId },
      })
    } else {
      void nav({
        to: '/applications/$appId/deployments/$deploymentId',
        params: { appId, deploymentId },
      })
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          {backLink}
          <h1 className="mt-2 text-xl font-semibold tracking-tight text-gray-900 dark:text-white sm:text-2xl">
            {a.name}
          </h1>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {a.build_pack} · {a.status}
            {a.fqdn ? ` · ${a.fqdn}` : ''}
          </p>
        </div>
        <div className="flex gap-2">
          {activeDep && <Btn onClick={() => cancel.mutate(activeDep.id)}>Cancel deploy</Btn>}
          <Btn primary onClick={() => deploy.mutate({})}>
            Deploy
          </Btn>
        </div>
      </div>

      <div className="grid gap-3 sm:grid-cols-3">
        <Meta label="Status" value={a.status} />
        <Meta label="Build pack" value={a.build_pack} />
        <Meta label="FQDN" value={a.fqdn || '—'} />
      </div>

      <ResourceTabs tabs={APP_TABS} active={tab} onChange={setTab} />

      {tab === 'configuration' && (
        <TabPanel>
          <form
            className="space-y-4"
            onSubmit={(e) => {
              e.preventDefault()
              save.mutate()
            }}
          >
            <div className="grid gap-4 sm:grid-cols-2">
              <Input label="Name" value={cfg.name} onChange={(v) => setCfg({ ...cfg, name: v })} />
              <Input
                label="FQDN"
                value={cfg.fqdn}
                onChange={(v) => setCfg({ ...cfg, fqdn: v })}
                required={false}
              />
              <Input
                label="Description"
                value={cfg.description}
                onChange={(v) => setCfg({ ...cfg, description: v })}
                required={false}
              />
              <Input
                label="Ports"
                value={cfg.ports_exposes}
                onChange={(v) => setCfg({ ...cfg, ports_exposes: v })}
              />
              {a.build_pack === 'dockerimage' ? (
                <>
                  <Input
                    label="Image"
                    value={cfg.docker_registry_image_name}
                    onChange={(v) => setCfg({ ...cfg, docker_registry_image_name: v })}
                  />
                  <Input
                    label="Tag"
                    value={cfg.docker_registry_image_tag}
                    onChange={(v) => setCfg({ ...cfg, docker_registry_image_tag: v })}
                  />
                </>
              ) : (
                <>
                  <Input
                    label="Git repository"
                    value={cfg.git_repository}
                    onChange={(v) => setCfg({ ...cfg, git_repository: v })}
                    required={false}
                  />
                  <Input
                    label="Git branch"
                    value={cfg.git_branch}
                    onChange={(v) => setCfg({ ...cfg, git_branch: v })}
                    required={false}
                  />
                </>
              )}
              <label className="block text-sm sm:col-span-2">
                <span className="mb-1 block text-gray-500 dark:text-gray-400">Destination</span>
                <select
                  value={cfg.destination_id}
                  onChange={(e) => setCfg({ ...cfg, destination_id: e.target.value })}
                  className="w-full rounded-lg border border-gray-200 bg-white px-3 py-2 dark:border-gray-800 dark:bg-gray-900"
                >
                  <option value="">Select…</option>
                  {(dests.data?.destinations || []).map((d) => (
                    <option key={d.id} value={d.id}>
                      {d.name} ({d.network})
                    </option>
                  ))}
                </select>
              </label>
            </div>
            {save.error && <p className="text-sm text-error-500">{save.error.message}</p>}
            <Btn primary type="submit">
              {save.isPending ? 'Saving…' : 'Save'}
            </Btn>
          </form>
        </TabPanel>
      )}

      {tab === 'environment' && (
        <TabPanel>
          <div className="panel-card overflow-hidden">
            <table className="w-full text-left text-sm">
              <thead className="bg-gray-50 text-gray-500 dark:bg-white/5 dark:text-gray-400">
                <tr>
                  <th className="px-3 py-2">Key</th>
                  <th className="px-3 py-2">Value</th>
                  <th className="px-3 py-2">Actions</th>
                </tr>
              </thead>
              <tbody>
                {(envVars.data?.environment_variables || []).map((v) => (
                  <tr key={v.id} className="border-t border-gray-200 dark:border-gray-800">
                    <td className="px-3 py-2 font-mono text-xs">{v.key}</td>
                    <td className="px-3 py-2 font-mono text-xs text-gray-500 dark:text-gray-400">
                      {v.value ?? '••••'}
                    </td>
                    <td className="px-3 py-2">
                      <button
                        type="button"
                        className="text-error-500"
                        onClick={() => delEnv.mutate(v.id)}
                      >
                        Delete
                      </button>
                    </td>
                  </tr>
                ))}
                {!envVars.data?.environment_variables?.length && (
                  <tr>
                    <td colSpan={3} className="px-4 py-8 text-center text-gray-500">
                      No env vars yet.
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
          <form
            className="mt-4 flex flex-wrap items-end gap-3"
            onSubmit={(e) => {
              e.preventDefault()
              addEnv.mutate()
            }}
          >
            <div className="min-w-[140px] flex-1">
              <Input label="Key" value={envKey} onChange={setEnvKey} />
            </div>
            <div className="min-w-[180px] flex-1">
              <Input label="Value" value={envValue} onChange={setEnvValue} />
            </div>
            <Btn primary type="submit">
              Add
            </Btn>
            {addEnv.error && <p className="w-full text-sm text-error-500">{addEnv.error.message}</p>}
          </form>
        </TabPanel>
      )}

      {tab === 'deployments' && (
        <TabPanel>
          <div className="panel-card overflow-hidden">
            <table className="w-full text-left text-sm">
              <thead className="bg-gray-50 text-gray-500 dark:bg-white/5 dark:text-gray-400">
                <tr>
                  <th className="px-3 py-2">ID</th>
                  <th className="px-3 py-2">Status</th>
                  <th className="px-3 py-2">Stage</th>
                  <th className="px-3 py-2">Created</th>
                  <th className="px-3 py-2">Actions</th>
                </tr>
              </thead>
              <tbody>
                {(deps.data?.deployments || []).map((d) => (
                  <tr key={d.id} className="border-t border-gray-200 dark:border-gray-800">
                    <td className="px-3 py-2 font-mono text-xs">{d.id.slice(0, 8)}…</td>
                    <td className="px-3 py-2">{d.status}</td>
                    <td className="px-3 py-2">{d.current_stage || '—'}</td>
                    <td className="px-3 py-2 text-gray-500 dark:text-gray-400">
                      {new Date(d.created_at).toLocaleString()}
                    </td>
                    <td className="space-x-3 px-3 py-2">
                      <button
                        type="button"
                        className="text-brand-600 dark:text-brand-400"
                        onClick={() => openDeployment(d.id)}
                      >
                        Open
                      </button>
                      {(d.status === 'queued' || d.status === 'in_progress') && (
                        <button
                          type="button"
                          className="text-error-500"
                          onClick={() => cancel.mutate(d.id)}
                        >
                          Cancel
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
                {!deps.data?.deployments?.length && (
                  <tr>
                    <td colSpan={5} className="px-4 py-8 text-center text-gray-500">
                      No deployments yet.
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </TabPanel>
      )}

      {tab === 'webhooks' && (
        <TabPanel>
          <div className="panel-card space-y-4 p-5">
            <div>
              <div className="text-xs text-gray-500 dark:text-gray-400">Git webhook URL</div>
              <code className="mt-1 block break-all font-mono text-sm text-gray-900 dark:text-white">
                {webhookUrl}
              </code>
            </div>
            <p className="text-sm text-gray-500 dark:text-gray-400">
              Point GitHub/GitLab webhook here. Generate a secret and configure the same value on your
              provider (HMAC).
            </p>
            <Btn primary onClick={() => webhook.mutate()}>
              {webhook.isPending ? 'Generating…' : 'Generate webhook secret'}
            </Btn>
            {webhookSecret && (
              <div className="rounded-lg border border-amber-200 bg-amber-50 p-3 dark:border-amber-500/30 dark:bg-amber-500/10">
                <p className="text-xs font-medium text-amber-800 dark:text-amber-200">
                  Copy now — shown once
                </p>
                <code className="mt-1 block break-all font-mono text-sm">{webhookSecret}</code>
              </div>
            )}
            {webhook.error && <p className="text-sm text-error-500">{webhook.error.message}</p>}
          </div>
        </TabPanel>
      )}

      {tab === 'tasks' && (
        <TabPanel>
          <div className="space-y-4">
            <p className="text-sm text-gray-500 dark:text-gray-400">
              Cron schedules run every minute from the Goolify server. Commands execute via{' '}
              <code className="font-mono text-xs">docker exec</code> in the application container.
            </p>
            <div className="panel-card overflow-hidden">
              <table className="w-full text-left text-sm">
                <thead className="bg-gray-50 text-gray-500 dark:bg-white/5 dark:text-gray-400">
                  <tr>
                    <th className="px-3 py-2">Name</th>
                    <th className="px-3 py-2">Command</th>
                    <th className="px-3 py-2">Frequency</th>
                    <th className="px-3 py-2">Enabled</th>
                  </tr>
                </thead>
                <tbody>
                  {(tasks.data?.scheduled_tasks || []).map((t) => (
                    <tr key={t.id} className="border-t border-gray-200 dark:border-gray-800">
                      <td className="px-3 py-2">{t.name}</td>
                      <td className="px-3 py-2 font-mono text-xs">{t.command}</td>
                      <td className="px-3 py-2 font-mono text-xs">{t.frequency}</td>
                      <td className="px-3 py-2">{t.enabled ? 'yes' : 'no'}</td>
                    </tr>
                  ))}
                  {!tasks.data?.scheduled_tasks?.length && (
                    <tr>
                      <td colSpan={4} className="px-4 py-8 text-center text-gray-500">
                        No scheduled tasks yet.
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
            <form
              className="panel-card grid gap-3 p-4 sm:grid-cols-2"
              onSubmit={(e) => {
                e.preventDefault()
                createTask.mutate()
              }}
            >
              <label className="block text-sm">
                <span className="mb-1 block text-gray-500 dark:text-gray-400">Name</span>
                <input
                  value={taskName}
                  onChange={(e) => setTaskName(e.target.value)}
                  required
                  className="w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm dark:border-gray-800 dark:bg-gray-900"
                />
              </label>
              <label className="block text-sm">
                <span className="mb-1 block text-gray-500 dark:text-gray-400">Cron frequency</span>
                <input
                  value={taskFrequency}
                  onChange={(e) => setTaskFrequency(e.target.value)}
                  required
                  className="w-full rounded-lg border border-gray-200 bg-white px-3 py-2 font-mono text-sm dark:border-gray-800 dark:bg-gray-900"
                />
              </label>
              <label className="block text-sm sm:col-span-2">
                <span className="mb-1 block text-gray-500 dark:text-gray-400">Command</span>
                <input
                  value={taskCommand}
                  onChange={(e) => setTaskCommand(e.target.value)}
                  required
                  placeholder="php artisan schedule:run"
                  className="w-full rounded-lg border border-gray-200 bg-white px-3 py-2 font-mono text-sm dark:border-gray-800 dark:bg-gray-900"
                />
              </label>
              {createTask.error && (
                <p className="text-sm text-error-500 sm:col-span-2">{createTask.error.message}</p>
              )}
              <div className="sm:col-span-2">
                <Btn primary type="submit">
                  {createTask.isPending ? 'Saving…' : 'Add task'}
                </Btn>
              </div>
            </form>
          </div>
        </TabPanel>
      )}

      {tab === 'rollback' && (
        <TabPanel>
          <div className="panel-card space-y-4 p-5">
            <p className="text-sm text-gray-600 dark:text-gray-300">
              Redeploy the last finished commit (or force an image rebuild). This queues a new
              deployment.
            </p>
            <Btn primary onClick={() => rollback.mutate()}>
              {rollback.isPending ? 'Queuing…' : 'Rollback / redeploy'}
            </Btn>
            {rollback.error && <p className="text-sm text-error-500">{rollback.error.message}</p>}
          </div>
        </TabPanel>
      )}

      {tab === 'danger' && (
        <TabPanel>
          <div className="space-y-4">
            <div className="panel-card space-y-4 border-error-200 p-5 dark:border-error-500/30">
              <h2 className="text-sm font-semibold text-error-500">Force rebuild</h2>
              <p className="text-sm text-gray-500 dark:text-gray-400">
                Queue a deployment with force rebuild enabled.
              </p>
              <Btn onClick={() => deploy.mutate({ force: true })}>Force rebuild deploy</Btn>
              {deploy.error && <p className="text-sm text-error-500">{deploy.error.message}</p>}
            </div>
            <div className="panel-card space-y-4 border-error-200 p-5 dark:border-error-500/30">
              <h2 className="text-sm font-semibold text-error-500">Delete application</h2>
              <p className="text-sm text-gray-500 dark:text-gray-400">
                Removes the application from Goolify and best-effort deletes its container on the
                server. This cannot be undone.
              </p>
              <Btn
                onClick={() => {
                  if (confirm(`Delete application ${a.name}? This cannot be undone.`)) {
                    remove.mutate()
                  }
                }}
              >
                {remove.isPending ? 'Deleting…' : 'Delete permanently'}
              </Btn>
              {remove.error && <p className="text-sm text-error-500">{remove.error.message}</p>}
            </div>
          </div>
        </TabPanel>
      )}
    </div>
  )
}
