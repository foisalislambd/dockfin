import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useParams } from '@tanstack/react-router'
import {
  Box,
  Boxes,
  Container,
  Database,
  FileCode2,
  GitBranch,
  Image as ImageIcon,
  Layers,
  Rocket,
  Search,
} from 'lucide-react'
import { useMemo, useState } from 'react'
import { ServiceLogo } from '../components/ServiceLogo'
import { PageSkeleton } from '../components/ui/Skeleton'
import { api, LAST_ENV_KEY, type Template } from '../lib/api'
import { Header } from './Servers'

const GIT_APPS = [
  {
    id: 'public',
    title: 'Public Repository',
    description: 'Deploy a public Git repository (Dockerfile / Nixpacks / Compose / Static).',
    icon: GitBranch,
    buildPack: 'nixpacks',
  },
  {
    id: 'dockerfile',
    title: 'Dockerfile',
    description: 'Build from a Git repository Dockerfile.',
    icon: FileCode2,
    buildPack: 'dockerfile',
  },
  {
    id: 'compose',
    title: 'Docker Compose',
    description: 'Deploy a docker-compose stack from Git.',
    icon: Layers,
    buildPack: 'dockercompose',
  },
]

const DOCKER_APPS = [
  {
    id: 'dockerimage',
    title: 'Docker Image',
    description: 'Pull and run a public or private container image.',
    icon: ImageIcon,
    buildPack: 'dockerimage',
  },
]

const DATABASES = [
  { id: 'postgresql', title: 'PostgreSQL', description: 'Relational DB — default for most apps.' },
  { id: 'mysql', title: 'MySQL', description: 'Classic relational database.' },
  { id: 'mariadb', title: 'MariaDB', description: 'MySQL-compatible community engine.' },
  { id: 'mongodb', title: 'MongoDB', description: 'Document store.' },
  { id: 'redis', title: 'Redis', description: 'In-memory cache / queue.' },
  { id: 'keydb', title: 'KeyDB', description: 'High-performance Redis fork.' },
  { id: 'dragonfly', title: 'Dragonfly', description: 'Modern Redis-compatible store.' },
  { id: 'clickhouse', title: 'ClickHouse', description: 'Columnar analytics database.' },
]

