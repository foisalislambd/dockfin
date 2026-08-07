import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { api, type Tag } from '../lib/api'
import { useToast } from './Toast'
import { Btn, Input } from '../pages/Servers'

export function ResourceTagsPanel({
  resourceType,
  resourceId,
}: {
  resourceType: 'application' | 'database' | 'service'
  resourceId: string
}) {
  const qc = useQueryClient()
  const toast = useToast()
  const [name, setName] = useState('')
  const [color, setColor] = useState('#14b8a6')

  const attached = useQuery({
    queryKey: ['resource-tags', resourceType, resourceId],
    queryFn: () => api.resourceTags(resourceType, resourceId),
    enabled: Boolean(resourceId),
  })
  const allTags = useQuery({ queryKey: ['tags'], queryFn: api.tags })

  const refresh = () => {
    void qc.invalidateQueries({ queryKey: ['resource-tags', resourceType, resourceId] })
    void qc.invalidateQueries({ queryKey: ['tags'] })
  }

  const attach = useMutation({
    mutationFn: (body: { tag_id?: string; name?: string; color?: string }) =>
      api.attachTag({ ...body, resource_type: resourceType, resource_id: resourceId }),
    onSuccess: () => {
      setName('')
      refresh()
      toast.success('Tag attached')
    },
    onError: (e: Error) => toast.error(e.message || 'Failed'),
  })

  const detach = useMutation({
    mutationFn: (tagId: string) => api.detachTag(tagId, resourceType, resourceId),
    onSuccess: () => {
      refresh()
      toast.success('Tag removed')
    },
    onError: (e: Error) => toast.error(e.message || 'Failed'),
  })

  const attachedIds = new Set((attached.data?.tags || []).map((t) => t.id))
  const available = (allTags.data?.tags || []).filter((t) => !attachedIds.has(t.id))

  return (
    <div className="space-y-4">
      <div className="panel-card space-y-3 p-5">
        <h3 className="text-sm font-medium text-gray-900 dark:text-white">Attached tags</h3>
        <div className="flex flex-wrap gap-2">
          {(attached.data?.tags || []).map((t) => (
            <TagChip key={t.id} tag={t} onRemove={() => detach.mutate(t.id)} />
          ))}
          {!attached.data?.tags?.length && (
            <p className="text-sm text-gray-500 dark:text-gray-400">No tags yet.</p>
          )}
        </div>
      </div>

      {available.length > 0 && (
        <div className="panel-card space-y-3 p-5">
          <h3 className="text-sm font-medium text-gray-900 dark:text-white">Add existing tag</h3>
          <div className="flex flex-wrap gap-2">
            {available.map((t) => (
              <button
                key={t.id}
                type="button"
                onClick={() => attach.mutate({ tag_id: t.id })}
                className="inline-flex items-center gap-1.5 rounded-full border border-gray-200 px-2.5 py-1 text-xs hover:bg-gray-50 dark:border-gray-700 dark:hover:bg-white/5"
              >
                <span
                  className="h-2 w-2 rounded-full"
                  style={{ backgroundColor: t.color || '#14b8a6' }}
                />
                {t.name}
              </button>
            ))}
          </div>
        </div>
      )}

      <form
        className="panel-card space-y-3 p-5"
        onSubmit={(e) => {
          e.preventDefault()
          if (!name.trim()) return
          attach.mutate({ name: name.trim(), color })
        }}
      >
        <h3 className="text-sm font-medium text-gray-900 dark:text-white">Create & attach</h3>
        <div className="grid gap-3 sm:grid-cols-[1fr_auto_auto] sm:items-end">
          <Input label="Tag name" value={name} onChange={setName} />
          <label className="block text-sm">
            <span className="mb-1 block text-gray-500 dark:text-gray-400">Color</span>
            <input
              type="color"
              value={color}
              onChange={(e) => setColor(e.target.value)}
              className="h-10 w-14 cursor-pointer rounded border border-gray-200 bg-transparent dark:border-gray-700"
            />
          </label>
          <Btn primary type="submit" disabled={!name.trim() || attach.isPending}>
            {attach.isPending ? 'Adding…' : 'Add tag'}
          </Btn>
        </div>
      </form>
    </div>
  )
}

function TagChip({ tag, onRemove }: { tag: Tag; onRemove: () => void }) {
  return (
    <span className="inline-flex items-center gap-1.5 rounded-full border border-gray-200 bg-gray-50 px-2.5 py-1 text-xs dark:border-gray-700 dark:bg-white/5">
      <span className="h-2 w-2 rounded-full" style={{ backgroundColor: tag.color || '#14b8a6' }} />
      {tag.name}
      <button
        type="button"
        onClick={onRemove}
        className="ml-0.5 text-gray-400 hover:text-error-500"
        aria-label={`Remove ${tag.name}`}
      >
        ×
      </button>
    </span>
  )
}
