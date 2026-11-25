import { describe, expect, it } from 'vitest'
import type { EnvVar } from './api'
import { deployBlockFromEnv, emptyUserEnvVars, formatEmptyEnvToast, isAutoManagedEnvKey } from './env-readiness'
import { isSecretEnvKey } from './secrets'

function env(key: string, value?: string, extra?: Partial<EnvVar>): EnvVar {
  return {
    id: key,
    key,
    value,
    is_runtime: true,
    is_buildtime: false,
    is_literal: true,
    comment: '',
    ...extra,
  }
}

describe('environment variable readiness', () => {
  it('treats generated service keys as filled and names the real gaps', () => {
    expect(isAutoManagedEnvKey('service_url_n8n')).toBe(true)
    expect(isAutoManagedEnvKey('SERVICE_FQDN_APP')).toBe(true)
    const empty = emptyUserEnvVars([
      env('SERVICE_URL_N8N', ''),
      env('DATABASE_URL', 'postgres://db'),
      env('APP_SECRET', ''),
    ])
    expect(empty.map((item) => item.key)).toEqual(['APP_SECRET'])
    expect(formatEmptyEnvToast(empty)).toBe('Fill environment variable APP_SECRET before deploying.')
    expect(
      deployBlockFromEnv({
        isPending: false,
        isError: false,
        data: { environment_variables: [env('APP_SECRET', 'set'), env('SERVICE_PASSWORD_DB', '')] },
      }),
    ).toEqual({ block: false, empty: [] })
  })

  it('blocks while env data is missing and when every user value is blank', () => {
    expect(emptyUserEnvVars(undefined)).toEqual([])
    expect(formatEmptyEnvToast([])).toBe('Fill 0 environment variables before deploying: .')
    expect(deployBlockFromEnv({ isPending: true, isError: false })).toEqual({
      block: true,
      message: 'Still loading environment variables…',
      empty: [],
    })
    expect(deployBlockFromEnv({ isPending: false, isError: true })).toEqual({
      block: true,
      message: 'Could not load environment variables. Try again.',
      empty: [],
    })
    const blank = emptyUserEnvVars([env('PORT', '   '), env('PREVIEW_ONLY', '', { is_preview: true })])
    expect(blank.map((item) => item.key)).toEqual(['PORT'])
    expect(deployBlockFromEnv({ isPending: false, isError: false, data: { environment_variables: blank } }).message).toBe(
      'Fill environment variable PORT before deploying.',
    )
  })

  it('rejects injected keys that are not real generated prefixes and summarizes extras', () => {
    const injected = 'SERVICE_URL_<script>'
    expect(isAutoManagedEnvKey(injected)).toBe(true)
    expect(isAutoManagedEnvKey(' SERVICE_URL_APP')).toBe(false)
    const keys = ['A', 'B', 'C', 'D', 'E'].map((key) => env(key, ''))
    expect(emptyUserEnvVars([env(injected, ''), ...keys]).map((item) => item.key)).toEqual(['A', 'B', 'C', 'D', 'E'])
    expect(formatEmptyEnvToast(keys)).toBe('Fill 5 environment variables before deploying: A, B, C, D and 1 more.')
    const blocked = deployBlockFromEnv({
      isPending: true,
      isError: true,
      data: { environment_variables: [env("'; DROP TABLE env;--", '  ')] },
    })
    expect(blocked.block).toBe(true)
    expect(blocked.message).toContain("'; DROP TABLE env;--")
    expect(blocked.empty).toHaveLength(1)
  })
})

describe('isSecretEnvKey', () => {
  it('masks passwords, tokens, and generated credentials', () => {
    expect(isSecretEnvKey('DATABASE_PASSWORD')).toBe(true)
    expect(isSecretEnvKey('api-key')).toBe(true)
    expect(isSecretEnvKey('SERVICE_PASSWORD_N8N')).toBe(true)
    expect(isSecretEnvKey('SERVICE_USER_N8N')).toBe(true)
    expect(isSecretEnvKey('SERVICE_URL_N8N')).toBe(false)
    expect(isSecretEnvKey('SERVICE_FQDN_N8N')).toBe(false)
  })

  it('does not mask an empty or public key', () => {
    expect(isSecretEnvKey('')).toBe(false)
    expect(isSecretEnvKey('   ')).toBe(false)
    expect(isSecretEnvKey('PUBLIC_HOST')).toBe(false)
  })

  it('masks a padded secret and leaves a script-like public key visible', () => {
    expect(isSecretEnvKey('  password  ')).toBe(true)
    expect(isSecretEnvKey('password\nTOKEN')).toBe(true)
    expect(isSecretEnvKey('javascript:alert(1)')).toBe(false)
    expect(isSecretEnvKey('MYAPIKEY')).toBe(false)
  })
})
