import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import { routeSkeletonKind, tabFromSearch } from './route-skeleton.ts'

const pid = '8bde7540-409e-46b8-a9b4-ae88fd2fb3b6'
const eid = '708ccf57-f1a4-45a1-a604-74d143271fb6'
const aid = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'
const sid = '7b8d804e-b23b-4152-b632-2a2b7eee7427'

describe('routeSkeletonKind', () => {
  it('never uses list skeleton on application details', () => {
    const nested = `/projects/${pid}/environments/${eid}/applications/${aid}`
    assert.equal(routeSkeletonKind(nested), 'app')
    assert.equal(routeSkeletonKind(`/applications/${aid}`), 'app')
    assert.equal(routeSkeletonKind(nested, { tab: 'configuration' }), 'app-settings')
    assert.equal(routeSkeletonKind(nested, { tab: 'overview' }), 'app')
    assert.equal(routeSkeletonKind(nested, '?tab=configuration'), 'app-settings')
  })

  it('treats create-application as form, not app detail', () => {
    assert.equal(
      routeSkeletonKind(`/projects/${pid}/environments/${eid}/applications/new`),
      'form',
    )
  })

  it('service details match app-style skeletons', () => {
    const p = `/projects/${pid}/environments/${eid}/services/${sid}`
    assert.equal(routeSkeletonKind(p), 'service')
    assert.equal(routeSkeletonKind(p, { tab: 'configuration' }), 'service-settings')
  })

  it('env resources is not the project-list skeleton', () => {
    assert.equal(routeSkeletonKind(`/projects/${pid}/environments/${eid}`), 'env')
  })

  it('project picker vs projects index', () => {
    assert.equal(routeSkeletonKind(`/projects/${pid}`), 'project')
    assert.equal(routeSkeletonKind('/projects'), 'list')
  })

  it('deployments are simple, not live-usage overview', () => {
    assert.equal(
      routeSkeletonKind(`/projects/${pid}/environments/${eid}/applications/${aid}/deployments/d1`),
      'simple',
    )
  })

  it('shared-variables is not mistaken for env resources', () => {
    assert.equal(
      routeSkeletonKind(`/projects/${pid}/environments/${eid}/shared-variables`),
      'list',
    )
  })
})

describe('tabFromSearch', () => {
  it('reads tanstack object and query string', () => {
    assert.equal(tabFromSearch({ tab: 'configuration' }), 'configuration')
    assert.equal(tabFromSearch('?tab=configuration'), 'configuration')
    assert.equal(tabFromSearch({}), '')
  })
})
