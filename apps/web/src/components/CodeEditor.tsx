import Editor, { type OnMount } from '@monaco-editor/react'
import { useTheme } from 'next-themes'
import { useEffect, useMemo, useSyncExternalStore } from 'react'

type Props = {
  value: string
  language?: string
  readOnly?: boolean
  onChange?: (value: string) => void
  /** Approximate editor height in CSS (default 22rem / ~16 rows). */
  height?: string
  className?: string
  ariaLabel?: string
}

/**
 * Coolify-style Monaco viewer/editor for YAML (compose), Dockerfile, etc.
 * Theme follows Dockfin light/dark (next-themes).
 */
export function CodeEditor({
  value,
  language = 'yaml',
  readOnly = true,
  onChange,
  height = '22rem',
  className = '',
  ariaLabel,
}: Props) {
  const { resolvedTheme } = useTheme()
  const mounted = useSyncExternalStore(
    () => () => {},
    () => true,
    () => false,
  )

  const options = useMemo(
    () => ({
      readOnly,
      minimap: { enabled: false },
      scrollBeyondLastLine: false,
      fontSize: 13,
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
      lineNumbers: 'on' as const,
      wordWrap: 'on' as const,
      wrappingIndent: 'indent' as const,
      automaticLayout: true,
      tabSize: 2,
      renderLineHighlight: readOnly ? ('none' as const) : ('line' as const),
      scrollbar: { verticalScrollbarSize: 10, horizontalScrollbarSize: 10 },
      padding: { top: 12, bottom: 12 },
      domReadOnly: readOnly,
      contextmenu: !readOnly,
      folding: true,
      glyphMargin: false,
      overviewRulerLanes: 0,
      hideCursorInOverviewRuler: true,
      overviewRulerBorder: false,
    }),
    [readOnly],
  )

  const monacoTheme =
    mounted && resolvedTheme === 'dark' ? 'dockfin-dark' : 'dockfin-light'

  useEffect(() => {
    const monaco = (window as unknown as { monaco?: { editor: { setTheme: (t: string) => void } } })
      .monaco
    monaco?.editor?.setTheme(monacoTheme)
  }, [monacoTheme])

  const handleMount: OnMount = (editor, monaco) => {
    monaco.editor.defineTheme('dockfin-dark', {
      base: 'vs-dark',
      inherit: true,
      rules: [],
      colors: {
        'editor.background': '#0b0f14',
        'editor.lineHighlightBackground': '#111827',
        'editorLineNumber.foreground': '#4b5563',
        'editorLineNumber.activeForeground': '#9ca3af',
      },
    })
    monaco.editor.defineTheme('dockfin-light', {
      base: 'vs',
      inherit: true,
      rules: [],
      colors: {
        'editor.background': '#f9fafb',
        'editor.lineHighlightBackground': '#f3f4f6',
        'editorLineNumber.foreground': '#9ca3af',
        'editorLineNumber.activeForeground': '#4b5563',
      },
    })
    monaco.editor.setTheme(monacoTheme)
    editor.updateOptions(options)
  }

  return (
    <div
      className={`overflow-hidden rounded-lg border border-gray-200 dark:border-gray-800 ${className}`}
      style={{ height }}
      role="region"
      aria-label={ariaLabel}
    >
      <Editor
        height="100%"
        language={language}
        theme={monacoTheme}
        value={value}
        options={options}
        onMount={handleMount}
        onChange={(v) => onChange?.(v ?? '')}
        loading={
          <div className="flex h-full items-center justify-center bg-gray-50 text-sm text-gray-500 dark:bg-gray-950 dark:text-gray-400">
            Loading editor…
          </div>
        }
      />
    </div>
  )
}
