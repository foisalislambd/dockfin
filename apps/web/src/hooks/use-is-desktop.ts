import { useSyncExternalStore } from 'react'

export const DESKTOP_BREAKPOINT = 1024

function getMediaQuery() {
  return window.matchMedia(`(min-width: ${DESKTOP_BREAKPOINT}px)`)
}

function subscribeDesktop(onStoreChange: () => void) {
  const mq = getMediaQuery()
  mq.addEventListener('change', onStoreChange)
  return () => mq.removeEventListener('change', onStoreChange)
}

function getDesktopSnapshot() {
  return getMediaQuery().matches
}

function getServerDesktopSnapshot() {
  return false
}

export function useIsDesktop() {
  return useSyncExternalStore(subscribeDesktop, getDesktopSnapshot, getServerDesktopSnapshot)
}
