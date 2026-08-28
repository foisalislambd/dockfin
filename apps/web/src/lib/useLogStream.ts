import { useEffect, useState } from 'react'

export type LogStreamStatus = 'connecting' | 'live' | 'ended' | 'error'

function parseLogData(raw: string): string {
  try {
    const data = JSON.parse(raw) as { line?: string }
    return data.line ?? raw
  } catch {
    return raw
  }
}

function httpErrorMessage(status: number, body: string): string {
  const trimmed = body.trim()
  try {
    const data = JSON.parse(trimmed) as { error?: string; message?: string }
    return data.error || data.message || trimmed || `Log stream failed (${status})`
  } catch {
    return trimmed || `Log stream failed (${status})`
  }
}

function consumeSse(buffer: string, onEvent: (event: string, data: string) => void): string {
  let rest = buffer.replace(/\r\n/g, '\n')
  for (;;) {
    const split = rest.indexOf('\n\n')
    if (split < 0) return rest
    const block = rest.slice(0, split)
    rest = rest.slice(split + 2)
    let event = 'message'
    const dataLines: string[] = []
    for (const line of block.split('\n')) {
      if (line.startsWith('event:')) event = line.slice(6).trim()
      else if (line.startsWith('data:')) dataLines.push(line.replace(/^data:\s?/, ''))
    }
    if (dataLines.length) onEvent(event, dataLines.join('\n'))
  }
}

/** Live docker logs over SSE via fetch (EventSource onerror closed the stream before the first line). */
export function useLogStream(url: string | null) {
  const [lines, setLines] = useState<string[]>([])
  const [status, setStatus] = useState<LogStreamStatus>('connecting')
  const [error, setError] = useState('')
  const [nonce, setNonce] = useState(0)

  useEffect(() => {
    if (!url) {
      setLines([])
      setStatus('connecting')
      setError('')
      return
    }

    const ac = new AbortController()
    let opened = false
    setLines([])
    setStatus('connecting')
    setError('')

    const run = async () => {
      try {
        const res = await fetch(url, {
          credentials: 'include',
          headers: { Accept: 'text/event-stream' },
          signal: ac.signal,
        })
        if (!res.ok) {
          const body = await res.text().catch(() => '')
          if (ac.signal.aborted) return
          setStatus('error')
          setError(httpErrorMessage(res.status, body))
          return
        }
        if (!res.body) {
          setStatus('error')
          setError('Log stream is empty')
          return
        }

        const reader = res.body.pipeThrough(new TextDecoderStream()).getReader()
        let buf = ''
        const onEvent = (event: string, data: string) => {
          if (event === 'ping') return
          if (event === 'meta') {
            opened = true
            setStatus((s) => (s === 'error' ? s : 'live'))
            return
          }
          if (event === 'done') {
            setStatus((s) => (s === 'error' ? s : 'ended'))
            return
          }
          if (event === 'error') {
            setError(parseLogData(data) || 'Stream error')
            setStatus('error')
            return
          }
          if (event === 'log' || event === 'message') {
            opened = true
            const line = parseLogData(data)
            setLines((prev) => [...prev.slice(-2000), line])
            setStatus('live')
          }
        }

        while (!ac.signal.aborted) {
          const { value, done } = await reader.read()
          if (done) break
          buf = consumeSse(buf + value, onEvent)
        }
        if (ac.signal.aborted) return
        if (!opened) {
          setStatus('error')
          setError((e) => e || 'Stream ended before any log lines')
          return
        }
        setStatus((s) => (s === 'error' ? s : 'ended'))
      } catch (e) {
        if (ac.signal.aborted) return
        setStatus('error')
        setError(e instanceof Error ? e.message : 'Stream disconnected')
      }
    }

    void run()
    return () => ac.abort()
  }, [url, nonce])

  return {
    lines,
    status,
    error,
    reconnect: () => setNonce((n) => n + 1),
  }
}
