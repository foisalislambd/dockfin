import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useParams } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
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

  // Coolify navigateTo(): single environment → resources directly
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
        <Link to="/projects" className="text-brand-600 dark:text-brand-400">
          ← Projects
        </Link>
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
        <Link
          to="/projects"
          className="text-sm text-gray-500 hover:text-brand-600 dark:text-gray-400 dark:hover:text-brand-400"
        >
          ← Projects
        </Link>
        <Header
          title="Environments"
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
                + Add
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
            className="panel-card group relative flex items-center justify-between gap-4 p-5 transition hover:border-brand-300 dark:hover:border-brand-500/40"
            onClick={() => localStorage.setItem(LAST_ENV_KEY, env.id)}
          >
            <div className="min-w-0">
              <div className="font-semibold text-gray-900 dark:text-white">{env.name}</div>
              {env.description && (
                <div className="mt-1 truncate text-sm text-gray-500 dark:text-gray-400">
                  {env.description}
                </div>
              )}
            </div>
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
