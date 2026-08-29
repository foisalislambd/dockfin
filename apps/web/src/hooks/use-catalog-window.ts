import { useEffect, useRef, useState } from 'react'
import { SERVICE_PAGE_SIZE, pageCatalog } from '../lib/new-resource-catalog'

/**
 * Window a pre-filtered list for the DOM. Search/filter must run on the full
 * array before this hook — it only controls how many items are mounted.
 */
export function useCatalogWindow<T>(items: T[], filterKey: string, pageSize = SERVICE_PAGE_SIZE) {
  const [visibleCount, setVisibleCount] = useState(pageSize)
  const [appliedKey, setAppliedKey] = useState(filterKey)
  if (appliedKey !== filterKey) {
    setAppliedKey(filterKey)
    setVisibleCount(pageSize)
  }

  const loadMoreRef = useRef<HTMLDivElement>(null)
  const lenRef = useRef(items.length)
  lenRef.current = items.length
  const sizeRef = useRef(pageSize)
  sizeRef.current = pageSize

  const { visible, hasMore } = pageCatalog(items, visibleCount)

  const loadMore = () => setVisibleCount((n) => Math.min(n + sizeRef.current, lenRef.current))

  useEffect(() => {
    if (!hasMore) return
    const el = loadMoreRef.current
    if (!el) return
    const root = el.closest('[data-catalog-scroll]') || el.closest('.panel-scrollbar')
    const obs = new IntersectionObserver(
      (entries) => {
        if (!entries[0]?.isIntersecting) return
        setVisibleCount((n) => Math.min(n + sizeRef.current, lenRef.current))
      },
      { root: root instanceof Element ? root : null, rootMargin: '200px 0px' },
    )
    obs.observe(el)
    return () => obs.disconnect()
  }, [hasMore, visibleCount, filterKey])

  return { visible, hasMore, total: items.length, loadMoreRef, loadMore }
}
