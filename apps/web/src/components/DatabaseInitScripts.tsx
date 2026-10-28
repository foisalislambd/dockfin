import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { api } from '../lib/api'
import { useToast } from './Toast'
import { Btn } from './ui/Button'
import { Input } from './ui/Input'

type Script = { filename: string; content: string }

export function DatabaseInitScripts({ databaseId }: { databaseId: string }) {
  const qc = useQueryClient()
  const toast = useToast()
  const scripts = useQuery({
    queryKey: ['database-init', databaseId],
    queryFn: () => api.databaseInitScripts(databaseId),
  })
  const [draft, setDraft] = useState<Script[] | null>(null)
  const rows = draft ?? scripts.data?.scripts ?? []

  const save = useMutation({
    mutationFn: () => api.saveDatabaseInitScripts(databaseId, rows),
    onSuccess: (res) => {
      setDraft(null)
      qc.setQueryData(['database-init', databaseId], res)
      toast.success('Init scripts saved')
    },
    onError: (e: Error) => toast.error(e.message || 'Failed to save init scripts'),
  })

  if (scripts.isLoading) {
    return <p className="mt-6 text-sm text-gray-500 dark:text-gray-400">Loading init scripts…</p>
  }
  if (scripts.isError || !scripts.data) {
    return (
      <p className="mt-6 text-sm text-error-500">
        {scripts.error instanceof Error ? scripts.error.message : 'Failed to load init scripts'}
      </p>
    )
  }
  if (scripts.data && !scripts.data.supported) {
    return (
      <div className="mt-6">
        <h2 className="text-lg font-semibold tracking-tight text-gray-900 dark:text-white">Init scripts</h2>
        <p className="mt-2 text-sm text-gray-500 dark:text-gray-400">
          This engine does not run init scripts. PostgreSQL, MySQL, MariaDB, and MongoDB do.
        </p>
      </div>
    )
  }

  const update = (index: number, next: Script) => {
    setDraft(rows.map((row, i) => (i === index ? next : row)))
  }

  return (
    <div className="mt-6 space-y-4">
      <h2 className="text-lg font-semibold tracking-tight text-gray-900 dark:text-white">Init scripts</h2>
      <p className="text-sm text-gray-500 dark:text-gray-400">
        These files are written on the next start and mounted into /docker-entrypoint-initdb.d. The
        image runs them only while the data directory is still empty, in filename order. PostgreSQL,
        MySQL, and MariaDB use .sql or .sh. MongoDB uses .js or .sh.
      </p>
      {rows.map((row, index) => (
        <div key={index} className="panel-card space-y-3 p-4">
          <Input
            label="Filename"
            value={row.filename}
            onChange={(value) => update(index, { ...row, filename: value })}
            required={false}
          />
          <label className="block text-sm">
            <span className="mb-1 block text-gray-500 dark:text-gray-400">Script</span>
            <textarea
              value={row.content}
              onChange={(e) => update(index, { ...row, content: e.target.value })}
              className="panel-field h-36 w-full rounded-lg px-3 py-2 font-mono text-xs"
            />
          </label>
          <Btn
            type="button"
            onClick={() => setDraft(rows.filter((_, i) => i !== index))}
          >
            Remove
          </Btn>
        </div>
      ))}
      <div className="flex flex-wrap gap-2">
        <Btn
          type="button"
          onClick={() => setDraft([...rows, { filename: '', content: '' }])}
        >
          Add script
        </Btn>
        <Btn primary type="button" disabled={save.isPending} onClick={() => save.mutate()}>
          {save.isPending ? 'Saving…' : 'Save init scripts'}
        </Btn>
      </div>
    </div>
  )
}
