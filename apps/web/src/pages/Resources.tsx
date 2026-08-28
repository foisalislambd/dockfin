import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import { FolderKanban } from 'lucide-react'
import { useMemo, useState } from 'react'
import { PageSkeleton } from '../components/ui/Skeleton'
import { api, fetchAllEnvironments, LAST_ENV_KEY } from '../lib/api'
import { Btn, Header, Input, Modal } from './Servers'

export function ProjectsPage() {
  const qc = useQueryClient()
  const nav = useNavigate()
  const projects = useQuery({ queryKey: ['projects'], queryFn: api.projects })
  const envs = useQuery({ queryKey: ['all-environments'], queryFn: fetchAllEnvironments })
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
      // Land in the new production environment resources
      void nav({
        to: '/projects/$projectId/environments/$envId',
        params: { projectId: data.project.id, envId: data.environment.id },
      })
    },
  })

  const envsByProject = useMemo(() => {
    const m = new Map<string, string[]>()
    for (const e of envs.data || []) {
      const list = m.get(e.project_id) || []
      list.push(e.id)
      m.set(e.project_id, list)
    }
    return m
  }, [envs.data])

  if (projects.isLoading) return <PageSkeleton cards={2} />

  return (
    <div className="space-y-6">
      <Header
        title="Projects"
        subtitle="Group applications, databases, and services by environment."
        actions={
          <Btn primary onClick={() => setShow(true)}>
            New project
          </Btn>
        }
      />

      {(projects.data?.projects || []).length > 0 ? (
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 sm:gap-4">
          {(projects.data?.projects || []).map((p) => {
            const envIds = envsByProject.get(p.id) || []
            // Single env → resources; else environments list
            const singleEnvId = envIds.length === 1 ? envIds[0] : undefined
            const projectHref = singleEnvId
              ? ({
                  to: '/projects/$projectId/environments/$envId' as const,
                  params: { projectId: p.id, envId: singleEnvId },
                })
              : ({
                  to: '/projects/$projectId' as const,
                  params: { projectId: p.id },
                })
            return (
              <div
                key={p.id}
                className="panel-card group flex min-h-[5.5rem] items-center gap-4 px-5 py-4 transition hover:border-brand-300 dark:hover:border-brand-500/40"
              >
                <Link
                  to={projectHref.to}
                  params={projectHref.params}
                  className="flex min-w-0 flex-1 items-center gap-4"
                  onClick={() => {
                    if (singleEnvId) localStorage.setItem(LAST_ENV_KEY, singleEnvId)
                  }}
                >
                  <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-brand-50 text-brand-600 dark:bg-brand-500/15 dark:text-brand-400">
                    <FolderKanban className="h-5 w-5" strokeWidth={1.75} />
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="truncate text-base font-semibold text-gray-900 dark:text-white">
                      {p.name}
                    </div>
                    {p.description ? (
                      <div className="mt-0.5 truncate text-sm text-gray-500 dark:text-gray-400">
                        {p.description}
                      </div>
                    ) : !envs.isLoading && !envs.isError ? (
                      <div className="mt-0.5 text-sm text-gray-400 dark:text-gray-500">
                        {envIds.length === 1
                          ? '1 environment'
                          : `${envIds.length} environments`}
                      </div>
                    ) : null}
                  </div>
                </Link>
                <div className="flex shrink-0 items-center gap-3">
                  {singleEnvId ? (
                    <Link
                      to="/projects/$projectId/environments/$envId/new"
                      params={{ projectId: p.id, envId: singleEnvId }}
                      className="hidden text-sm font-medium text-brand-600 sm:inline dark:text-brand-400"
                      onClick={() => localStorage.setItem(LAST_ENV_KEY, singleEnvId)}
                    >
                      Add resource
                    </Link>
                  ) : null}
                  <Link
                    to="/projects/$projectId/edit"
                    params={{ projectId: p.id }}
                    className="hidden text-sm text-gray-400 hover:text-gray-700 sm:inline dark:hover:text-gray-200"
                  >
                    Settings
                  </Link>
                </div>
              </div>
            )
          })}
        </div>
      ) : (
        <div className="panel-card p-10 text-center">
          <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-2xl bg-brand-50 text-brand-600 dark:bg-brand-500/15 dark:text-brand-400">
            <FolderKanban className="h-6 w-6" strokeWidth={1.75} />
          </div>
          <p className="mt-4 text-sm font-medium text-gray-900 dark:text-white">No projects yet</p>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            Create a project to deploy apps, databases, and services.
          </p>
          <div className="mt-4">
            <Btn primary onClick={() => setShow(true)}>
              New project
            </Btn>
          </div>
        </div>
      )}

      {show && (
        <Modal title="New project" onClose={() => setShow(false)}>
          <form
            className="space-y-3"
            onSubmit={(e) => {
              e.preventDefault()
              create.mutate()
            }}
          >
            <Input label="Name" value={name} onChange={setName} required />
            <Input label="Description" value={description} onChange={setDescription} />
            {create.isError ? (
              <p className="text-sm text-error-500">{(create.error as Error).message}</p>
            ) : null}
            <div className="flex justify-end gap-2 pt-2">
              <Btn type="button" onClick={() => setShow(false)}>
                Cancel
              </Btn>
              <Btn primary type="submit" disabled={create.isPending || !name.trim()}>
                {create.isPending ? 'Creating…' : 'Create'}
              </Btn>
            </div>
          </form>
        </Modal>
      )}
    </div>
  )
}
