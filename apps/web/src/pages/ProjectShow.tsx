import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useParams } from '@tanstack/react-router'
import { Layers, Search } from 'lucide-react'
import { useEffect, useState } from 'react'
import { BackLink } from '../components/BackLink'
import { CatalogLoadMore } from '../components/CatalogLoadMore'
import { EnvResourcesSkeleton, PageSkeleton } from '../components/ui/Skeleton'
import { useCatalogWindow } from '../hooks/use-catalog-window'
import { api, LAST_ENV_KEY, type Environment } from '../lib/api'
import { catalogMatchesQuery } from '../lib/new-resource-catalog'
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
  const [search, setSearch] = useState('')

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

  if ((envs.data?.environments || []).length === 1) {
    return <EnvResourcesSkeleton />
  }

  const q = search.trim().toLowerCase()
  const filteredEnvs = (envs.data?.environments || []).filter((env) =>
    catalogMatchesQuery(q, env.name, env.description),
  )

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

      {(envs.data?.environments || []).length > 0 && (
        <div className="relative max-w-xl">
          <Search className="pointer-events-none absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2 text-gray-400" />
          <input
            type="search"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search environments…"
            className="panel-field h-9 w-full rounded-md py-1.5 pr-3 pl-9 text-sm"
          />
        </div>
      )}

      <EnvironmentGrid projectId={projectId} environments={filteredEnvs} query={q} />

      {!filteredEnvs.length && (envs.data?.environments || []).length > 0 && (
        <div className="panel-card p-8 text-center text-sm text-gray-500 dark:text-gray-400">
          No environments match your search.
        </div>
      )}
      {!envs.data?.environments?.length && (
        <div className="panel-card p-8 text-center text-sm text-gray-500 dark:text-gray-400">
          No environments found.
        </div>
      )}

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

function EnvironmentGrid({
  projectId,
  environments,
  query,
}: {
  projectId: string
  environments: Environment[]
  query: string
}) {
  const { visible, hasMore, total, loadMoreRef, loadMore } = useCatalogWindow(environments, query)
  if (!environments.length) return null
  return (
    <>
      <div className="grid gap-3 lg:grid-cols-2">
        {visible.map((env) => (
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
      </div>
      <CatalogLoadMore
        hasMore={hasMore}
        shown={visible.length}
        total={total}
        noun="environments"
        loadMoreRef={loadMoreRef}
        onLoadMore={loadMore}
      />
    </>
  )
}
