import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import { SEARCH_MAX_PER_GROUP, buildGlobalSearchHits } from './global-search.ts'

const navGroups = [
  {
    id: 'workspace',
    label: 'Workspace',
    items: [
      { name: 'Dashboard', href: '/dashboard' },
      { name: 'Projects', href: '/projects' },
      { name: 'Terminal', href: '/terminal' },
    ],
  },
  {
    id: 'infrastructure',
    label: 'Infrastructure',
    items: [
      { name: 'Servers', href: '/servers' },
      { name: 'Sources', href: '/git-sources' },
    ],
  },
]

const env = { id: 'e1', name: 'production', project_id: 'p1', project_name: 'foisal' }

function base(over: Partial<Parameters<typeof buildGlobalSearchHits>[0]> = {}) {
  return buildGlobalSearchHits({
    query: '',
    navGroups,
    projects: [{ id: 'p1', name: 'foisal', description: 'fs' }],
    environments: [env],
    applications: [
      { id: 'a1', name: 'api', environment_id: 'e1', fqdn: 'api.example.com', git_repository: 'org/api' },
    ],
    databases: [{ id: 'd1', name: 'pg', engine: 'postgres', environment_id: 'e1' }],
    services: [{ id: 's1', name: 'wordpress-with-mysql', service_type: 'wordpress-with-mysql', environment_id: 'e1' }],
    servers: [{ id: 'srv1', name: 'vps-1', ip: '10.0.0.8' }],
    gitSources: [{ id: 'g1', name: 'github', provider: 'github' }],
    destinations: [{ id: 'dest1', name: 'dockfin', network: 'dockfin', kind: 'standalone', server_id: 'srv1' }],
    storages: [{ id: 'st1', name: 'backups', bucket: 'dockfin-bak' }],
    tags: [{ id: 't1', name: 'prod' }],
    ...over,
  })
}

describe('buildGlobalSearchHits', () => {
  it('empty query returns every page and nothing else', () => {
    const hits = base({ query: '' })
    assert.equal(hits.length, 5)
    assert.ok(hits.every((h) => h.group === 'Pages'))
  })

  it('does not treat "/" as a match for every page', () => {
    const hits = base({ query: '/' })
    assert.ok(!hits.some((h) => h.group === 'Pages'))
  })

  it('finds Sources via git-sources slug, not only the label Sources', () => {
    const hits = base({ query: 'git' })
    assert.ok(hits.some((h) => h.name === 'Sources' && h.target.kind === 'href'))
  })

  it('finds a service by name without dropping it when env list is empty', () => {
    const hits = base({ query: 'wordpress', environments: [] })
    const svc = hits.find((h) => h.id === 'svc:s1')
    assert.ok(svc)
    assert.deepEqual(svc.target, { kind: 'service', svcId: 's1', projectId: undefined, envId: 'e1' })
  })

  it('uses nested ids when the environment is known', () => {
    const hits = base({ query: 'wordpress' })
    const svc = hits.find((h) => h.id === 'svc:s1')
    assert.deepEqual(svc?.target, {
      kind: 'service',
      svcId: 's1',
      projectId: 'p1',
      envId: 'e1',
    })
  })

  it('does not list every service when searching a project name', () => {
    const hits = base({ query: 'foisal' })
    assert.ok(hits.some((h) => h.id === 'project:p1'))
    assert.ok(hits.some((h) => h.id === 'env:e1'))
    assert.ok(!hits.some((h) => h.id === 'svc:s1'))
  })

  it('matches destinations by host server name', () => {
    const hits = base({ query: 'vps-1' })
    assert.ok(hits.some((h) => h.id === 'server:srv1'))
    assert.ok(hits.some((h) => h.id === 'dest:dest1' && h.target.kind === 'destination'))
  })

  it('caps each group so later groups are not dropped', () => {
    const services = Array.from({ length: 20 }, (_, i) => ({
      id: `s${i}`,
      name: `alpha-svc-${i}`,
      environment_id: 'e1',
    }))
    const hits = base({
      query: 'alpha',
      services,
      tags: [{ id: 't1', name: 'alpha-tag' }],
    })
    assert.equal(hits.filter((h) => h.group === 'Services').length, SEARCH_MAX_PER_GROUP)
    assert.ok(hits.some((h) => h.id === 'tag:t1'))
  })
})
