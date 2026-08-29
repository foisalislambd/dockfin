import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Eye, EyeOff } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { api, type EnvVar } from '../lib/api'
import { emptyUserEnvVars } from '../lib/env-readiness'
import { useConfirm } from './ConfirmDialog'
import { InfoHint } from './ui/forms'
import { TableSkeleton } from './ui/Skeleton'
import { Btn, Modal } from '../pages/Servers'

type Props = {
  resourceType: string
  resourceId: string
  title?: string
  subtitle?: string
  /** When true, show Production / Preview tabs (applications with preview deploys). */
  previewTabs?: boolean
  /** Sort variables alphabetically by key (Coolify is_env_sorting_enabled). */
  sortByKey?: boolean
}

type Draft = {
  key: string
  value: string
  comment: string
  is_runtime: boolean
  is_buildtime: boolean
  is_multiline: boolean
  is_literal: boolean
  is_build_secret: boolean
}

const emptyDraft = (): Draft => ({
  key: '',
  value: '',
  comment: '',
  is_runtime: true,
  is_buildtime: true,
  is_multiline: false,
  is_literal: false,
  is_build_secret: false,
})

function draftFrom(v: EnvVar): Draft {
  return {
    key: v.key,
    value: v.value ?? '',
    comment: v.comment || '',
    is_runtime: v.is_runtime,
    is_buildtime: v.is_buildtime,
    is_multiline: !!v.is_multiline,
    is_literal: v.is_literal,
    is_build_secret: !!v.is_build_secret,
  }
}

export function EnvVarsPanel({
  resourceType,
  resourceId,
  title = 'Environment Variables',
  subtitle = 'Environment (secrets) variables for this resource.',
  previewTabs = false,
  sortByKey = false,
}: Props) {
  const [scope, setScope] = useState<'production' | 'preview'>('production')
  const isPreview = previewTabs && scope === 'preview'
  const qc = useQueryClient()
  const queryKey = ['env-vars', resourceType, resourceId, isPreview ? 'preview' : 'prod']
  const [addOpen, setAddOpen] = useState(false)
  const vars = useQuery({
    queryKey,
    queryFn: () =>
      api.envVars(resourceType, resourceId, true, previewTabs ? isPreview : undefined),
  })

  const sortedVars = useMemo(() => {
    const list = vars.data?.environment_variables || []
    if (!sortByKey) return list
    return [...list].sort((a, b) => a.key.localeCompare(b.key))
  }, [vars.data?.environment_variables, sortByKey])

  const emptyUser = useMemo(
    () => (isPreview ? [] : emptyUserEnvVars(sortedVars)),
    [isPreview, sortedVars],
  )

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          {title ? (
            <h2 className="text-lg font-semibold text-gray-900 dark:text-white">{title}</h2>
          ) : null}
          {subtitle ? <p className="mt-0.5 text-sm text-gray-500 dark:text-gray-400">{subtitle}</p> : null}
        </div>
        <Btn primary type="button" onClick={() => setAddOpen(true)}>
          + Add Environment Variable
        </Btn>
      </div>

      {previewTabs ? (
        <div className="flex gap-1 border-b border-gray-200 dark:border-gray-800">
          {(['production', 'preview'] as const).map((tab) => (
            <button
              key={tab}
              type="button"
              onClick={() => setScope(tab)}
              className={`px-3 py-2 text-sm font-medium capitalize transition ${
                scope === tab
                  ? 'border-b-2 border-brand-500 text-gray-900 dark:text-white'
                  : 'text-gray-500 hover:text-gray-800 dark:text-gray-400'
              }`}
            >
              {tab}
            </button>
          ))}
        </div>
      ) : null}

      {previewTabs && scope === 'preview' ? (
        <p className="text-xs text-gray-500 dark:text-gray-400">
          Preview variables override production keys for PR preview deployments.
        </p>
      ) : null}

      <div className="space-y-3">
        {vars.isLoading ? (
          <div className="panel-card overflow-hidden">
            <TableSkeleton rows={4} cols={2} />
          </div>
        ) : (
          <>
            {!isPreview && emptyUser.length > 0 ? (
              <div className="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-900 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-100">
                {emptyUser.length === 1
                  ? '1 variable still needs a value. Deploy is blocked until you fill it.'
                  : `${emptyUser.length} variables still need a value. Fill them before deploying.`}
              </div>
            ) : null}
            {sortedVars.map((v) => (
              <EnvVarCard
                key={v.id}
                variable={v}
                resourceType={resourceType}
                resourceId={resourceId}
                isPreview={isPreview}
                needsValue={emptyUser.some((e) => e.id === v.id)}
                onChanged={() => void qc.invalidateQueries({ queryKey })}
              />
            ))}
            {!sortedVars.length && (
              <div className="panel-card px-5 py-10 text-center text-sm text-gray-500 dark:text-gray-400">
                No environment variables yet. Click Add Environment Variable to create one.
              </div>
            )}
          </>
        )}
      </div>

      {addOpen && (
        <AddEnvVarModal
          resourceType={resourceType}
          resourceId={resourceId}
          isPreview={isPreview}
          onClose={() => setAddOpen(false)}
          onSaved={() => {
            setAddOpen(false)
            void qc.invalidateQueries({ queryKey })
          }}
        />
      )}
    </div>
  )
}

