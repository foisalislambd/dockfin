/** Keys that should stay masked in the Environment Variables UI. */
export function isSecretEnvKey(key: string): boolean {
  const k = key.trim().toUpperCase()
  if (!k) return false
  if (
    k.startsWith('SERVICE_PASSWORD_') ||
    k.startsWith('SERVICE_BASE64_') ||
    k.startsWith('SERVICE_HEX_') ||
    k.startsWith('SERVICE_USER_')
  ) {
    return true
  }
  // URL / FQDN pairs are public.
  if (k.startsWith('SERVICE_URL_') || k.startsWith('SERVICE_FQDN_')) {
    return false
  }
  return (
    k.includes('PASSWORD') ||
    k.includes('SECRET') ||
    k.includes('TOKEN') ||
    k.includes('PRIVATE') ||
    k.includes('CREDENTIAL') ||
    k.includes('PASSPHRASE') ||
    /(^|_)(API[_-]?KEY|ACCESS[_-]?KEY|AUTH[_-]?KEY|PRIVATE[_-]?KEY)(_|$)/.test(k) ||
    /(^|_)(AWS_SECRET|CLIENT_SECRET|WEBHOOK_SECRET)(_|$)/.test(k)
  )
}
