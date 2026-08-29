/** Pause polling when the tab is hidden so idle dashboards do not hammer the API. */
export function gentleRefetchInterval(ms: number) {
  return () => {
    if (typeof document !== 'undefined' && document.visibilityState === 'hidden') return false
    return ms
  }
}
