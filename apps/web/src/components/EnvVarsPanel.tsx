import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { api } from '../lib/api'
import { isSecretEnvKey } from '../lib/secrets'
import { Btn, Input } from '../pages/Servers'
import { EnvSecretCell, SecretInput } from './SecretValue'

type Props = {
  resourceType: string
  resourceId: string
  title?: string
}

export function EnvVarsPanel({ resourceType, resourceId, title = 'Environment Variables' }: Props) {
  const qc = useQueryClient()
  const [key, setKey] = useState('')
  const [value, setValue] = useState('')
  const queryKey = ['env-vars', resourceType, resourceId]
  const vars = useQuery({
    queryKey,
    queryFn: () => api.envVars(resourceType, resourceId, true),
  })
  const add = useMutation({
    mutationFn: () =>
      api.upsertEnvVar({
        resource_type: resourceType,
        resource_id: resourceId,
        key,
        value,
      }),
    onSuccess: () => {
      setKey('')
      setValue('')
      void qc.invalidateQueries({ queryKey })
    },
  })
  const del = useMutation({
    mutationFn: (id: string) => api.deleteEnvVar(id),
    onSuccess: () => void qc.invalidateQueries({ queryKey }),
  })

  const valueIsSecret = isSecretEnvKey(key)

  return (
    <div className="space-y-4">
      {title ? (
        <h2 className="text-lg font-semibold text-gray-900 dark:text-white">{title}</h2>
      ) : null}
      <div className="panel-card overflow-hidden">
        <table className="panel-table">
          <thead>
            <tr>
              <th>Key</th>
              <th>Value</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {(vars.data?.environment_variables || []).map((v) => (
              <tr key={v.id}>
                <td className="font-mono text-xs">
                  <span className="text-gray-900 dark:text-gray-100">{v.key}</span>
                </td>
                <td>
                  <EnvSecretCell envKey={v.key} value={v.value} />
                </td>
                <td>
                  <button
                    type="button"
                    className="text-error-500 hover:underline"
                    onClick={() => del.mutate(v.id)}
                  >
                    Delete
                  </button>
                </td>
              </tr>
            ))}
            {!vars.data?.environment_variables?.length && (
              <tr>
                <td colSpan={3} className="panel-table-empty">
                  No env vars yet.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
      <form
        className="flex flex-wrap items-end gap-3"
        onSubmit={(e) => {
          e.preventDefault()
          add.mutate()
        }}
      >
        <div className="min-w-[140px] flex-1">
          <Input label="Key" value={key} onChange={setKey} />
        </div>
        <div className="min-w-[180px] flex-1">
          {valueIsSecret ? (
            <SecretInput label="Value" value={value} onChange={setValue} required />
          ) : (
            <Input label="Value" value={value} onChange={setValue} />
          )}
        </div>
        <Btn primary type="submit" disabled={add.isPending || !key.trim()}>
          {add.isPending ? 'Adding…' : 'Add'}
        </Btn>
        {add.error ? <p className="w-full text-sm text-error-500">{add.error.message}</p> : null}
      </form>
    </div>
  )
}
