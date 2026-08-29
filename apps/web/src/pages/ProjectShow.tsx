import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useParams } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { Layers } from 'lucide-react'
import { BackLink } from '../components/BackLink'
import { PageSkeleton } from '../components/ui/Skeleton'
import { api, LAST_ENV_KEY } from '../lib/api'
import { Btn, Header, Input, Modal } from './Servers'

export function ProjectShowPage() {
  const { projectId } = useParams({ strict: false }) as { projectId: string }
  const qc = useQueryClient()
  const nav = useNavigate()
  const project = useQuery({
    queryKey: ['project', projectId],
    queryFn: () => api.getProject(projectId),
  })
  const envs = useQuery({
    queryKey: ['environments', projectId],
    queryFn: () => api.environments(projectId),
  })
  const [show, setShow] = useState(false)
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')

  // Single environment → resources directly
  useEffect(() => {
    const list = envs.data?.environments
    if (!list || list.length !== 1) return
    const env = list[0]
    localStorage.setItem(LAST_ENV_KEY, env.id)
    void nav({
      to: '/projects/$projectId/environments/$envId',
      params: { projectId, envId: env.id },
      replace: true,
    })
  }, [envs.data, projectId, nav])

  const create = useMutation({
    mutationFn: () => api.createEnvironment(projectId, name, description),
    onSuccess: (env) => {
      localStorage.setItem(LAST_ENV_KEY, env.id)
      void qc.invalidateQueries({ queryKey: ['environments', projectId] })
      void qc.invalidateQueries({ queryKey: ['all-environments'] })
      setShow(false)
      setName('')
      setDescription('')
      void nav({
        to: '/projects/$projectId/environments/$envId',
        params: { projectId, envId: env.id },
      })
    },
  })

  if (project.isLoading || envs.isLoading) {
    return <PageSkeleton cards={1} />
  }
  if (project.error || !project.data) {
    return (
      <div className="space-y-4">
        <p className="text-error-500">{project.error?.message || 'Project not found'}</p>
        <BackLink label="Projects" to="/projects" />
      </div>
    )
  }

  // While redirecting single-env projects, keep skeleton
  if ((envs.data?.environments || []).length === 1) {
    return <PageSkeleton cards={1} />
  }

  return (
    <div className="space-y-6">
      <div>
        <BackLink label="Projects" to="/projects" />
        <Header
          title={project.data.name}
          subtitle="Choose an environment to deploy and manage resources."
          actions={
            <div className="flex items-center gap-2">
              <Link
                to="/projects/$projectId/edit"
                params={{ projectId }}
                className="inline-flex h-8 items-center rounded-md border border-gray-200 px-2.5 text-xs font-medium text-gray-600 hover:bg-gray-50 dark:border-gray-800 dark:text-gray-300 dark:hover:bg-white/5"
              >
                Settings
              </Link>
              <Btn primary onClick={() => setShow(true)}>
                Add environment
              </Btn>
            </div>
          }
        />
      </div>

      <div className="grid gap-3 lg:grid-cols-2">
        {(envs.data?.environments || []).map((env) => (
          <Link
            key={env.id}
            to="/projects/$projectId/environments/$envId"
            params={{ projectId, envId: env.id }}
            className="panel-card group flex items-center gap-4 p-5 transition hover:border-brand-300 dark:hover:border-brand-500/40"
            onClick={() => localStorage.setItem(LAST_ENV_KEY, env.id)}
          >
            <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-gray-50 text-gray-500 dark:bg-white/5 dark:text-gray-400">
              <Layers className="h-5 w-5" strokeWidth={1.75} />
            </div>
            <div className="min-w-0 flex-1">
              <div className="font-semibold text-gray-900 dark:text-white">{env.name}</div>
              {env.description && (
                <div className="mt-1 truncate text-sm text-gray-500 dark:text-gray-400">
                  {env.description}
                </div>
              )}
            </div>
            <span className="text-xs font-medium text-gray-400 group-hover:text-brand-600 dark:group-hover:text-brand-400">
              Open
            </span>
          </Link>
        ))}
        {!envs.data?.environments?.length && (
          <div className="panel-card col-span-full p-8 text-center text-sm text-gray-500 dark:text-gray-400">
            No environments found.
          </div>
        )}
      </div>

      {show && (
        <Modal title="New Environment" onClose={() => setShow(false)}>
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
              Save
            </Btn>
          </form>
        </Modal>
      )}
    </div>
  )
}
