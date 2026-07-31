import { useEffect, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api } from '../lib/api'
import { Btn, Modal } from '../pages/Servers'

export type DeletePayload = {
  confirmation_name: string
  password?: string
  delete_volumes?: boolean
  delete_configurations?: boolean
  delete_networks?: boolean
  docker_cleanup?: boolean
}

type CheckboxKey = 'delete_volumes' | 'delete_configurations' | 'delete_networks' | 'docker_cleanup'

const RESOURCE_CHECKBOXES: { id: CheckboxKey; label: string }[] = [
  { id: 'delete_volumes', label: 'Permanently delete all volumes associated with this resource.' },
  {
    id: 'delete_networks',
    label: 'Permanently delete all non-predefined networks associated with this resource.',
  },
  {
    id: 'delete_configurations',
    label: 'Permanently delete all configuration files from the server.',
  },
  { id: 'docker_cleanup', label: 'Run Docker Cleanup (remove unused images and builder cache).' },
]

export function DangerZoneCard({
  title = 'Danger Zone',
  subtitle = 'Woah. I hope you know what you are doing.',
  children,
}: {
  title?: string
  subtitle?: string
  children: React.ReactNode
}) {
  return (
    <div className="panel-card space-y-4 border-error-200 p-5 dark:border-error-500/30">
      <div>
        <h2 className="text-lg font-medium text-error-500">{title}</h2>
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">{subtitle}</p>
      </div>
      {children}
    </div>
  )
}

