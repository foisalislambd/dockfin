import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useParams } from '@tanstack/react-router'
import { Boxes, Container, Search } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { ServiceLogo } from '../components/ServiceLogo'
import { PageSkeleton } from '../components/ui/Skeleton'
import { api, LAST_ENV_KEY, type Template } from '../lib/api'
import { Header } from './Servers'

type AppOption = {
  id: string
  title: string
  description: string
  logo: string
  buildPack: string
  sourceType?: string
}

/** Coolify Select.php — Git Based (Applications column 1). */
const GIT_BASED: AppOption[] = [
  {
    id: 'public',
    title: 'Public Repository',
    description:
      'You can deploy any kind of public repositories from the supported git providers.',
    logo: '/svgs/git.svg',
    buildPack: 'railpack',
    sourceType: 'public',
  },
  {
    id: 'private-gh-app',
    title: 'Private Repository (with GitHub App)',
    description: 'You can deploy public & private repositories through your GitHub Apps.',
    logo: '/svgs/github.svg',
    buildPack: 'railpack',
    sourceType: 'private-gh-app',
  },
  {
    id: 'private-deploy-key',
    title: 'Private Repository (with Deploy Key)',
    description: 'You can deploy private repositories with a deploy key.',
    logo: '/svgs/git.svg',
    buildPack: 'railpack',
    sourceType: 'private-deploy-key',
  },
  {
    id: 'compose-git',
    title: 'Docker Compose (Git)',
    description: 'Deploy a docker-compose stack from a Git repository.',
    logo: '/svgs/docker.svg',
    buildPack: 'dockercompose',
    sourceType: 'public',
  },
]

/** Coolify Select.php — Docker Based (Applications column 2). */
const DOCKER_BASED: AppOption[] = [
  {
    id: 'dockerfile',
    title: 'Dockerfile',
    description: 'You can deploy a simple Dockerfile, without Git.',
    logo: '/svgs/docker.svg',
    buildPack: 'dockerfile',
  },
  {
    id: 'docker-compose-empty',
    title: 'Docker Compose Empty',
    description: 'You can deploy complex application easily with Docker Compose, without Git.',
    logo: '/svgs/docker.svg',
    buildPack: 'compose-empty',
  },
  {
    id: 'docker-image',
    title: 'Docker Image',
    description: 'You can deploy an existing Docker Image from any Registry, without Git.',
    logo: '/svgs/docker.svg',
    buildPack: 'dockerimage',
  },
  {
    id: 'static',
    title: 'Static Site',
    description: 'Build a static site (HTML/JS/CSS) with Railpack or a Dockerfile and serve it.',
    logo: '/svgs/docker.svg',
    buildPack: 'static',
    sourceType: 'public',
  },
]

const DATABASES = [
  {
    id: 'postgresql',
    title: 'PostgreSQL',
    description:
      'PostgreSQL is an object-relational database known for its robustness, advanced features, and strong standards compliance.',
    logo: '/svgs/postgresql.svg',
  },
  {
    id: 'mysql',
    title: 'MySQL',
    description: 'MySQL is an open-source relational database management system.',
    logo: '/svgs/mysql.svg',
  },
  {
    id: 'mariadb',
    title: 'MariaDB',
    description:
      'MariaDB is a community-developed, commercially supported fork of the MySQL relational database management system.',
    logo: '/svgs/mariadb.svg',
  },
  {
    id: 'redis',
    title: 'Redis',
    description:
      'Redis is a source-available, in-memory storage, used as a distributed, in-memory key–value database, cache and message broker.',
    logo: '/svgs/redis.svg',
  },
  {
    id: 'keydb',
    title: 'KeyDB',
    description:
      'KeyDB is a database that offers high performance, low latency, and scalability for various data structures and workloads.',
    logo: '/svgs/keydb.svg',
  },
  {
    id: 'dragonfly',
    title: 'Dragonfly',
    description:
      'Dragonfly DB is a drop-in Redis replacement that delivers 25x more throughput and 12x faster snapshotting than Redis.',
    logo: '/svgs/dragonfly.svg',
  },
  {
    id: 'mongodb',
    title: 'MongoDB',
    description: 'MongoDB is a source-available, cross-platform, document-oriented database program.',
    logo: '/svgs/mongodb.svg',
  },
  {
    id: 'clickhouse',
    title: 'ClickHouse',
    description:
      'ClickHouse is a column-oriented database that supports real-time analytics, business intelligence, observability, ML and GenAI, and more.',
    logo: '/svgs/clickhouse.svg',
  },
]

