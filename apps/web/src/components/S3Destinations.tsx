import type { BackupExecution, S3Storage } from '../lib/api'

export function S3DestinationPicker({
  storages,
  selected,
  onChange,
}: {
  storages: S3Storage[]
  selected: string[]
  onChange: (ids: string[]) => void
}) {
  if (!storages.length) {
    return <p className="text-sm text-gray-500 dark:text-gray-400">No S3 storages yet. Add one under Storages.</p>
  }
  return (
    <div className="space-y-2">
      {storages.map((storage) => {
        const checked = selected.includes(storage.id)
        return (
          <label key={storage.id} className="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
            <input
              type="checkbox"
              checked={checked}
              onChange={() => {
                if (checked) {
                  onChange(selected.filter((id) => id !== storage.id))
                  return
                }
                if (selected.length >= 8) return
                onChange([...selected, storage.id])
              }}
            />
            <span>
              {storage.name} ({storage.bucket})
            </span>
          </label>
        )
      })}
      <p className="text-xs text-gray-500 dark:text-gray-400">
        The same backup file is copied to every selected storage. Up to 8 storages.
      </p>
    </div>
  )
}

export function s3DestinationLabel(
  backup: { s3_storage_ids?: string[]; s3_storage_id?: string | null },
  storages: S3Storage[],
) {
  const ids = backup.s3_storage_ids?.length
    ? backup.s3_storage_ids
    : backup.s3_storage_id
      ? [backup.s3_storage_id]
      : []
  if (!ids.length) return '—'
  return ids.map((id) => storages.find((storage) => storage.id === id)?.name || id.slice(0, 8)).join(', ')
}

export function S3ReplicaStatus({ execution }: { execution: BackupExecution }) {
  const rows = execution.s3_replicas || []
  if (!rows.length) return <>{execution.s3_uploaded ? 'yes' : '—'}</>
  const copied = rows.filter((row) => row.uploaded).length
  const title = rows
    .map((row) => `${row.storage_name || 'storage'}: ${row.uploaded ? 'copied' : row.message || 'failed'}`)
    .join('\n')
  return <span title={title}>{`${copied}/${rows.length}`}</span>
}