export function DangerConfirmModal({
  open,
  onClose,
  title,
  resourceLabel = 'Resource Name',
  expectedName,
  actions,
  statusLine,
  requirePassword = true,
  showResourceCheckboxes = false,
  confirmButtonLabel = 'Delete',
  busy,
  error,
  onConfirm,
}: {
  open: boolean
  onClose: () => void
  title: string
  resourceLabel?: string
  expectedName: string
  actions: string[]
  statusLine?: string
  requirePassword?: boolean
  showResourceCheckboxes?: boolean
  confirmButtonLabel?: string
  busy?: boolean
  error?: string
  onConfirm: (payload: DeletePayload) => void
}) {
  const settings = useQuery({
    queryKey: ['instance-settings'],
    queryFn: api.instanceSettings,
    enabled: open,
  })
  const settingsReady = !settings.isLoading
  const skipTwoStep = !!settings.data?.settings.disable_two_step_confirmation

  const [step, setStep] = useState<'options' | 'confirm'>('confirm')
  const [confirmName, setConfirmName] = useState('')
  const [password, setPassword] = useState('')
  const [checks, setChecks] = useState<Record<CheckboxKey, boolean>>({
    delete_volumes: true,
    delete_configurations: true,
    delete_networks: true,
    docker_cleanup: true,
  })

  useEffect(() => {
    if (!open) return
    setStep(showResourceCheckboxes ? 'options' : 'confirm')
    setConfirmName('')
    setPassword('')
    setChecks({
      delete_volumes: true,
      delete_configurations: true,
      delete_networks: true,
      docker_cleanup: true,
    })
  }, [open, showResourceCheckboxes])

  if (!open) return null

  const needsPassword = requirePassword && !skipTwoStep
  const needsName = !skipTwoStep
  const nameMatches = confirmName.trim() === expectedName

  const submit = () => {
    if (!settingsReady || busy) return
    if (needsName && !nameMatches) return
    if (needsPassword && password.length < 1) return
    onConfirm({
      confirmation_name: skipTwoStep ? expectedName : confirmName.trim(),
      password: needsPassword ? password : undefined,
      delete_volumes: checks.delete_volumes,
      delete_configurations: checks.delete_configurations,
      delete_networks: checks.delete_networks,
      docker_cleanup: checks.docker_cleanup,
    })
  }

  const confirmDisabled =
    !settingsReady ||
    !!busy ||
    (needsName && !nameMatches) ||
    (needsPassword && password.length < 1)

  return (
    <Modal title={title} onClose={onClose}>
      <div className="max-h-[80vh] space-y-4 overflow-y-auto">
        {statusLine && (
          <p className="rounded-md border border-amber-500/30 bg-amber-500/10 px-3 py-2 text-sm text-amber-700 dark:text-amber-300">
            {statusLine}
          </p>
        )}

        {step === 'options' && showResourceCheckboxes && (
          <>
            <p className="text-sm text-gray-600 dark:text-gray-300">
              Choose what else should be cleaned up when this resource is deleted.
            </p>
            <div className="space-y-2">
              {RESOURCE_CHECKBOXES.map((c) => (
                <label
                  key={c.id}
                  className="flex items-start gap-2 text-sm text-gray-700 dark:text-gray-200"
                >
                  <input
                    type="checkbox"
                    className="mt-0.5 accent-[var(--color-accent)]"
                    checked={checks[c.id]}
                    onChange={(e) => setChecks({ ...checks, [c.id]: e.target.checked })}
                  />
                  <span>{c.label}</span>
                </label>
              ))}
            </div>
            <div className="flex gap-2">
              <Btn
                primary
                type="button"
                disabled={!settingsReady || !!busy}
                onClick={() => {
                  if (skipTwoStep) submit()
                  else setStep('confirm')
                }}
              >
                {skipTwoStep
                  ? busy
                    ? 'Deleting…'
                    : !settingsReady
                      ? 'Loading…'
                      : confirmButtonLabel
                  : !settingsReady
                    ? 'Loading…'
                    : 'Continue'}
              </Btn>
              <Btn type="button" onClick={onClose}>
                Cancel
              </Btn>
            </div>
            {error && <p className="text-sm text-error-500">{error}</p>}
          </>
        )}

        {step === 'confirm' && (
          <>
            <div className="rounded-md border border-error-200 bg-error-50/50 p-3 dark:border-error-500/30 dark:bg-error-500/5">
              <p className="text-sm font-medium text-error-600 dark:text-error-400">
                This operation is permanent and cannot be undone.
              </p>
              <ul className="mt-2 list-disc space-y-1 pl-5 text-sm text-gray-600 dark:text-gray-300">
                {actions.map((a) => (
                  <li key={a}>{a}</li>
                ))}
              </ul>
            </div>

            {needsName && (
              <label className="block text-sm">
                <span className="mb-1 block text-gray-500 dark:text-gray-400">
                  Type{' '}
                  <span className="font-semibold text-gray-900 dark:text-white">{expectedName}</span>{' '}
                  ({resourceLabel}) to confirm
                </span>
                <input
                  value={confirmName}
                  onChange={(e) => setConfirmName(e.target.value)}
                  autoComplete="off"
                  className="panel-field w-full rounded-lg px-3 py-2 outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-500/20"
                  placeholder={resourceLabel}
                />
              </label>
            )}

            {needsPassword && (
              <label className="block text-sm">
                <span className="mb-1 block text-gray-500 dark:text-gray-400">
                  Your account password
                </span>
                <input
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  autoComplete="current-password"
                  className="panel-field w-full rounded-lg px-3 py-2 outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-500/20"
                  placeholder="Password"
                />
              </label>
            )}

            {error && <p className="text-sm text-error-500">{error}</p>}

            <div className="flex flex-wrap gap-2">
              {showResourceCheckboxes && (
                <Btn type="button" onClick={() => setStep('options')}>
                  Back
                </Btn>
              )}
              <Btn type="button" disabled={confirmDisabled} onClick={submit}>
                {busy ? 'Deleting…' : !settingsReady ? 'Loading…' : confirmButtonLabel}
              </Btn>
              <Btn type="button" onClick={onClose}>
                Cancel
              </Btn>
            </div>
          </>
        )}
      </div>
    </Modal>
  )
}
