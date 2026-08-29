export const SEARCH_MAX_PER_GROUP = 8

function matches(query: string, ...parts: Array<string | undefined>) {
  const needle = query.trim().toLowerCase()
  if (!needle) return true
  return parts.some((p) => (p || '').toLowerCase().includes(needle))
}

export type SearchTarget =
  | { kind: 'href'; href: string }
  | { kind: 'project'; projectId: string }
  | { kind: 'environment'; projectId: string; envId: string }
  | { kind: 'application'; appId: string; projectId?: string; envId?: string }
  | { kind: 'database'; dbId: string; projectId?: string; envId?: string }
  | { kind: 'service'; svcId: string; projectId?: string; envId?: string }
  | { kind: 'server'; serverId: string }
  | { kind: 'git-source'; sourceId: string }
  | { kind: 'destination'; serverId: string }
  | { kind: 'storages' }
  | { kind: 'tags' }

export type SearchHit = {
  id: string
  group: string
  name: string
  hint?: string
  target: SearchTarget
}

export type SearchNavItem = { name: string; href: string }
export type SearchNavGroup = { id: string; label: string; items: SearchNavItem[] }

export type SearchEnv = { id: string; name: string; project_id: string; project_name: string }

export type GlobalSearchInput = {
  query: string
  navGroups: SearchNavGroup[]
  projects: Array<{ id: string; name: string; description?: string }>
  environments: SearchEnv[]
  applications: Array<{
    id: string
    name: string
    description?: string
    fqdn?: string
    git_repository?: string
    environment_id: string
  }>
  databases: Array<{
    id: string
    name: string
    description?: string
    engine?: string
    environment_id: string
  }>
  services: Array<{
    id: string
    name: string
    description?: string
    service_type?: string
    fqdn?: string
    environment_id: string
  }>
  servers: Array<{ id: string; name: string; ip?: string; public_ip?: string; description?: string }>
  gitSources: Array<{ id: string; name: string; provider?: string; organization?: string }>
  destinations: Array<{ id: string; name: string; network?: string; kind?: string; server_id: string }>
  storages: Array<{ id: string; name: string; bucket?: string; endpoint?: string }>
  tags: Array<{ id: string; name: string }>
}

function pageSlug(href: string) {
  return href.replace(/^\//, '').replace(/-/g, ' ')
}

function pushCapped(out: SearchHit[], counts: Map<string, number>, hit: SearchHit) {
  const n = counts.get(hit.group) || 0
  if (n >= SEARCH_MAX_PER_GROUP) return
  counts.set(hit.group, n + 1)
  out.push(hit)
}

/** Build ranked search hits. Empty query returns pages only (uncapped). */
export function buildGlobalSearchHits(input: GlobalSearchInput): SearchHit[] {
  const q = input.query.trim()
  const out: SearchHit[] = []
  const counts = new Map<string, number>()
  const envById = new Map(input.environments.map((e) => [e.id, e]))
  const serverById = new Map(input.servers.map((s) => [s.id, s]))

  for (const group of input.navGroups) {
    for (const item of group.items) {
      if (!matches(q, item.name, group.label, pageSlug(item.href))) continue
      const hit: SearchHit = {
        id: `page:${item.href}`,
        group: 'Pages',
        name: item.name,
        hint: group.label,
        target: { kind: 'href', href: item.href },
      }
      if (!q) {
        out.push(hit)
        continue
      }
      pushCapped(out, counts, hit)
    }
  }

  if (!q) return out

  for (const p of input.projects) {
    if (!matches(q, p.name, p.description)) continue
    pushCapped(out, counts, {
      id: `project:${p.id}`,
      group: 'Projects',
      name: p.name,
      hint: p.description || undefined,
      target: { kind: 'project', projectId: p.id },
    })
  }

  for (const e of input.environments) {
    if (!matches(q, e.name, e.project_name)) continue
    pushCapped(out, counts, {
      id: `env:${e.id}`,
      group: 'Environments',
      name: e.name,
      hint: e.project_name,
      target: { kind: 'environment', projectId: e.project_id, envId: e.id },
    })
  }

  for (const a of input.applications) {
    const env = envById.get(a.environment_id)
    if (!matches(q, a.name, a.description, a.fqdn, a.git_repository)) continue
    pushCapped(out, counts, {
      id: `app:${a.id}`,
      group: 'Applications',
      name: a.name,
      hint: env?.project_name,
      target: { kind: 'application', appId: a.id, projectId: env?.project_id, envId: a.environment_id },
    })
  }

  for (const d of input.databases) {
    const env = envById.get(d.environment_id)
    if (!matches(q, d.name, d.description, d.engine)) continue
    pushCapped(out, counts, {
      id: `db:${d.id}`,
      group: 'Databases',
      name: d.name,
      hint: d.engine || env?.project_name,
      target: { kind: 'database', dbId: d.id, projectId: env?.project_id, envId: d.environment_id },
    })
  }

  for (const s of input.services) {
    const env = envById.get(s.environment_id)
    if (!matches(q, s.name, s.description, s.service_type, s.fqdn)) continue
    pushCapped(out, counts, {
      id: `svc:${s.id}`,
      group: 'Services',
      name: s.name,
      hint: s.service_type || env?.project_name,
      target: { kind: 'service', svcId: s.id, projectId: env?.project_id, envId: s.environment_id },
    })
  }

  for (const s of input.servers) {
    if (!matches(q, s.name, s.ip, s.public_ip, s.description)) continue
    pushCapped(out, counts, {
      id: `server:${s.id}`,
      group: 'Servers',
      name: s.name,
      hint: s.ip || s.public_ip,
      target: { kind: 'server', serverId: s.id },
    })
  }

  for (const g of input.gitSources) {
    if (!matches(q, g.name, g.provider, g.organization)) continue
    pushCapped(out, counts, {
      id: `git:${g.id}`,
      group: 'Sources',
      name: g.name,
      hint: g.provider,
      target: { kind: 'git-source', sourceId: g.id },
    })
  }

  for (const d of input.destinations) {
    const serverName = serverById.get(d.server_id)?.name
    if (!matches(q, d.name, d.network, d.kind, serverName)) continue
    pushCapped(out, counts, {
      id: `dest:${d.id}`,
      group: 'Destinations',
      name: d.name,
      hint: serverName || d.kind || d.network,
      target: { kind: 'destination', serverId: d.server_id },
    })
  }

  for (const s of input.storages) {
    if (!matches(q, s.name, s.bucket, s.endpoint)) continue
    pushCapped(out, counts, {
      id: `s3:${s.id}`,
      group: 'Storages',
      name: s.name,
      hint: s.bucket,
      target: { kind: 'storages' },
    })
  }

  for (const t of input.tags) {
    if (!matches(q, t.name)) continue
    pushCapped(out, counts, {
      id: `tag:${t.id}`,
      group: 'Tags',
      name: t.name,
      target: { kind: 'tags' },
    })
  }

  return out
}