function EnvVarCard({
  variable,
  resourceType,
  resourceId,
  isPreview = false,
  needsValue = false,
  onChanged,
}: {
  variable: EnvVar
  resourceType: string
  resourceId: string
  isPreview?: boolean
  needsValue?: boolean
  onChanged: () => void
}) {
  const confirm = useConfirm()
  const [draft, setDraft] = useState(() => draftFrom(variable))
  const [show, setShow] = useState(false)
  const locked = !!variable.is_locked

  useEffect(() => {
    setDraft(draftFrom(variable))
    setShow(false)
  }, [variable])

  const save = useMutation({
    mutationFn: () =>
      api.upsertEnvVar({
        resource_type: resourceType,
        resource_id: resourceId,
        key: draft.key.trim(),
        value: draft.value,
        is_runtime: draft.is_runtime,
        is_buildtime: draft.is_buildtime,
        is_multiline: draft.is_multiline,
        is_literal: draft.is_literal || draft.is_multiline,
        is_build_secret: draft.is_build_secret,
        is_preview: isPreview,
        comment: draft.comment,
      }),
    onSuccess: onChanged,
  })
  const lock = useMutation({
    mutationFn: () => api.lockEnvVar(variable.id, !locked),
    onSuccess: onChanged,
  })
  const del = useMutation({
    mutationFn: () => api.deleteEnvVar(variable.id),
    onSuccess: onChanged,
  })

  const disabled = locked || save.isPending

  return (
    <div
      className={`panel-card space-y-4 p-4 ${locked ? 'opacity-90' : ''} ${
        needsValue ? 'border-amber-300 ring-1 ring-amber-200 dark:border-amber-500/40 dark:ring-amber-500/20' : ''
      }`}
    >
      {needsValue ? (
        <p className="text-xs font-medium text-amber-700 dark:text-amber-300">Needs a value before deploy</p>
      ) : null}
      <div className="grid gap-3 sm:grid-cols-2">
        <label className="block text-sm">
          <span className="mb-1 block text-gray-500 dark:text-gray-400">Name</span>
          <input
            value={draft.key}
            readOnly
            disabled={disabled}
            title="Key cannot be renamed — delete and re-add to change"
            className="panel-field w-full rounded-lg px-3 py-2 font-mono text-sm outline-none disabled:opacity-60"
          />
        </label>
        <label className="block text-sm">
          <span className="mb-1 block text-gray-500 dark:text-gray-400">Value</span>
          <span className="relative block">
            {draft.is_multiline ? (
              show ? (
                <textarea
                  rows={4}
                  value={draft.value}
                  disabled={disabled}
                  onChange={(e) => setDraft({ ...draft, value: e.target.value })}
                  className="panel-field w-full rounded-lg px-3 py-2 pr-10 font-mono text-sm outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-500/20 disabled:opacity-60"
                />
              ) : (
                <input
                  type="password"
                  readOnly
                  value={draft.value ? '••••••••••••••••' : ''}
                  disabled={disabled}
                  className="panel-field w-full rounded-lg py-2 pr-10 pl-3 font-mono text-sm outline-none disabled:opacity-60"
                />
              )
            ) : (
              <input
                type={show ? 'text' : 'password'}
                value={draft.value}
                disabled={disabled}
                autoComplete="off"
                spellCheck={false}
                onChange={(e) => setDraft({ ...draft, value: e.target.value })}
                className="panel-field w-full rounded-lg py-2 pr-10 pl-3 font-mono text-sm outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-500/20 disabled:opacity-60"
              />
            )}
            <button
              type="button"
              className="absolute top-2 right-1.5 inline-flex h-7 w-7 items-center justify-center rounded-md text-gray-400 hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-white/10 dark:hover:text-gray-200"
              aria-label={show ? 'Hide value' : 'Show value'}
              onClick={() => setShow((v) => !v)}
            >
              {show ? <EyeOff className="h-3.5 w-3.5" /> : <Eye className="h-3.5 w-3.5" />}
            </button>
          </span>
        </label>
      </div>

      <label className="block text-sm">
        <span className="mb-1 flex items-center gap-1 text-gray-500 dark:text-gray-400">
          Comment
          <InfoHint text="Optional note for teammates. Not passed into the container." />
        </span>
        <input
          value={draft.comment}
          disabled={disabled}
          onChange={(e) => setDraft({ ...draft, comment: e.target.value })}
          className="panel-field w-full rounded-lg px-3 py-2 text-sm outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-500/20 disabled:opacity-60"
        />
      </label>

      <div className="flex flex-wrap gap-x-5 gap-y-2 text-sm text-gray-700 dark:text-gray-300">
        <Check
          label="Available at Buildtime"
          hint="Injected as a build argument / build secret during image build."
          checked={draft.is_buildtime}
          disabled={disabled}
          onChange={(v) => setDraft({ ...draft, is_buildtime: v })}
        />
        <Check
          label="Is Build Secret?"
          hint="Mount as a Docker build secret instead of a plain build arg."
          checked={draft.is_build_secret}
          disabled={disabled || !draft.is_buildtime}
          onChange={(v) => setDraft({ ...draft, is_build_secret: v })}
        />
        <Check
          label="Available at Runtime"
          hint="Passed into the running container as an environment variable."
          checked={draft.is_runtime}
          disabled={disabled}
          onChange={(v) => setDraft({ ...draft, is_runtime: v })}
        />
        <Check
          label="Is Multiline?"
          hint="Use a textarea for values that span multiple lines."
          checked={draft.is_multiline}
          disabled={disabled}
          onChange={(v) =>
            setDraft({
              ...draft,
              is_multiline: v,
              is_literal: v ? true : draft.is_literal,
            })
          }
        />
        {!draft.is_multiline && (
          <Check
            label="Is Literal?"
            hint="Do not expand shared/variable references in this value."
            checked={draft.is_literal}
            disabled={disabled}
            onChange={(v) => setDraft({ ...draft, is_literal: v })}
          />
        )}
      </div>

      {save.error || lock.error || del.error ? (
        <p className="text-sm text-error-500">
          {(save.error || lock.error || del.error)?.message === 'conflict' ||
          (save.error || lock.error || del.error)?.message?.includes('locked')
            ? 'This variable is locked. Unlock it to edit, or delete and re-add.'
            : (save.error || lock.error || del.error)?.message || 'Request failed'}
        </p>
      ) : null}

      <div className="flex flex-wrap justify-end gap-2">
        <Btn
          type="button"
          primary
          disabled={disabled || !draft.key.trim()}
          onClick={() => save.mutate()}
        >
          {save.isPending ? 'Updating…' : 'Update'}
        </Btn>
        <Btn type="button" disabled={lock.isPending} onClick={() => lock.mutate()}>
          {lock.isPending ? '…' : locked ? 'Unlock' : 'Lock'}
        </Btn>
        <button
          type="button"
          disabled={del.isPending}
          onClick={() => {
            void (async () => {
              if (
                await confirm({
                  title: 'Delete variable',
                  message: `Delete ${variable.key}?`,
                  confirmLabel: 'Delete',
                  danger: true,
                })
              ) {
                del.mutate()
              }
            })()
          }}
          className="inline-flex h-8 items-center rounded-md bg-error-500 px-2.5 text-xs font-medium text-white transition hover:bg-error-500/90 disabled:opacity-50"
        >
          {del.isPending ? 'Deleting…' : 'Delete'}
        </button>
      </div>
    </div>
  )
}