function ResourceTile({
  title,
  description,
  icon: Icon,
  onClick,
  logo,
}: {
  title: string
  description: string
  icon?: typeof Rocket
  logo?: string
  onClick: () => void
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="panel-card flex w-full items-start gap-3 p-4 text-left transition hover:border-brand-300 dark:hover:border-brand-500/40"
    >
      {logo ? (
        <ServiceLogo src={logo} name={title} className="h-10 w-10 shrink-0" />
      ) : Icon ? (
        <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-brand-50 dark:bg-brand-500/15">
          <Icon className="h-5 w-5 text-brand-600 dark:text-brand-400" />
        </div>
      ) : (
        <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-gray-100 dark:bg-gray-800">
          <Box className="h-5 w-5 text-gray-500" />
        </div>
      )}
      <div className="min-w-0">
        <div className="font-semibold text-gray-900 dark:text-white">{title}</div>
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">{description}</p>
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

  const categories = useMemo(() => {
    const set = new Set<string>()
    for (const t of templates.data?.templates || []) {
      if (t.category) set.add(t.category)
    }
    return Array.from(set).sort((a, b) => a.localeCompare(b))
  }, [templates.data])

  const q = search.trim().toLowerCase()

  const matchText = (parts: Array<string | undefined>) => {
    if (!q) return true
    return parts.some((p) => (p || '').toLowerCase().includes(q))
  }

  const showGit = matchText(['git', 'repository', 'dockerfile', 'compose', 'nixpacks', 'application'])
  const showDocker = matchText(['docker', 'image', 'application'])
  const filteredDbs = DATABASES.filter(
    (d) => !q || matchText([d.id, d.title, d.description, 'database']),
  )

  const filteredServices = useMemo(() => {
    const list = templates.data?.templates || []
    return list.filter((t) => {
      if (category && (t.category || '').toLowerCase() !== category.toLowerCase()) return false
      return matchText([t.name, t.type, t.description, t.category, 'service'])
    })
  }, [templates.data, category, q])

  const goApp = (buildPack: string) => {
    localStorage.setItem(LAST_ENV_KEY, envId)
    void nav({
      to: '/projects/$projectId/environments/$envId/applications/new',
      params: { projectId, envId },
      search: { build_pack: buildPack, environment_id: undefined },
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
    mutationFn: (tpl: Template) => {
      const destId = dests.data?.destinations?.[0]?.id || ''
      return api.createService({
        name: tpl.name.toLowerCase().replace(/\s+/g, '-'),
        environment_id: envId,
        destination_id: destId || undefined,
        template: tpl.type,
      })
    },
    onSuccess: (svc) => {
      localStorage.setItem(LAST_ENV_KEY, envId)
      void qc.invalidateQueries({ queryKey: ['services'] })
      void qc.invalidateQueries({ queryKey: ['services', envId] })
      void nav({
        to: '/projects/$projectId/environments/$envId/services/$svcId',
        params: { projectId, envId, svcId: svc.id },
      })
    },
    onError: (e: Error) => {
      setCreateError(e.message)
      setCreatingType(null)
    },
  })

  const onServiceClick = (tpl: Template) => {
    setCreateError('')
    setCreatingType(tpl.type)
    createService.mutate(tpl)
  }

  if (project.isLoading || templates.isLoading) return <PageSkeleton cards={3} />

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

      {(showGit || showDocker) && (
        <section className="space-y-3">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">Applications</h2>
          <div className="grid gap-6 lg:grid-cols-2">
            {showGit && (
              <div className="space-y-3">
                <h3 className="text-sm font-medium text-gray-500 dark:text-gray-400">Git Based</h3>
                <div className="grid gap-3">
                  {GIT_APPS.filter((a) => matchText([a.title, a.description, a.buildPack, 'git'])).map(
                    (a) => (
                      <ResourceTile
                        key={a.id}
                        title={a.title}
                        description={a.description}
                        icon={a.icon}
                        onClick={() => goApp(a.buildPack)}
                      />
                    ),
                  )}
                </div>
              </div>
            )}
            {showDocker && (
              <div className="space-y-3">
                <h3 className="text-sm font-medium text-gray-500 dark:text-gray-400">Docker Based</h3>
                <div className="grid gap-3">
                  {DOCKER_APPS.filter((a) =>
                    matchText([a.title, a.description, a.buildPack, 'docker']),
                  ).map((a) => (
                    <ResourceTile
                      key={a.id}
                      title={a.title}
                      description={a.description}
                      icon={a.icon}
                      onClick={() => goApp(a.buildPack)}
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
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
            {filteredDbs.map((d) => (
              <ResourceTile
                key={d.id}
                title={d.title}
                description={d.description}
                icon={Database}
                onClick={() => goDb(d.id)}
              />
            ))}
          </div>
        </section>
      )}

      {(filteredServices.length > 0 || (!q && !category && (templates.data?.templates || []).length > 0)) && (
        <section className="space-y-3">
          <div className="flex items-center gap-2">
            <Boxes className="h-5 w-5 text-brand-600 dark:text-brand-400" />
            <h2 className="text-lg font-semibold text-gray-900 dark:text-white">Services</h2>
          </div>
          <p className="text-sm text-gray-500 dark:text-gray-400">
            One-click services from the catalog. Click to create in this environment
            {dests.data?.destinations?.[0]
              ? ` (destination: ${dests.data.destinations[0].name})`
              : ' — add a server destination for free sslip.io domains'}
            .
          </p>
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
            {filteredServices.map((t) => {
              const busy = creatingType === t.type && createService.isPending
              return (
                <button
                  key={t.type}
                  type="button"
                  disabled={Boolean(creatingType)}
                  onClick={() => onServiceClick(t)}
                  className={`panel-card flex items-start gap-3 p-3 text-left transition ${
                    busy
                      ? 'border-brand-400 opacity-80'
                      : 'hover:border-brand-300 dark:hover:border-brand-500/40'
                  } disabled:cursor-wait`}
                >
                  <ServiceLogo src={t.logo} name={t.name} className="h-10 w-10 shrink-0" />
                  <div className="min-w-0">
                    <div className="truncate font-semibold text-gray-900 dark:text-white">
                      {busy ? 'Creating…' : t.name}
                    </div>
                    <p className="mt-0.5 line-clamp-2 text-xs text-gray-500 dark:text-gray-400">
                      {t.description}
                    </p>
                    {t.category && (
                      <div className="mt-1 text-[10px] font-semibold tracking-wide text-gray-400 uppercase">
                        {t.category}
                      </div>
                    )}
                  </div>
                </button>
              )
            })}
          </div>
          {!filteredServices.length && (
            <p className="text-sm text-gray-500 dark:text-gray-400">No services match your search.</p>
          )}
        </section>
      )}

      {!showGit && !showDocker && !filteredDbs.length && !filteredServices.length && (
        <div className="panel-card flex flex-col items-center gap-2 p-10 text-center">
          <Container className="h-8 w-8 text-gray-400" />
          <p className="text-sm text-gray-500 dark:text-gray-400">No resources match your search.</p>
        </div>
      )}
    </div>
  )
}
