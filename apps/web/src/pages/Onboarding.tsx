import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import { useMemo, useState } from 'react'
import { api, LAST_ENV_KEY } from '../lib/api'
import { Btn, Header, Input } from './Servers'

const steps = [
  'Add SSH key',
  'Add server',
  'Validate',
  'Start proxy',
  'Create project',
] as const

export function OnboardingPage() {
  const qc = useQueryClient()
  const navigate = useNavigate()
  const keys = useQuery({ queryKey: ['keys'], queryFn: api.keys })
  const servers = useQuery({ queryKey: ['servers'], queryFn: api.servers })
  const projects = useQuery({ queryKey: ['projects'], queryFn: api.projects })

  const [step, setStep] = useState(0)
  const [keyName, setKeyName] = useState('')
  const [privateKey, setPrivateKey] = useState('')
  const [serverForm, setServerForm] = useState({
    name: '',
    ip: '',
    port: 22,
    user_name: 'root',
    private_key_id: '',
  })
  const [selectedServer, setSelectedServer] = useState('')
  const [projectName, setProjectName] = useState('')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')

  const firstUsable = useMemo(
    () => (servers.data?.servers || []).find((s) => s.is_usable) || servers.data?.servers?.[0],
    [servers.data],
  )

  const createKey = useMutation({
    mutationFn: () => api.createKey(keyName, privateKey),
    onSuccess: (key) => {
      void qc.invalidateQueries({ queryKey: ['keys'] })
      setServerForm((f) => ({ ...f, private_key_id: key.id }))
      setMessage('SSH key saved.')
      setError('')
      setStep(1)
    },
    onError: (e: Error) => setError(e.message),
  })

  const createServer = useMutation({
    mutationFn: () =>
      api.createServer({
        name: serverForm.name,
        ip: serverForm.ip,
        port: serverForm.port,
        user_name: serverForm.user_name,
        private_key_id: serverForm.private_key_id || undefined,
      }),
    onSuccess: (server) => {
      void qc.invalidateQueries({ queryKey: ['servers'] })
      setSelectedServer(server.id)
      setMessage('Server created.')
      setError('')
      setStep(2)
    },
    onError: (e: Error) => setError(e.message),
  })

  const validate = useMutation({
    mutationFn: () => api.validateServer(selectedServer || firstUsable?.id || ''),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['servers'] })
      setMessage('Server validated.')
      setError('')
      setStep(3)
    },
    onError: (e: Error) => setError(e.message),
  })

  const startProxy = useMutation({
    mutationFn: () => api.startProxy(selectedServer || firstUsable?.id || ''),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['servers'] })
      setMessage('Proxy started.')
      setError('')
      setStep(4)
    },
    onError: (e: Error) => setError(e.message),
  })

  const createProject = useMutation({
    mutationFn: () => api.createProject(projectName),
    onSuccess: (data) => {
      void qc.invalidateQueries({ queryKey: ['projects'] })
      localStorage.setItem(LAST_ENV_KEY, data.environment.id)
      setMessage('Project created. Production environment saved.')
      setError('')
      void navigate({
        to: '/projects/$projectId/environments/$envId',
        params: { projectId: data.project.id, envId: data.environment.id },
      })
    },
    onError: (e: Error) => setError(e.message),
  })

  return (
    <div className="space-y-8">
      <Header
        title="Onboarding"
        actions={
          <Link to="/dashboard" className="text-sm text-gray-500 dark:text-gray-400 hover:text-brand-600 dark:text-brand-400">
            Skip to dashboard
          </Link>
        }
      />

      <ol className="flex flex-wrap gap-2">
        {steps.map((label, i) => (
          <li key={label}>
            <button
              type="button"
              onClick={() => {
                setStep(i)
                setError('')
              }}
              className={`rounded-lg px-3 py-1.5 text-sm transition ${
                i === step
                  ? 'bg-brand-500 text-white'
                  : i < step
                    ? 'border border-brand-500/40 text-brand-600 dark:text-brand-400'
                    : 'border border-gray-200 dark:border-gray-800 text-gray-500 dark:text-gray-400'
              }`}
            >
              {i + 1}. {label}
            </button>
          </li>
        ))}
      </ol>

      {message && <p className="text-sm text-brand-600 dark:text-brand-400">{message}</p>}
      {error && <p className="text-sm text-error-500">{error}</p>}

      <div className="rounded-xl border border-gray-200 dark:border-gray-800 panel-card bg-white dark:bg-white/3/60 p-6">
        {step === 0 && (
          <form
            className="max-w-lg space-y-3"
            onSubmit={(e) => {
              e.preventDefault()
              createKey.mutate()
            }}
          >
            <p className="text-sm text-gray-500 dark:text-gray-400">
              Upload an SSH private key used to reach your Docker hosts.
              {keys.data?.private_keys?.length
                ? ` You already have ${keys.data.private_keys.length} key(s).`
                : ''}
            </p>
            <Input label="Key name" value={keyName} onChange={setKeyName} />
            <label className="block text-sm">
              <span className="mb-1 block text-gray-500 dark:text-gray-400">Private key (PEM)</span>
              <textarea
                required
                rows={6}
                value={privateKey}
                onChange={(e) => setPrivateKey(e.target.value)}
                className="panel-field w-full rounded-lg px-3 py-2 font-mono text-xs"
              />
            </label>
            <div className="flex gap-2">
              <Btn primary type="submit">
                Save key
              </Btn>
              {!!keys.data?.private_keys?.length && (
                <Btn
                  onClick={() => {
                    setServerForm((f) => ({
                      ...f,
                      private_key_id: keys.data!.private_keys[0].id,
                    }))
                    setStep(1)
                  }}
                >
                  Use existing →
                </Btn>
              )}
            </div>
          </form>
        )}

        {step === 1 && (
          <form
            className="max-w-lg space-y-3"
            onSubmit={(e) => {
              e.preventDefault()
              createServer.mutate()
            }}
          >
            <p className="text-sm text-gray-500 dark:text-gray-400">
              Add a server reachable over SSH with Docker installed.
            </p>
            <Input
              label="Name"
              value={serverForm.name}
              onChange={(v) => setServerForm({ ...serverForm, name: v })}
            />
            <Input
              label="IP / hostname"
              value={serverForm.ip}
              onChange={(v) => setServerForm({ ...serverForm, ip: v })}
            />
            <Input
              label="SSH user"
              value={serverForm.user_name}
              onChange={(v) => setServerForm({ ...serverForm, user_name: v })}
            />
            <label className="block text-sm">
              <span className="mb-1 block text-gray-500 dark:text-gray-400">Port</span>
              <input
                type="number"
                value={serverForm.port}
                onChange={(e) => setServerForm({ ...serverForm, port: Number(e.target.value) })}
                className="panel-field w-full rounded-lg px-3 py-2"
              />
            </label>
            <label className="block text-sm">
              <span className="mb-1 block text-gray-500 dark:text-gray-400">SSH key</span>
              <select
                value={serverForm.private_key_id}
                onChange={(e) => setServerForm({ ...serverForm, private_key_id: e.target.value })}
                className="panel-field w-full rounded-lg px-3 py-2"
              >
                <option value="">None</option>
                {(keys.data?.private_keys || []).map((k) => (
                  <option key={k.id} value={k.id}>
                    {k.name}
                  </option>
                ))}
              </select>
            </label>
            <div className="flex gap-2">
              <Btn primary type="submit">
                Add server
              </Btn>
              {!!servers.data?.servers?.length && (
                <Btn
                  onClick={() => {
                    setSelectedServer(servers.data!.servers[0].id)
                    setStep(2)
                  }}
                >
                  Use existing →
                </Btn>
              )}
            </div>
          </form>
        )}

        {step === 2 && (
          <div className="max-w-lg space-y-3">
            <p className="text-sm text-gray-500 dark:text-gray-400">
              Validate SSH connectivity and Docker on the host.
            </p>
            <label className="block text-sm">
              <span className="mb-1 block text-gray-500 dark:text-gray-400">Server</span>
              <select
                value={selectedServer || firstUsable?.id || ''}
                onChange={(e) => setSelectedServer(e.target.value)}
                className="panel-field w-full rounded-lg px-3 py-2"
              >
                {(servers.data?.servers || []).map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.name} ({s.ip})
                  </option>
                ))}
              </select>
            </label>
            <Btn
              primary
              onClick={() => {
                if (!selectedServer && firstUsable) setSelectedServer(firstUsable.id)
                validate.mutate()
              }}
            >
              Validate
            </Btn>
          </div>
        )}

        {step === 3 && (
          <div className="max-w-lg space-y-3">
            <p className="text-sm text-gray-500 dark:text-gray-400">
              Start the Traefik reverse proxy on the selected server.
            </p>
            <label className="block text-sm">
              <span className="mb-1 block text-gray-500 dark:text-gray-400">Server</span>
              <select
                value={selectedServer || firstUsable?.id || ''}
                onChange={(e) => setSelectedServer(e.target.value)}
                className="panel-field w-full rounded-lg px-3 py-2"
              >
                {(servers.data?.servers || []).map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.name} — proxy: {s.proxy_status}
                  </option>
                ))}
              </select>
            </label>
            <Btn primary onClick={() => startProxy.mutate()}>
              Start proxy
            </Btn>
          </div>
        )}

        {step === 4 && (
          <form
            className="max-w-lg space-y-3"
            onSubmit={(e) => {
              e.preventDefault()
              createProject.mutate()
            }}
          >
            <p className="text-sm text-gray-500 dark:text-gray-400">
              Create a project (includes a production environment). You have{' '}
              {projects.data?.projects?.length ?? 0} project(s) already.
            </p>
            <Input label="Project name" value={projectName} onChange={setProjectName} />
            <Btn primary type="submit">
              Create project
            </Btn>
          </form>
        )}
      </div>
    </div>
  )
}
