/** How many service tiles to mount at once. Search/filter still run on the full catalog. */
export const SERVICE_PAGE_SIZE = 24

export function catalogMatchesQuery(query: string, ...parts: Array<string | undefined>) {
  const needle = query.trim().toLowerCase()
  if (!needle) return true
  return parts.some((p) => (p || '').toLowerCase().includes(needle))
}

export type CatalogTemplate = {
  name: string
  type: string
  description?: string
  category?: string
}

/** Filter the full one-click catalog. Pagination must slice this result, not the raw list. */
export function filterServiceTemplates<T extends CatalogTemplate>(
  templates: T[],
  query: string,
  category: string,
): T[] {
  const cat = category.trim().toLowerCase()
  return templates.filter((t) => {
    if (cat && (t.category || '').toLowerCase() !== cat) return false
    return catalogMatchesQuery(query, t.name, t.type, t.description, t.category, 'service')
  })
}

export function pageCatalog<T>(items: T[], visibleCount: number): { visible: T[]; hasMore: boolean } {
  const n = Math.max(0, visibleCount)
  return {
    visible: items.slice(0, n),
    hasMore: n < items.length,
  }
}
