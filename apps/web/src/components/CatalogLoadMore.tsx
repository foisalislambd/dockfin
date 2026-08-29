import type { RefObject } from 'react'
import { SERVICE_PAGE_SIZE } from '../lib/new-resource-catalog'

export function CatalogLoadMore({
  hasMore,
  shown,
  total,
  noun,
  loadMoreRef,
  onLoadMore,
}: {
  hasMore: boolean
  shown: number
  total: number
  noun: string
  loadMoreRef: RefObject<HTMLDivElement | null>
  onLoadMore: () => void
}) {
  if (hasMore) {
    return (
      <div ref={loadMoreRef} className="flex flex-col items-center gap-2 py-2">
        <p className="text-xs text-gray-500 dark:text-gray-400">
          Showing {shown} of {total} {noun}
        </p>
        <button
          type="button"
          className="text-sm font-medium text-brand-600 hover:underline dark:text-brand-400"
          onClick={onLoadMore}
        >
          Load more
        </button>
      </div>
    )
  }
  if (total > SERVICE_PAGE_SIZE) {
    return (
      <p className="text-xs text-gray-500 dark:text-gray-400">
        {total} {noun}
      </p>
    )
  }
  return null
}
