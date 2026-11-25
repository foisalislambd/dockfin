import { afterEach, describe, expect, it, vi } from 'vitest'
import { api, ApiError } from './api'
import { safeExternalHref } from './url'

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('request errors', () => {
  it('uses the JSON error and does not treat a login 401 as a session expiry', async () => {
    const events: string[] = []
    window.addEventListener('dockfin:unauthorized', () => events.push('unauthorized'))
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => new Response(JSON.stringify({ error: 'invalid credentials' }), { status: 401, statusText: 'Unauthorized' })),
    )
    await expect(api.login('a@b.co', 'nope')).rejects.toMatchObject({ status: 401, message: 'invalid credentials' })
    expect(events).toEqual([])
  })

  it('dispatches unauthorized for a signed-in call that 401s', async () => {
    const events: string[] = []
    window.addEventListener('dockfin:unauthorized', () => events.push('unauthorized'))
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => new Response(JSON.stringify({ error: 'unauthorized' }), { status: 401 })),
    )
    await expect(api.projects()).rejects.toBeInstanceOf(ApiError)
    expect(events).toEqual(['unauthorized'])
  })

  it('falls back to the status text when the body is not JSON', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response('nope', { status: 502, statusText: 'Bad Gateway' })))
    await expect(api.projects()).rejects.toMatchObject({ status: 502, message: 'Bad Gateway' })
  })
})

describe('safeExternalHref', () => {
  it('allows http(s) and drops other schemes', () => {
    expect(safeExternalHref('https://example.com/a')).toBe('https://example.com/a')
    expect(safeExternalHref(' javascript:alert(1)')).toBeUndefined()
    expect(safeExternalHref('')).toBeUndefined()
  })
})
