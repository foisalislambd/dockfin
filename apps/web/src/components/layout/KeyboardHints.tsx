import { useConsoleUi } from './console-ui'

export function KeyboardHints() {
  const { setPaletteOpen, setShortcutsOpen } = useConsoleUi()
  return (
    <div className="sticky bottom-0 z-20 hidden shrink-0 items-center gap-4 border-t border-gray-200 bg-white/95 px-4 py-1.5 text-[11px] text-gray-400 backdrop-blur sm:flex dark:border-gray-800 dark:bg-gray-900/95">
      <button type="button" className="hover:text-gray-700 dark:hover:text-gray-200" onClick={() => setPaletteOpen(true)}>
        Search <kbd className="font-mono">⌘K</kbd>
      </button>
      <span>
        Go to <kbd className="font-mono">g</kbd> then a letter
      </span>
      <button type="button" className="hover:text-gray-700 dark:hover:text-gray-200" onClick={() => setShortcutsOpen(true)}>
        Shortcuts <kbd className="font-mono">?</kbd>
      </button>
    </div>
  )
}
