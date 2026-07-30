import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useParams } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { PageSkeleton } from '../components/ui/Skeleton'
import { api } from '../lib/api'
import { Btn, Header, Input, Modal } from './Servers'

export function ProjectEditPage() {
  const { projectId } = useParams({ strict: false }) as { projectId: string }
  const qc = useQueryClient()
  const nav = useNavigate()
  const project = useQuery({
    queryKey: ['project', projectId],
    queryFn: () => api.getProject(projectId),
  })

  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [confirmName, setConfirmName] = useState('')

  useEffect(() => {
    if (!project.data) return
    setName(project.data.name)
    setDescription(project.data.description || '')
  }, [project.data])

  const save = useMutation({
    mutationFn: () => api.updateProject(projectId, name, description),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['project', projectId] })
      void qc.invalidateQueries({ queryKey: ['projects'] })
    },
  })

  const remove = useMutation({
    mutationFn: () => api.deleteProject(projectId),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['projects'] })
      void qc.invalidateQueries({ queryKey: ['all-environments'] })
      void nav({ to: '/projects' })
    },
  })

  if (project.isLoading) return <PageSkeleton cards={1} />
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

  const isEmpty = project.data.is_empty === true
  const nameMatches = confirmName.trim() === project.data.name

  return (
    <div className="space-y-6">
      <div>
        <Link
          to="/projects"
          className="text-sm text-gray-500 hover:text-brand-600 dark:text-gray-400 dark:hover:text-brand-400"
        >
          ← Projects
        </Link>
        <Header title="Project settings" />
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
          Edit name and description, or delete this project when it has no resources.
        </p>
      </div>

      <form
        className="panel-card max-w-xl space-y-4 p-5"
        onSubmit={(e) => {
          e.preventDefault()
          save.mutate()
        }}
      >
        <Input label="Name" value={name} onChange={setName} />
        <Input label="Description" value={description} onChange={setDescription} required={false} />
        {save.error && <p className="text-sm text-error-500">{save.error.message}</p>}
        {save.isSuccess && <p className="text-sm text-success-600 dark:text-success-400">Saved.</p>}
        <div className="flex flex-wrap gap-2">
          <Btn primary type="submit" disabled={save.isPending || !name.trim()}>
            {save.isPending ? 'Saving…' : 'Save'}
          </Btn>
          <Btn
            type="button"
            disabled={!isEmpty}
            onClick={() => {
              setConfirmName('')
              setConfirmOpen(true)
            }}
          >
            Delete Project
          </Btn>
        </div>
        {!isEmpty && (
          <p className="text-sm text-amber-600 dark:text-amber-400">
            Delete is disabled while this project has applications, databases, or services. Remove
            those resources first.
          </p>
        )}
      </form>

      {confirmOpen && (
        <Modal title="Delete Project" onClose={() => setConfirmOpen(false)}>
          <div className="space-y-3">
            <p className="text-sm text-gray-600 dark:text-gray-300">
              This permanently deletes the project and its empty environments. Type{' '}
              <span className="font-semibold text-gray-900 dark:text-white">{project.data.name}</span>{' '}
              to confirm.
            </p>
            <Input label="Project name" value={confirmName} onChange={setConfirmName} />
            {remove.error && <p className="text-sm text-error-500">{remove.error.message}</p>}
            <div className="flex gap-2">
              <Btn
                type="button"
                disabled={!nameMatches || remove.isPending}
                onClick={() => remove.mutate()}
              >
                {remove.isPending ? 'Deleting…' : 'Delete permanently'}
              </Btn>
              <Btn type="button" onClick={() => setConfirmOpen(false)}>
                Cancel
              </Btn>
            </div>
          </div>
        </Modal>
      )}
    </div>
  )
}

