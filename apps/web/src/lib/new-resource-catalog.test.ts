import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import {
  SERVICE_PAGE_SIZE,
  catalogMatchesQuery,
  filterServiceTemplates,
  pageCatalog,
} from './new-resource-catalog.ts'

const catalog = [
  { name: 'Activepieces', type: 'activepieces', description: 'automation', category: 'automation' },
  { name: 'N8n', type: 'n8n', description: 'workflow automation', category: 'automation' },
  { name: 'N8n With Postgres', type: 'n8n-with-postgres', description: 'n8n + db', category: 'automation' },
  { name: 'Wordpress With Mysql', type: 'wordpress-with-mysql', description: 'CMS', category: 'cms' },
  { name: 'Uptime Kuma', type: 'uptime-kuma', description: 'monitoring', category: 'monitoring' },
]

describe('filterServiceTemplates', () => {
  it('searches the full catalog, not a visible page', () => {
    const matched = filterServiceTemplates(catalog, 'wordpress', '')
    assert.equal(matched.length, 1)
    assert.equal(matched[0]?.type, 'wordpress-with-mysql')
  })

  it('matches name, type, description, and category', () => {
    assert.equal(filterServiceTemplates(catalog, 'n8n', '').length, 2)
    assert.equal(filterServiceTemplates(catalog, 'n8n-with-postgres', '').length, 1)
    assert.equal(filterServiceTemplates(catalog, 'monitoring', '').length, 1)
    assert.equal(filterServiceTemplates(catalog, 'cms', '').length, 1)
  })

  it('applies category and search together (AND)', () => {
    const matched = filterServiceTemplates(catalog, 'n8n', 'automation')
    assert.equal(matched.length, 2)
    assert.equal(filterServiceTemplates(catalog, 'n8n', 'cms').length, 0)
  })

  it('is case-insensitive and trims query', () => {
    assert.equal(filterServiceTemplates(catalog, '  WordPress  ', '').length, 1)
  })
})

describe('pageCatalog', () => {
  it('slices after filtering so later catalog items stay searchable', () => {
    const matched = filterServiceTemplates(catalog, '', '')
    const { visible, hasMore } = pageCatalog(matched, 2)
    assert.deepEqual(
      visible.map((t) => t.type),
      ['activepieces', 'n8n'],
    )
    assert.equal(hasMore, true)
    assert.equal(filterServiceTemplates(catalog, 'wordpress', '').length, 1)
  })

  it('shows every match when the filtered set is smaller than a page', () => {
    const matched = filterServiceTemplates(catalog, 'wordpress', '')
    const { visible, hasMore } = pageCatalog(matched, SERVICE_PAGE_SIZE)
    assert.equal(visible.length, 1)
    assert.equal(hasMore, false)
  })
})

describe('catalogMatchesQuery', () => {
  it('matches any provided field', () => {
    assert.equal(catalogMatchesQuery('git', 'Public Repository', 'railpack', 'git'), true)
    assert.equal(catalogMatchesQuery('zzz', 'Public Repository'), false)
    assert.equal(catalogMatchesQuery('', 'anything'), true)
  })
})
