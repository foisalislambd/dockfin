import { afterEach, describe, expect, it, vi } from 'vitest'
import { api, ApiError } from './api'

afterEach(() => {
  vi.unstubAllGlobals()
})

function trackUnauthorized() {
  const events: string[] = []
  const onUnauthorized = () => events.push('unauthorized')
  window.addEventListener('dockfin:unauthorized', onUnauthorized)
  return {
    events,
    stop() {
      window.removeEventListener('dockfin:unauthorized', onUnauthorized)
    },
  }
}

function jsonResponse(body: unknown, status: number, statusText: string) {
  return new Response(JSON.stringify(body), { status, statusText })
}

describe('API client error handling', () => {
  it('returns parsed JSON on success and undefined for 204', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      expect(init?.credentials).toBe('include')
      if (url.endsWith('/health')) {
        return new Response(JSON.stringify({ status: 'ok' }), { status: 200, statusText: 'OK' })
      }
      if (url.endsWith('/api/v1/auth/logout')) {
        expect(init?.method).toBe('POST')
        expect(init?.body).toBeUndefined()
        return new Response(null, { status: 204, statusText: 'No Content' })
      }
      expect(new Headers(init?.headers).get('Content-Type')).toBe('application/json')
      return jsonResponse(
        { project: { id: 'p1', name: 'web', description: '' }, environment: { id: 'e1', name: 'production', project_id: 'p1' } },
        201,
        'Created',
      )
    })
    vi.stubGlobal('fetch', fetchMock)

    await expect(api.health()).resolves.toEqual({ status: 'ok' })
    await expect(api.logout()).resolves.toBeUndefined()
    await expect(api.createProject('web')).resolves.toMatchObject({ project: { name: 'web' } })
  })

  it('uses the status text when the error body is empty or not JSON', async () => {
    const tracker = trackUnauthorized()
    try {
      vi.stubGlobal(
        'fetch',
        vi.fn(async () => new Response('', { status: 500, statusText: 'Internal Server Error' })),
      )
      await expect(api.projects()).rejects.toMatchObject({
        status: 500,
        message: 'Internal Server Error',
      })
      expect(tracker.events).toEqual([])

      vi.stubGlobal(
        'fetch',
        vi.fn(async () => new Response('<html>bad gateway</html>', { status: 502, statusText: 'Bad Gateway' })),
      )
      const error = await api.servers().then(
        () => {
          throw new Error('expected servers() to reject')
        },
        (err: unknown) => err,
      )
      expect(error).toBeInstanceOf(ApiError)
      expect(error).toMatchObject({ status: 502, message: 'Bad Gateway' })
      expect(tracker.events).toEqual([])
    } finally {
      tracker.stop()
    }
  })

  it('dispatches unauthorized for a normal 401 and keeps login 401 local', async () => {
    const tracker = trackUnauthorized()
    try {
      vi.stubGlobal(
        'fetch',
        vi.fn(async () => jsonResponse({ error: 'session expired' }, 401, 'Unauthorized')),
      )
      await expect(api.projects()).rejects.toMatchObject({ status: 401, message: 'session expired' })
      expect(tracker.events).toEqual(['unauthorized'])

      tracker.events.length = 0
      await expect(api.login('a@b.co', 'wrong')).rejects.toMatchObject({
        status: 401,
        message: 'session expired',
      })
      await expect(api.me()).rejects.toBeInstanceOf(ApiError)
      expect(tracker.events).toEqual([])

      vi.stubGlobal(
        'fetch',
        vi.fn(async () => new Response('expired', { status: 401, statusText: 'Unauthorized' })),
      )
      await expect(api.teams()).rejects.toMatchObject({ status: 401, message: 'Unauthorized' })
      expect(tracker.events).toEqual(['unauthorized'])
      tracker.events.length = 0
      await expect(api.login('a@b.co', 'wrong')).rejects.toMatchObject({
        status: 401,
        message: 'Unauthorized',
      })
      expect(tracker.events).toEqual([])
    } finally {
      tracker.stop()
    }
  })

  it('surfaces a 409 conflict message and ignores a missing error field', async () => {
    const tracker = trackUnauthorized()
    try {
      vi.stubGlobal(
        'fetch',
        vi.fn(async () =>
          jsonResponse({ error: 'A project with that name already exists' }, 409, 'Conflict'),
        ),
      )
      await expect(api.createProject('web')).rejects.toMatchObject({
        status: 409,
        message: 'A project with that name already exists',
      })
      expect(tracker.events).toEqual([])

      vi.stubGlobal(
        'fetch',
        vi.fn(async () => jsonResponse({ error: '' }, 409, 'Conflict')),
      )
      await expect(api.createProject('')).rejects.toMatchObject({ status: 409, message: 'Conflict' })

      vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(null, 409, 'Conflict')))
      await expect(api.createProject('web')).rejects.toMatchObject({ status: 409, message: 'Conflict' })
      expect(tracker.events).toEqual([])
    } finally {
      tracker.stop()
    }
  })

  it('keeps an abusive error string as text and does not treat a network failure as 401', async () => {
    const tracker = trackUnauthorized()
    try {
      const payload = '<script>alert(1)</script>'
      vi.stubGlobal('fetch', vi.fn(async () => jsonResponse({ error: payload }, 400, 'Bad Request')))
      await expect(api.createProject(payload)).rejects.toMatchObject({ status: 400, message: payload })

      vi.stubGlobal(
        'fetch',
        vi.fn(async () => {
          throw new TypeError('Failed to fetch')
        }),
      )
      await expect(api.projects()).rejects.toThrow('Failed to fetch')
      expect(tracker.events).toEqual([])
    } finally {
      tracker.stop()
    }
  })
})