export function EnvironmentEditPage() {
  const { projectId, envId } = useParams({ strict: false }) as {
    projectId: string
    envId: string
  }
  const qc = useQueryClient()
  const nav = useNavigate()
  const env = useQuery({
    queryKey: ['environment', projectId, envId],
    queryFn: () => api.getEnvironment(projectId, envId),
  })
  const project = useQuery({
    queryKey: ['project', projectId],
    queryFn: () => api.getProject(projectId),
  })

  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [confirmName, setConfirmName] = useState('')

  useEffect(() => {
    if (!env.data) return
    setName(env.data.name)
    setDescription(env.data.description || '')
  }, [env.data])

  const save = useMutation({
    mutationFn: () => api.updateEnvironment(projectId, envId, name, description),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['environment', projectId, envId] })
      void qc.invalidateQueries({ queryKey: ['environments', projectId] })
      void qc.invalidateQueries({ queryKey: ['all-environments'] })
    },
  })

  const remove = useMutation({
    mutationFn: () => api.deleteEnvironment(projectId, envId),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['environments', projectId] })
      void qc.invalidateQueries({ queryKey: ['all-environments'] })
      void qc.invalidateQueries({ queryKey: ['project', projectId] })
      void nav({ to: '/projects/$projectId', params: { projectId } })
    },
  })

  if (env.isLoading) return <PageSkeleton cards={1} />
  if (env.error || !env.data) {
    return (
      <div className="space-y-4">
        <p className="text-error-500">{env.error?.message || 'Environment not found'}</p>
        <Link
          to="/projects/$projectId/environments/$envId"
          params={{ projectId, envId }}
          className="text-brand-600 dark:text-brand-400"
        >
          ← Resources
        </Link>
      </div>
    )
  }

  const isEmpty = env.data.is_empty === true
  const nameMatches = confirmName.trim() === env.data.name

  return (
    <div className="space-y-6">
      <div>
        <nav className="flex flex-wrap items-center gap-1 text-sm text-gray-500 dark:text-gray-400">
          <Link to="/projects" className="hover:text-brand-600 dark:hover:text-brand-400">
            Projects
          </Link>
          <span>/</span>
          <Link
            to="/projects/$projectId"
            params={{ projectId }}
            className="hover:text-brand-600 dark:hover:text-brand-400"
          >
            {project.data?.name || '…'}
          </Link>
          <span>/</span>
          <Link
            to="/projects/$projectId/environments/$envId"
            params={{ projectId, envId }}
            className="hover:text-brand-600 dark:hover:text-brand-400"
          >
            {env.data.name}
          </Link>
          <span>/</span>
          <span className="text-gray-900 dark:text-white">Settings</span>
        </nav>
        <Header title="Environment settings" />
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
          Edit this environment, or delete it when it has no resources.
        </p>
      </div>

      <form
        className="panel-card max-w-xl space-y-4 p-5"
        onSubmit={(e) => {
          e.preventDefault()
          save.mutate()
        }}
      >
        <Input label="Name" value={name} onChange={setName} />
        <Input label="Description" value={description} onChange={setDescription} required={false} />
        {save.error && <p className="text-sm text-error-500">{save.error.message}</p>}
        {save.isSuccess && <p className="text-sm text-success-600 dark:text-success-400">Saved.</p>}
        <div className="flex flex-wrap gap-2">
          <Btn primary type="submit" disabled={save.isPending || !name.trim()}>
            {save.isPending ? 'Saving…' : 'Save'}
          </Btn>
          <Btn
            type="button"
            disabled={!isEmpty}
            onClick={() => {
              setConfirmName('')
              setConfirmOpen(true)
            }}
          >
            Delete Environment
          </Btn>
        </div>
        {!isEmpty && (
          <p className="text-sm text-amber-600 dark:text-amber-400">
            Delete is disabled while this environment has applications, databases, or services.
            Remove those resources first.
          </p>
        )}
      </form>

      {confirmOpen && (
        <Modal title="Delete Environment" onClose={() => setConfirmOpen(false)}>
          <div className="space-y-3">
            <p className="text-sm text-gray-600 dark:text-gray-300">
              This permanently deletes the environment. Type{' '}
              <span className="font-semibold text-gray-900 dark:text-white">{env.data.name}</span> to
              confirm.
            </p>
            <Input label="Environment name" value={confirmName} onChange={setConfirmName} />
            {remove.error && <p className="text-sm text-error-500">{remove.error.message}</p>}
            <div className="flex gap-2">
              <Btn
                type="button"
                disabled={!nameMatches || remove.isPending}
                onClick={() => remove.mutate()}
              >
                {remove.isPending ? 'Deleting…' : 'Delete permanently'}
              </Btn>
              <Btn type="button" onClick={() => setConfirmOpen(false)}>
                Cancel
              </Btn>
            </div>
          </div>
        </Modal>
      )}
    </div>
  )
}
