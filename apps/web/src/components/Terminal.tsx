import { FitAddon } from '@xterm/addon-fit'
import { Terminal } from '@xterm/xterm'
import { useEffect, useRef, useState } from 'react'
import { api } from '../lib/api'
import { Btn } from '../pages/Servers'
import '@xterm/xterm/css/xterm.css'

type Props = {
  serverId: string
}

export function ServerTerminal({ serverId }: Props) {
  const hostRef = useRef<HTMLDivElement>(null)
  const termRef = useRef<Terminal | null>(null)
  const fitRef = useRef<FitAddon | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const [status, setStatus] = useState<'idle' | 'connecting' | 'open' | 'closed'>('idle')
  const [error, setError] = useState('')
  const [container, setContainer] = useState('')

  const disconnect = () => {
    wsRef.current?.close()
    wsRef.current = null
    termRef.current?.dispose()
    termRef.current = null
    fitRef.current = null
    setStatus('closed')
  }

  const connect = async () => {
    setError('')
    disconnect()
    setStatus('connecting')
    try {
      const { session_id } = await api.createTerminal(serverId, container || undefined)
      if (!hostRef.current) return

      const term = new Terminal({
        cursorBlink: true,
        fontFamily: 'Geist Mono, ui-monospace, monospace',
        fontSize: 13,
        theme: {
          background: '#0b1220',
          foreground: '#e2e8f0',
          cursor: '#94a3b8',
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
        const dims = fit.proposeDimensions()
        if (dims) {
          ws.send(JSON.stringify({ type: 'resize', cols: dims.cols, rows: dims.rows }))
        }
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

      const onResize = () => {
        fit.fit()
        const dims = fit.proposeDimensions()
        if (dims && ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({ type: 'resize', cols: dims.cols, rows: dims.rows }))
        }
      }
      window.addEventListener('resize', onResize)
      return () => window.removeEventListener('resize', onResize)
    } catch (e) {
      setStatus('closed')
      setError(e instanceof Error ? e.message : 'Failed to open terminal')
    }
  }

  useEffect(() => () => disconnect(), [serverId])

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-end gap-3">
        <label className="block min-w-[12rem] flex-1 text-sm">
          <span className="mb-1 block text-gray-500 dark:text-gray-400">
            Container (optional docker exec)
          </span>
          <input
            value={container}
            onChange={(e) => setContainer(e.target.value)}
            placeholder="leave empty for host shell"
            disabled={status === 'open' || status === 'connecting'}
            className="w-full rounded-lg border border-gray-200 bg-white px-3 py-2 font-mono text-sm dark:border-gray-800 dark:bg-gray-900"
          />
        </label>
        {status === 'open' ? (
          <Btn onClick={disconnect}>Disconnect</Btn>
        ) : (
          <Btn primary onClick={() => void connect()}>
            {status === 'connecting' ? 'Connecting…' : 'Connect'}
          </Btn>
        )}
        <span className="pb-2 text-xs text-gray-500 dark:text-gray-400">
          {status === 'open' ? 'connected' : status === 'connecting' ? 'connecting' : 'disconnected'}
        </span>
      </div>
      {error && <p className="text-sm text-error-500">{error}</p>}
      <div
        ref={hostRef}
        className="h-[28rem] overflow-hidden rounded-lg border border-gray-200 bg-[#0b1220] p-2 dark:border-gray-800"
      />
      <p className="text-xs text-gray-500 dark:text-gray-400">
        Interactive SSH PTY over WebSocket. Sessions idle out after 30 minutes.
      </p>
    </div>
  )
}
