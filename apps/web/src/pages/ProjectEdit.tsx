import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useParams } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { DangerConfirmModal, DangerZoneCard } from '../components/DangerConfirmModal'
import { FormPageSkeleton } from '../components/ui/Skeleton'
import { api } from '../lib/api'
import { Btn, Header, Input } from './Servers'

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
  const [deleteOpen, setDeleteOpen] = useState(false)

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
    mutationFn: (body: Parameters<typeof api.deleteProject>[1]) => api.deleteProject(projectId, body),
    onSuccess: () => {
      setDeleteOpen(false)
      void qc.invalidateQueries({ queryKey: ['projects'] })
      void qc.invalidateQueries({ queryKey: ['all-environments'] })
      void nav({ to: '/projects' })
    },
  })

  if (project.isLoading) return <FormPageSkeleton />
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

  return (
    <div className="space-y-6">
      <div>
        <Link
          to="/projects"
          className="text-sm text-gray-500 hover:text-brand-600 dark:text-gray-400 dark:hover:text-brand-400"
        >
          ← Projects
        </Link>
        <Header title={project.data.name} />
      </div>

      <form
        className="panel-card max-w-xl space-y-4 p-5"
        onSubmit={(e) => {
          e.preventDefault()
          save.mutate()
        }}
      >
        <div className="flex flex-wrap gap-2">
          <Btn primary type="submit" disabled={save.isPending || !name.trim()}>
            {save.isPending ? 'Saving…' : 'Save'}
          </Btn>
          <Btn type="button" disabled={!isEmpty} onClick={() => setDeleteOpen(true)}>
            Delete Project
          </Btn>
        </div>
        <Input label="Name" value={name} onChange={setName} />
        <Input label="Description" value={description} onChange={setDescription} required={false} />
        {save.error && <p className="text-sm text-error-500">{save.error.message}</p>}
        {save.isSuccess && <p className="text-sm text-success-600 dark:text-success-400">Saved.</p>}
      </form>

      <DangerZoneCard>
        <div>
          <h3 className="text-sm font-semibold text-error-500">Delete Project</h3>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            Deletes this project and all of its empty environments. Delete is only allowed when the
            project has no applications, databases, or services left.
          </p>
          {!isEmpty ? (
            <p className="mt-2 text-sm text-amber-600 dark:text-amber-400">
              This project still has resources. Open each resource → Danger Zone → Delete, then
              return here.
            </p>
          ) : (
            <p className="mt-2 text-sm text-success-600 dark:text-success-400">
              Project is empty — safe to delete.
            </p>
          )}
        </div>
        <Btn type="button" disabled={!isEmpty} onClick={() => setDeleteOpen(true)}>
          Delete
        </Btn>
      </DangerZoneCard>

      <DangerConfirmModal
        open={deleteOpen}
        onClose={() => setDeleteOpen(false)}
        title="Confirm Project Deletion?"
        resourceLabel="Project Name"
        expectedName={project.data.name}
        statusLine={
          isEmpty
            ? 'Project has no resources. Environments inside will be deleted with it.'
            : 'Project still has resources — delete is blocked until they are removed.'
        }
        actions={[
          'Permanently delete this project.',
          'Delete all empty environments belonging to this project.',
          'This cannot be undone.',
        ]}
        requirePassword={false}
        confirmButtonLabel="Permanently Delete"
        busy={remove.isPending}
        error={remove.error?.message}
        onConfirm={(payload) => remove.mutate(payload)}
      />
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
  const [deleteOpen, setDeleteOpen] = useState(false)

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
    mutationFn: (body: Parameters<typeof api.deleteEnvironment>[2]) =>
      api.deleteEnvironment(projectId, envId, body),
    onSuccess: () => {
      setDeleteOpen(false)
      void qc.invalidateQueries({ queryKey: ['environments', projectId] })
      void qc.invalidateQueries({ queryKey: ['all-environments'] })
      void qc.invalidateQueries({ queryKey: ['project', projectId] })
      void nav({ to: '/projects/$projectId', params: { projectId } })
    },
  })

  if (env.isLoading) return <FormPageSkeleton />
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
      </div>

      <form
        className="panel-card max-w-xl space-y-4 p-5"
        onSubmit={(e) => {
          e.preventDefault()
          save.mutate()
        }}
      >
        <div className="flex flex-wrap gap-2">
          <Btn primary type="submit" disabled={save.isPending || !name.trim()}>
            {save.isPending ? 'Saving…' : 'Save'}
          </Btn>
          <Btn type="button" disabled={!isEmpty} onClick={() => setDeleteOpen(true)}>
            Delete Environment
          </Btn>
        </div>
        <Input label="Name" value={name} onChange={setName} />
        <Input label="Description" value={description} onChange={setDescription} required={false} />
        {save.error && <p className="text-sm text-error-500">{save.error.message}</p>}
        {save.isSuccess && <p className="text-sm text-success-600 dark:text-success-400">Saved.</p>}
      </form>

      <DangerZoneCard>
        <div>
          <h3 className="text-sm font-semibold text-error-500">Delete Environment</h3>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            Removes this environment from the project. Only allowed when it has no applications,
            databases, or services.
          </p>
          {!isEmpty ? (
            <p className="mt-2 text-sm text-amber-600 dark:text-amber-400">
              Delete is disabled while this environment still has resources. Remove them from each
              resource&apos;s Danger Zone first.
            </p>
          ) : (
            <p className="mt-2 text-sm text-success-600 dark:text-success-400">
              Environment is empty — safe to delete.
            </p>
          )}
        </div>
        <Btn type="button" disabled={!isEmpty} onClick={() => setDeleteOpen(true)}>
          Delete
        </Btn>
      </DangerZoneCard>

      <DangerConfirmModal
        open={deleteOpen}
        onClose={() => setDeleteOpen(false)}
        title="Confirm Environment Deletion?"
        resourceLabel="Environment Name"
        expectedName={env.data.name}
        statusLine={
          isEmpty
            ? 'Environment has no resources and can be deleted.'
            : 'Environment still has resources — delete is blocked.'
        }
        actions={[
          'Permanently delete this environment.',
          'The project itself will remain.',
          'This cannot be undone.',
        ]}
        requirePassword={false}
        confirmButtonLabel="Permanently Delete"
        busy={remove.isPending}
        error={remove.error?.message}
        onConfirm={(payload) => remove.mutate(payload)}
      />
    </div>
  )
}