function AddEnvVarModal({
  resourceType,
  resourceId,
  isPreview = false,
  onClose,
  onSaved,
}: {
  resourceType: string
  resourceId: string
  isPreview?: boolean
  onClose: () => void
  onSaved: () => void
}) {
  const [draft, setDraft] = useState(emptyDraft)
  const [show, setShow] = useState(false)
  const add = useMutation({
    mutationFn: () =>
      api.upsertEnvVar({
        resource_type: resourceType,
        resource_id: resourceId,
        key: draft.key.trim(),
        value: draft.value,
        is_runtime: draft.is_runtime,
        is_buildtime: draft.is_buildtime,
        is_multiline: draft.is_multiline,
        is_literal: draft.is_literal || draft.is_multiline,
        is_build_secret: draft.is_build_secret,
        is_preview: isPreview,
        comment: draft.comment,
      }),
    onSuccess: onSaved,
  })

  return (
    <Modal title="Add Environment Variable" onClose={onClose}>
      <form
        className="space-y-3"
        onSubmit={(e) => {
          e.preventDefault()
          add.mutate()
        }}
      >
        <label className="block text-sm">
          <span className="mb-1 block text-gray-500 dark:text-gray-400">Name</span>
          <input
            required
            value={draft.key}
            onChange={(e) => setDraft({ ...draft, key: e.target.value })}
            className="panel-field w-full rounded-lg px-3 py-2 font-mono text-sm outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-500/20"
            placeholder="MY_VARIABLE"
          />
        </label>
        <label className="block text-sm">
          <span className="mb-1 block text-gray-500 dark:text-gray-400">Value</span>
          <span className="relative block">
            {draft.is_multiline ? (
              <textarea
                required
                rows={4}
                value={draft.value}
                onChange={(e) => setDraft({ ...draft, value: e.target.value })}
                className="panel-field w-full rounded-lg px-3 py-2 font-mono text-sm outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-500/20"
              />
            ) : (
              <input
                required
                type={show ? 'text' : 'password'}
                value={draft.value}
                autoComplete="off"
                onChange={(e) => setDraft({ ...draft, value: e.target.value })}
                className="panel-field w-full rounded-lg py-2 pr-10 pl-3 font-mono text-sm outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-500/20"
              />
            )}
            {!draft.is_multiline && (
              <button
                type="button"
                className="absolute top-1/2 right-1.5 inline-flex h-7 w-7 -translate-y-1/2 items-center justify-center rounded-md text-gray-400 hover:bg-gray-100 dark:hover:bg-white/10"
                onClick={() => setShow((v) => !v)}
              >
                {show ? <EyeOff className="h-3.5 w-3.5" /> : <Eye className="h-3.5 w-3.5" />}
              </button>
            )}
          </span>
        </label>
        <label className="block text-sm">
          <span className="mb-1 block text-gray-500 dark:text-gray-400">Comment</span>
          <input
            value={draft.comment}
            onChange={(e) => setDraft({ ...draft, comment: e.target.value })}
            className="panel-field w-full rounded-lg px-3 py-2 text-sm outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-500/20"
          />
        </label>
        <div className="flex flex-wrap gap-x-5 gap-y-2 text-sm text-gray-700 dark:text-gray-300">
          <Check
            label="Available at Buildtime"
            hint="Injected as a build argument / build secret during image build."
            checked={draft.is_buildtime}
            onChange={(v) => setDraft({ ...draft, is_buildtime: v })}
          />
          <Check
            label="Is Build Secret?"
            hint="Mount as a Docker build secret instead of a plain build arg."
            checked={draft.is_build_secret}
            disabled={!draft.is_buildtime}
            onChange={(v) => setDraft({ ...draft, is_build_secret: v })}
          />
          <Check
            label="Available at Runtime"
            hint="Passed into the running container as an environment variable."
            checked={draft.is_runtime}
            onChange={(v) => setDraft({ ...draft, is_runtime: v })}
          />
          <Check
            label="Is Multiline?"
            hint="Use a textarea for values that span multiple lines."
            checked={draft.is_multiline}
            onChange={(v) =>
              setDraft({
                ...draft,
                is_multiline: v,
                is_literal: v ? true : draft.is_literal,
              })
            }
          />
          {!draft.is_multiline && (
            <Check
              label="Is Literal?"
              hint="Do not expand shared/variable references in this value."
              checked={draft.is_literal}
              onChange={(v) => setDraft({ ...draft, is_literal: v })}
            />
          )}
        </div>
        {add.error ? <p className="text-sm text-error-500">{add.error.message}</p> : null}
        <div className="flex justify-end gap-2 pt-2">
          <Btn type="button" onClick={onClose}>
            Cancel
          </Btn>
          <Btn primary type="submit" disabled={add.isPending || !draft.key.trim()}>
            {add.isPending ? 'Saving…' : 'Save'}
          </Btn>
        </div>
      </form>
    </Modal>
  )
}

function Check({
  label,
  hint,
  checked,
  disabled,
  onChange,
}: {
  label: string
  hint?: string
  checked: boolean
  disabled?: boolean
  onChange: (v: boolean) => void
}) {
  return (
    <label className="inline-flex items-center gap-2">
      <input
        type="checkbox"
        className="accent-brand-500"
        checked={checked}
        disabled={disabled}
        onChange={(e) => onChange(e.target.checked)}
      />
      <span className="inline-flex items-center gap-1">
        {label}
        {hint ? <InfoHint text={hint} /> : null}
      </span>
    </label>
  )
}
