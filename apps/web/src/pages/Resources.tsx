import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
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
      // Coolify: land in the new production environment resources
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
        actions={
          <Btn primary onClick={() => setShow(true)}>
            + Add
          </Btn>
        }
      />

      {(projects.data?.projects || []).length > 0 ? (
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 sm:gap-4">
          {(projects.data?.projects || []).map((p) => {
            const envIds = envsByProject.get(p.id) || []
            // Coolify navigateTo(): single env → resources; else environments list
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
                className="panel-card flex min-h-[4.5rem] items-center justify-between gap-4 px-5 py-4"
              >
                <Link
                  to={projectHref.to}
                  params={projectHref.params}
                  className="min-w-0 flex-1"
                  onClick={() => {
                    if (singleEnvId) localStorage.setItem(LAST_ENV_KEY, singleEnvId)
                  }}
                >
                  <div className="truncate text-base font-semibold text-gray-900 dark:text-white">
                    {p.name}
                  </div>
                  {p.description ? (
                    <div className="mt-0.5 truncate text-sm text-gray-500 dark:text-gray-400">
                      {p.description}
                    </div>
                  ) : null}
                </Link>
                <div className="flex shrink-0 items-center gap-2">
                  {singleEnvId ? (
                    <Link
                      to="/projects/$projectId/environments/$envId/new"
                      params={{ projectId: p.id, envId: singleEnvId }}
                      className="text-sm font-medium text-brand-600 hover:underline dark:text-brand-400"
                      onClick={() => localStorage.setItem(LAST_ENV_KEY, singleEnvId)}
                    >
                      + Add Resource
                    </Link>
                  ) : (
                    <Link
                      to="/projects/$projectId"
                      params={{ projectId: p.id }}
                      className="text-sm font-medium text-brand-600 hover:underline dark:text-brand-400"
                    >
                      Environments
                    </Link>
                  )}
                  <Link
                    to="/projects/$projectId/edit"
                    params={{ projectId: p.id }}
                    className="text-sm text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200"
                  >
                    Settings
                  </Link>
                </div>
              </div>
            )
          })}
        </div>
      ) : (
        <div className="panel-card p-10 text-center text-sm text-gray-500 dark:text-gray-400">
          No projects yet. Create one to get started.
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
