import { FitAddon } from '@xterm/addon-fit'
import { Terminal } from '@xterm/xterm'
import { useEffect, useRef, useState } from 'react'
import { api } from '../lib/api'
import { Btn } from '../pages/Servers'
import '@xterm/xterm/css/xterm.css'

type Props = {
  serverId: string
  /** Pre-select docker exec target (full container name or compose unit). */
  defaultContainer?: string
  /** Optional dropdown of container names. */
  containerOptions?: string[]
  hideHostShell?: boolean
  /** Grow to fill the parent instead of a fixed 28rem pane. */
  fill?: boolean
}

export function ServerTerminal({
  serverId,
  defaultContainer = '',
  containerOptions,
  hideHostShell = false,
  fill = false,
}: Props) {
  const hostRef = useRef<HTMLDivElement>(null)
  const termRef = useRef<Terminal | null>(null)
  const fitRef = useRef<FitAddon | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const [status, setStatus] = useState<'idle' | 'connecting' | 'open' | 'closed'>('idle')
  const [error, setError] = useState('')
  const [container, setContainer] = useState(defaultContainer)

  useEffect(() => {
    setContainer(defaultContainer)
  }, [defaultContainer, serverId])

  const disconnect = () => {
    wsRef.current?.close()
    wsRef.current = null
    termRef.current?.dispose()
    termRef.current = null
    fitRef.current = null
    setStatus('closed')
  }

  const sendResize = () => {
    const fit = fitRef.current
    const ws = wsRef.current
    if (!fit) return
    fit.fit()
    const dims = fit.proposeDimensions()
    if (dims && ws?.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'resize', cols: dims.cols, rows: dims.rows }))
    }
  }

  const connect = async () => {
    setError('')
    disconnect()
    setStatus('connecting')
    try {
      const { session_id } = await api.createTerminal(serverId, container || undefined)
      if (!hostRef.current) return

      const compact = typeof window !== 'undefined' && window.matchMedia('(max-width: 639px)').matches
      const term = new Terminal({
        cursorBlink: true,
        fontFamily: 'Geist Mono, ui-monospace, SFMono-Regular, Menlo, monospace',
        fontSize: compact ? 12 : 13,
        lineHeight: 1.25,
        theme: {
          background: '#0b1220',
          foreground: '#e2e8f0',
          cursor: '#94a3b8',
          selectionBackground: '#334155',
        },
      })
      const fit = new FitAddon()
      term.loadAddon(fit)
      term.open(hostRef.current)
      fit.fit()
      termRef.current = term
      fitRef.current = fit

      const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      const ws = new WebSocket(`${proto}//${window.location.host}/api/v1/terminal/ws/${session_id}`)
      wsRef.current = ws

      ws.onopen = () => {
        setStatus('open')
        requestAnimationFrame(sendResize)
        term.focus()
      }
      ws.onmessage = (ev) => {
        try {
          const msg = JSON.parse(String(ev.data)) as { type?: string; data?: string }
          if (msg.type === 'stdout' && msg.data) term.write(msg.data)
        } catch {
          term.write(String(ev.data))
        }
      }
      ws.onerror = () => setError('WebSocket error')
      ws.onclose = () => setStatus('closed')

      term.onData((data) => {
        if (ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({ type: 'stdin', data }))
        }
      })
    } catch (e) {
      setStatus('closed')
      setError(e instanceof Error ? e.message : 'Failed to open terminal')
    }
  }

  useEffect(() => () => disconnect(), [serverId])

  useEffect(() => {
    const el = hostRef.current
    if (!el) return
    const ro = new ResizeObserver(() => sendResize())
    ro.observe(el)
    return () => ro.disconnect()
  }, [status, fill])

  const locked = status === 'open' || status === 'connecting'

  const toolbar = (
    <div
      className={
        fill
          ? 'flex flex-col gap-2 border-b border-white/10 bg-[#111827] px-3 py-2.5 sm:flex-row sm:flex-wrap sm:items-center sm:gap-3'
          : 'flex flex-wrap items-end gap-3'
      }
    >
      {containerOptions && containerOptions.length > 0 ? (
        <label className={`block min-w-0 text-sm ${fill ? 'w-full sm:min-w-[12rem] sm:flex-1' : 'min-w-[12rem] flex-1'}`}>
          {!fill && <span className="mb-1 block text-gray-500 dark:text-gray-400">Container</span>}
          {fill && (
            <span className="mb-1 block text-[11px] font-medium tracking-wide text-slate-400 uppercase sm:sr-only">
              Target
            </span>
          )}
          <select
            value={container}
            onChange={(e) => setContainer(e.target.value)}
            disabled={locked}
            className={
              fill
                ? 'h-9 w-full rounded-md border border-white/10 bg-[#0b1220] px-2.5 font-mono text-xs text-slate-200'
                : 'panel-field w-full rounded-lg px-3 py-2 font-mono text-sm'
            }
          >
            {!hideHostShell && <option value="">Host shell</option>}
            {containerOptions.map((c) => (
              <option key={c} value={c}>
                {c}
              </option>
            ))}
          </select>
        </label>
      ) : (
        <label className={`block min-w-0 text-sm ${fill ? 'w-full sm:min-w-[12rem] sm:flex-1' : 'min-w-[12rem] flex-1'}`}>
          {!fill && (
            <span className="mb-1 block text-gray-500 dark:text-gray-400">
              Container (optional docker exec)
            </span>
          )}
          {fill && (
            <span className="mb-1 block text-[11px] font-medium tracking-wide text-slate-400 uppercase sm:sr-only">
              Container
            </span>
          )}
          <input
            value={container}
            onChange={(e) => setContainer(e.target.value)}
            placeholder={hideHostShell ? 'container name required' : 'Host shell (or container name)'}
            disabled={locked}
            className={
              fill
                ? 'h-9 w-full rounded-md border border-white/10 bg-[#0b1220] px-2.5 font-mono text-xs text-slate-200 placeholder:text-slate-500'
                : 'panel-field w-full rounded-lg px-3 py-2 font-mono text-sm'
            }
          />
        </label>
      )}
      <div className="flex shrink-0 items-center">
        {status === 'open' ? (
          <Btn onClick={disconnect}>Disconnect</Btn>
        ) : (
          <Btn primary onClick={() => void connect()} disabled={hideHostShell && !container}>
            {status === 'connecting' ? 'Connecting…' : 'Connect'}
          </Btn>
        )}
      </div>
    </div>
  )

  const pane = (
    <div
      ref={hostRef}
      className={
        fill
          ? 'dockfin-xterm relative min-h-[14rem] flex-1 overflow-hidden bg-[#0b1220] px-2 pt-2 sm:min-h-0'
          : 'dockfin-xterm h-[28rem] overflow-hidden rounded-lg border border-gray-200 bg-[#0b1220] p-2 dark:border-gray-800'
      }
    />
  )

  if (fill) {
    return (
      <div className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden rounded-xl border border-gray-200 bg-[#0b1220] shadow-sm dark:border-gray-800">
        {toolbar}
        {error ? <p className="px-3 py-2 text-sm text-error-400">{error}</p> : null}
        {pane}
      </div>
    )
  }

  return (
    <div className="space-y-3">
      {toolbar}
      {error && <p className="text-sm text-error-500">{error}</p>}
      {pane}
      <p className="text-xs text-gray-500 dark:text-gray-400">
        Interactive SSH PTY over WebSocket. Sessions idle out after 30 minutes.
      </p>
    </div>
  )
}
