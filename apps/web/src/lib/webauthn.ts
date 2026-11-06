function bytesToB64url(bytes: Uint8Array) {
  let bin = ''
  for (const b of bytes) bin += String.fromCharCode(b)
  return btoa(bin).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/g, '')
}

function b64urlToBytes(value: string) {
  const pad = value.replace(/-/g, '+').replace(/_/g, '/')
  const bin = atob(pad + '='.repeat((4 - (pad.length % 4)) % 4))
  const out = new Uint8Array(bin.length)
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i)
  return out
}

function credentialJSON(cred: PublicKeyCredential) {
  const response = cred.response as AuthenticatorAttestationResponse & AuthenticatorAssertionResponse
  const body: Record<string, unknown> = {
    id: cred.id,
    rawId: bytesToB64url(new Uint8Array(cred.rawId)),
    type: cred.type,
    clientExtensionResults: cred.getClientExtensionResults(),
    response: {
      clientDataJSON: bytesToB64url(new Uint8Array(response.clientDataJSON)),
    },
  }
  const out = body.response as Record<string, string>
  if (response.attestationObject) {
    out.attestationObject = bytesToB64url(new Uint8Array(response.attestationObject))
  }
  if (response.authenticatorData) {
    out.authenticatorData = bytesToB64url(new Uint8Array(response.authenticatorData))
    out.signature = bytesToB64url(new Uint8Array(response.signature))
    if (response.userHandle) out.userHandle = bytesToB64url(new Uint8Array(response.userHandle))
  }
  return body
}

function publicKeyOptions(options: { publicKey?: PublicKeyCredentialCreationOptions } & Record<string, unknown>, kind: 'create' | 'get') {
  const pk = (options.publicKey || options) as Record<string, unknown>
  pk.challenge = b64urlToBytes(String(pk.challenge))
  if (kind === 'create' && pk.user && typeof pk.user === 'object') {
    const user = pk.user as { id: string }
    user.id = b64urlToBytes(String(user.id)) as unknown as string
  }
  const listKey = kind === 'create' ? 'excludeCredentials' : 'allowCredentials'
  const list = pk[listKey]
  if (Array.isArray(list)) {
    pk[listKey] = list.map((item) => {
      const cred = item as { id: string }
      return { ...cred, id: b64urlToBytes(String(cred.id)) }
    })
  }
  return pk as unknown as PublicKeyCredentialCreationOptions
}

export async function createPasskey(options: { publicKey?: PublicKeyCredentialCreationOptions }) {
  const cred = (await navigator.credentials.create({
    publicKey: publicKeyOptions(options as never, 'create'),
  })) as PublicKeyCredential | null
  if (!cred) throw new Error('passkey was cancelled')
  return credentialJSON(cred)
}

export async function getPasskey(options: { publicKey?: PublicKeyCredentialRequestOptions }) {
  const cred = (await navigator.credentials.get({
    publicKey: publicKeyOptions(options as never, 'get') as PublicKeyCredentialRequestOptions,
  })) as PublicKeyCredential | null
  if (!cred) throw new Error('passkey was cancelled')
  return credentialJSON(cred)
}
