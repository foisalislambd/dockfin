/** Allow only http(s) URLs in user-controlled hrefs. */
export function safeExternalHref(url: string | undefined | null): string | undefined {
  const t = (url || '').trim()
  if (!t) return undefined
  const lower = t.toLowerCase()
  if (lower.startsWith('https://') || lower.startsWith('http://')) {
    return t
  }
  return undefined
}
