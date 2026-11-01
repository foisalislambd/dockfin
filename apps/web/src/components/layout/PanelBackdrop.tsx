import { useSidebar } from '../../context/sidebar-context'
import { useEffect } from 'react'

export function PanelBackdrop() {
  const { isMobileOpen, isDesktop, closeMobileSidebar } = useSidebar()

  useEffect(() => {
    if (!isMobileOpen || isDesktop) return
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape') closeMobileSidebar()
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [isMobileOpen, isDesktop, closeMobileSidebar])

  if (!isMobileOpen || isDesktop) return null

  return (
    <button
      type="button"
      className="fixed top-[3.25rem] right-0 bottom-0 left-0 z-30 bg-gray-900/60 backdrop-blur-[2px] lg:hidden"
      onClick={closeMobileSidebar}
      aria-label="Close navigation"
    />
  )
}
