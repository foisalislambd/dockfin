import type { EnvVar } from './api'

/** Keys Dockfin fills automatically on prepare/deploy — empty is OK. */
const AUTO_PREFIXES = [
  'SERVICE_URL_',
  'SERVICE_FQDN_',
  'SERVICE_PASSWORD_',
  'SERVICE_USER_',
  'SERVICE_BASE64_',
  'SERVICE_HEX_',
  'SERVICE_REALNAME_',
]

export function isAutoManagedEnvKey(key: string): boolean {
  const k = key.toUpperCase()
  return AUTO_PREFIXES.some((p) => k.startsWith(p))
}

/** User-owned variables that still have no value (templates / compose `${VAR:?…}`). */
export function emptyUserEnvVars(vars: EnvVar[] | undefined): EnvVar[] {
  return (vars || []).filter((v) => {
    if (v.is_preview) return false
    if (isAutoManagedEnvKey(v.key)) return false
    return !String(v.value ?? '').trim()
  })
}

export function formatEmptyEnvToast(vars: EnvVar[]): string {
  const keys = vars.map((v) => v.key)
  const shown = keys.slice(0, 4).join(', ')
  const extra = keys.length > 4 ? ` and ${keys.length - 4} more` : ''
  if (keys.length === 1) {
    return `Fill environment variable ${shown} before deploying.`
  }
  return `Fill ${keys.length} environment variables before deploying: ${shown}${extra}.`
}

/** Gate deploy until production env is known and user-owned keys have values. */
export function deployBlockFromEnv(q: {
  isPending: boolean
  isError: boolean
  data?: { environment_variables?: EnvVar[] }
}): { block: boolean; message?: string; empty: EnvVar[] } {
  if (q.isPending && !q.data) {
    return { block: true, message: 'Still loading environment variables…', empty: [] }
  }
  if (q.isError && !q.data) {
    return { block: true, message: 'Could not load environment variables. Try again.', empty: [] }
  }
  const empty = emptyUserEnvVars(q.data?.environment_variables)
  if (empty.length) {
    return { block: true, message: formatEmptyEnvToast(empty), empty }
  }
  return { block: false, empty: [] }
}