function ResourceTile({
  title,
  description,
  logo,
  onClick,
  busy,
}: {
  title: string
  description: string
  logo: string
  onClick: () => void
  busy?: boolean
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={busy}
      className={`panel-card group flex w-full items-center gap-3 p-3 text-left transition hover:border-brand-300 dark:hover:border-brand-500/40 ${
        busy ? 'cursor-wait opacity-70' : ''
      }`}
    >
      <span className="flex h-[4.5rem] w-[4.5rem] shrink-0 items-center justify-center overflow-hidden rounded-lg bg-black/5 p-2 transition dark:bg-white/10">
        <ServiceLogo src={logo} name={title} className="h-full w-full" imgClassName="h-full w-full object-contain" />
      </span>
      <div className="min-w-0 flex-1">
        <div className="text-[15px] font-medium text-gray-900 dark:text-white">{title}</div>
        <p className="mt-1 line-clamp-2 text-xs text-gray-500 group-hover:text-gray-600 dark:text-gray-400 dark:group-hover:text-gray-300">
          {busy ? 'Creating…' : description}
        </p>
      </div>
    </button>
  )
}

export function NewResourcePage() {
  const { projectId, envId } = useParams({ strict: false }) as { projectId: string; envId: string }
  const nav = useNavigate()
  const qc = useQueryClient()
  const [search, setSearch] = useState('')
  const [category, setCategory] = useState('')
  const [creatingType, setCreatingType] = useState<string | null>(null)
  const [createError, setCreateError] = useState('')
  const [destinationId, setDestinationId] = useState('')

  const project = useQuery({
    queryKey: ['project', projectId],
    queryFn: () => api.getProject(projectId),
  })
  const envs = useQuery({
    queryKey: ['environments', projectId],
    queryFn: () => api.environments(projectId),
  })
  const env = (envs.data?.environments || []).find((e) => e.id === envId)
  const templates = useQuery({ queryKey: ['templates'], queryFn: api.templates })
  const dests = useQuery({ queryKey: ['destinations'], queryFn: api.destinations })

  useEffect(() => {
    const first = dests.data?.destinations?.[0]?.id || ''
    if (first && !destinationId) setDestinationId(first)
  }, [dests.data, destinationId])

  const categories = useMemo(() => {
    const set = new Set<string>()
    for (const t of templates.data?.templates || []) {
      if (t.category) set.add(t.category)
    }
    return Array.from(set).sort((a, b) => a.localeCompare(b))
  }, [templates.data])

  const q = search.trim().toLowerCase()
  const matchText = (...parts: Array<string | undefined>) => {
    if (!q) return true
    return parts.some((p) => (p || '').toLowerCase().includes(q))
  }

  const filteredGit = GIT_BASED.filter((a) =>
    matchText(a.title, a.description, a.buildPack, 'git', 'repository', 'application', 'compose'),
  )
  const filteredDocker = DOCKER_BASED.filter((a) =>
    matchText(a.title, a.description, a.buildPack, 'docker', 'image', 'compose', 'static', 'application'),
  )
  const filteredDbs = DATABASES.filter((d) =>
    matchText(d.id, d.title, d.description, 'database'),
  )

  const filteredServices = useMemo(() => {
    const list = templates.data?.templates || []
    return list.filter((t) => {
      if (category && (t.category || '').toLowerCase() !== category.toLowerCase()) return false
      return matchText(t.name, t.type, t.description, t.category, 'service')
    })
  }, [templates.data, category, q])

  const goApp = (buildPack: string, sourceType?: string) => {
    localStorage.setItem(LAST_ENV_KEY, envId)
    if (buildPack === 'compose-empty') {
      void nav({
        to: '/projects/$projectId/environments/$envId/services/new',
        params: { projectId, envId },
        search: { empty_compose: '1', environment_id: undefined },
      })
      return
    }
    void nav({
      to: '/projects/$projectId/environments/$envId/applications/new',
      params: { projectId, envId },
      search: {
        build_pack: buildPack,
        environment_id: undefined,
        source_type: sourceType || undefined,
      },
    })
  }

  const goDb = (engine: string) => {
    localStorage.setItem(LAST_ENV_KEY, envId)
    void nav({
      to: '/projects/$projectId/environments/$envId/databases/new',
      params: { projectId, envId },
      search: { engine, environment_id: undefined },
    })
  }

  const createService = useMutation({
    mutationFn: (tpl: Template) =>
      api.createService({
        name: tpl.name.toLowerCase().replace(/\s+/g, '-'),
        environment_id: envId,
        destination_id: destinationId || undefined,
        template: tpl.type,
      }),
    onSuccess: (svc) => {
      localStorage.setItem(LAST_ENV_KEY, envId)
      void qc.invalidateQueries({ queryKey: ['services'] })
      void qc.invalidateQueries({ queryKey: ['services', envId] })
      void nav({
        to: '/projects/$projectId/environments/$envId/services/$svcId',
        params: { projectId, envId, svcId: svc.id },
        search: { deploy: undefined },
      })
    },
    onError: (e: Error) => {
      setCreateError(e.message)
      setCreatingType(null)
    },
  })

  if (project.isLoading || templates.isLoading) return <PageSkeleton cards={3} />

  const showApps = filteredGit.length > 0 || filteredDocker.length > 0
  const showServices =
    filteredServices.length > 0 || (!q && !category && (templates.data?.templates || []).length > 0)

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
            {env?.name || 'Resources'}
          </Link>
          <span>/</span>
          <span className="text-gray-900 dark:text-white">New</span>
        </nav>
        <Header title="New Resource" />
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
          Deploy resources, like Applications, Databases, Services…
        </p>
      </div>

      <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
        <div className="relative min-w-0 flex-1">
          <Search className="pointer-events-none absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2 text-gray-400" />
          <input
            type="search"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Type / to search…"
            className="panel-field h-9 w-full rounded-md py-1.5 pr-3 pl-9 text-sm"
            onKeyDown={(e) => {
              if (e.key === '/') e.stopPropagation()
            }}
          />
        </div>
        <select
          value={category}
          onChange={(e) => setCategory(e.target.value)}
          className="panel-field h-9 w-full rounded-md px-3 text-sm sm:w-56"
          aria-label="Filter by category"
        >
          <option value="">All categories</option>
          {categories.map((c) => (
            <option key={c} value={c}>
              {c}
            </option>
          ))}
        </select>
      </div>

      {createError && (
        <p className="text-sm text-error-500" role="alert">
          {createError}
        </p>
      )}

      {showApps && (
        <section className="space-y-4">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">Applications</h2>
          <div className="grid gap-8 lg:grid-cols-2">
            {filteredGit.length > 0 && (
              <div className="space-y-3">
                <h3 className="text-sm font-semibold text-gray-600 dark:text-gray-300">Git Based</h3>
                <div className="grid gap-3">
                  {filteredGit.map((a) => (
                    <ResourceTile
                      key={a.id}
                      title={a.title}
                      description={a.description}
                      logo={a.logo}
                      onClick={() => goApp(a.buildPack, a.sourceType)}
                    />
                  ))}
                </div>
              </div>
            )}
            {filteredDocker.length > 0 && (
              <div className="space-y-3">
                <h3 className="text-sm font-semibold text-gray-600 dark:text-gray-300">Docker Based</h3>
                <div className="grid gap-3">
                  {filteredDocker.map((a) => (
                    <ResourceTile
                      key={a.id}
                      title={a.title}
                      description={a.description}
                      logo={a.logo}
                      onClick={() => goApp(a.buildPack, a.sourceType)}
                    />
                  ))}
                </div>
              </div>
            )}
          </div>
        </section>
      )}

      {filteredDbs.length > 0 && (
        <section className="space-y-3">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">Databases</h2>
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {filteredDbs.map((d) => (
              <ResourceTile
                key={d.id}
                title={d.title}
                description={d.description}
                logo={d.logo}
                onClick={() => goDb(d.id)}
              />
            ))}
          </div>
        </section>
      )}

      {showServices && (
        <section className="space-y-3">
          <div className="flex items-center gap-2">
            <Boxes className="h-5 w-5 text-brand-600 dark:text-brand-400" />
            <h2 className="text-lg font-semibold text-gray-900 dark:text-white">Services</h2>
          </div>
          {(dests.data?.destinations || []).length > 0 ? (
            <label className="block max-w-md text-sm">
              <span className="mb-1 block text-gray-500 dark:text-gray-400">Destination</span>
              <select
                value={destinationId}
                onChange={(e) => setDestinationId(e.target.value)}
                className="panel-field h-9 w-full rounded-md px-3 text-sm"
              >
                {(dests.data?.destinations || []).map((d) => (
                  <option key={d.id} value={d.id}>
                    {d.name} ({d.network || 'default'})
                  </option>
                ))}
              </select>
            </label>
          ) : (
            <p className="text-sm text-amber-600 dark:text-amber-400">
              Add a server destination for free sslip.io domains before deploying services.
            </p>
          )}
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {filteredServices.map((t) => {
              const busy = creatingType === t.type && createService.isPending
              const logo = t.logo?.startsWith('http')
                ? t.logo
                : t.logo?.startsWith('/')
                  ? t.logo
                  : t.logo
                    ? `/svgs/${t.logo.replace(/^svgs\//, '')}`
                    : undefined
              return (
                <ResourceTile
                  key={t.type}
                  title={busy ? 'Creating…' : t.name}
                  description={t.description || t.category || t.type}
                  logo={logo || '/svgs/docker.svg'}
                  busy={busy || Boolean(creatingType)}
                  onClick={() => {
                    setCreateError('')
                    setCreatingType(t.type)
                    createService.mutate(t)
                  }}
                />
              )
            })}
          </div>
          {!filteredServices.length && (
            <p className="text-sm text-gray-500 dark:text-gray-400">No services match your search.</p>
          )}
        </section>
      )}

      {!showApps && !filteredDbs.length && !filteredServices.length && (
        <div className="panel-card flex flex-col items-center gap-2 p-10 text-center">
          <Container className="h-8 w-8 text-gray-400" />
          <p className="text-sm text-gray-500 dark:text-gray-400">No resources match your search.</p>
        </div>
      )}
    </div>
  )
}
