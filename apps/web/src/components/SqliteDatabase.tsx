import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { api, type Database } from '../lib/api'
import { Btn } from './ui/Button'
import { Input } from './ui/Input'

export function SqliteDatabaseCard({ db }: { db: Database }) {
  const qc = useQueryClient()
  const [files, setFiles] = useState('database.sqlite')
  const [appId, setAppId] = useState('')
  const [note, setNote] = useState('')
  const [compose, setCompose] = useState('')
  useEffect(() => {
    const saved = db.engine_config?.sqlite_databases
    setFiles(saved && saved.trim() ? saved : 'database.sqlite')
  }, [db.engine_config])

  const apps = useQuery({
    queryKey: ['applications', db.environment_id],
    queryFn: () => api.applications(db.environment_id),
  })
  const save = useMutation({
    mutationFn: () => api.updateDatabase(db.id, { sqlite_databases: files }),
    onSuccess: () => {
      setNote('File names saved. Restart the database to create new files.')
      setCompose('')
      void qc.invalidateQueries({ queryKey: ['database', db.id] })
    },
  })
  const connect = useMutation({
    mutationFn: () => api.connectSqliteApplication(db.id, appId),
    onSuccess: (res) => {
      if (res.status === 'compose' && res.compose) {
        setCompose(res.compose)
        setNote('Add these lines to the compose service, then redeploy it.')
        return
      }
      setCompose('')
      setNote('Mounted. Redeploy the application so it can open the file.')
    },
  })

  return (
    <div className="mt-6 panel-card space-y-4 p-5">
      <h3 className="text-sm font-semibold text-gray-900 dark:text-white">SQLite files</h3>
      <p className="text-sm text-gray-500 dark:text-gray-400">
        SQLite is a file, not a network port. The files live in the database directory and are mounted into an
        application on the same server at <code>/var/lib/sqlite</code>.
      </p>
      <Input label="File names" value={files} onChange={setFiles} />
      <p className="text-xs text-gray-500 dark:text-gray-400">
        Comma-separated names such as <code>database.sqlite</code> or <code>app.db,jobs.db</code>. At most 8.
      </p>
      <Btn primary disabled={save.isPending} onClick={() => save.mutate()}>
        {save.isPending ? 'Saving…' : 'Save file names'}
      </Btn>
      {save.error && <p className="text-sm text-error-500">{save.error.message}</p>}

      <div className="space-y-2 border-t border-gray-200 pt-4 dark:border-gray-800">
        <label className="block text-sm font-medium text-gray-900 dark:text-white">Mount into an application</label>
        <select
          className="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-gray-700 dark:bg-gray-900"
          value={appId}
          onChange={(e) => setAppId(e.target.value)}
        >
          <option value="">Select an application</option>
          {(apps.data?.applications || []).map((app) => (
            <option key={app.id} value={app.id}>
              {app.name}
            </option>
          ))}
        </select>
        <Btn disabled={!appId || connect.isPending} onClick={() => connect.mutate()}>
          {connect.isPending ? 'Mounting…' : 'Mount'}
        </Btn>
        {connect.error && <p className="text-sm text-error-500">{connect.error.message}</p>}
        {note && <p className="text-sm text-gray-600 dark:text-gray-300">{note}</p>}
        {compose && (
          <pre className="overflow-x-auto rounded-lg bg-gray-950 p-3 text-xs text-gray-100">{compose}</pre>
        )}
      </div>
    </div>
  )
}
