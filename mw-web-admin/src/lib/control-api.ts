export type ControlIdentity = {
  sub: string
  email: string
  name: string
  authTime: number
  authMethod: string
  assurance: string
  sessionExpiresAt: number
  recentWebAuthn: boolean
}

export class ControlApiError extends Error {
  readonly status: number
  readonly code: string
  readonly stepUpUrl?: string

  constructor(
    message: string,
    status: number,
    code: string,
    stepUpUrl?: string,
  ) {
    super(message)
    this.name = 'ControlApiError'
    this.status = status
    this.code = code
    this.stepUpUrl = stepUpUrl
  }
}

async function responseBody(response: Response): Promise<unknown> {
  const text = await response.text()
  if (!text) return null
  try {
    return JSON.parse(text)
  } catch {
    return text
  }
}

function csrfCookie() {
  const value = document.cookie
    .split('; ')
    .find((cookie) => cookie.startsWith('mw_admin_csrf='))
    ?.split('=')[1]
  return value ? decodeURIComponent(value) : ''
}

export async function controlRequest<T>(
  path: string,
  init: RequestInit = {},
): Promise<T> {
  const method = (init.method ?? 'GET').toUpperCase()
  const headers = new Headers(init.headers)
  headers.set('Accept', 'application/json')
  if (init.body && !(init.body instanceof FormData)) {
    headers.set('Content-Type', 'application/json')
  }
  if (!['GET', 'HEAD', 'OPTIONS'].includes(method)) {
    let token = csrfCookie()
    if (!token) {
      const csrfResponse = await fetch('/api/auth/csrf', {
        method: 'POST',
        credentials: 'same-origin',
      })
      const csrfBody = (await responseBody(csrfResponse)) as {
        csrf_token?: string
      } | null
      token = csrfBody?.csrf_token ?? ''
    }
    if (token) headers.set('X-MW-CSRF', token)
  }

  const response = await fetch(`/api${path}`, {
    ...init,
    method,
    headers,
    credentials: 'same-origin',
  })
  const body = await responseBody(response)
  if (!response.ok) {
    const error = (body ?? {}) as {
      error?: { code?: string; step_up_url?: string } | string
      code?: string
      message?: string
    }
    const nested = typeof error.error === 'object' ? error.error : undefined
    const code =
      nested?.code ??
      (typeof error.error === 'string' ? error.error : undefined) ??
      error.code ??
      'request_failed'
    throw new ControlApiError(
      error.message ?? readableError(code),
      response.status,
      code,
      nested?.step_up_url,
    )
  }
  return body as T
}

function readableError(code: string) {
  const labels: Record<string, string> = {
    unauthenticated: 'Sua sessão expirou. Entre novamente.',
    insufficient_scope: 'Sua conta não tem permissão para esta operação.',
    step_up_required: 'Confirme sua identidade para continuar.',
    downstream_unavailable: 'O serviço não está disponível no momento.',
    email_exists: 'Já existe uma identidade com este e-mail.',
    client_id_exists: 'Já existe uma aplicação com este Client ID.',
    invalid_request: 'Revise os campos informados e tente novamente.',
    application_archive_required:
      'Arquive a aplicação antes de excluí-la permanentemente.',
    application_in_use:
      'Esta aplicação ainda possui usuários associados, códigos de autorização ou tokens emitidos.',
  }
  return labels[code] ?? 'Não foi possível concluir a solicitação.'
}

export async function currentIdentity(): Promise<ControlIdentity> {
  const body = await controlRequest<{
    identity: {
      sub: string
      email: string
      name: string
      auth_time: number
      auth_method: string
      assurance: string
    }
    session: { expires_at: number; recent_webauthn: boolean }
  }>('/auth/me')
  return {
    sub: body.identity.sub,
    email: body.identity.email,
    name: body.identity.name,
    authTime: body.identity.auth_time,
    authMethod: body.identity.auth_method,
    assurance: body.identity.assurance,
    sessionExpiresAt: body.session.expires_at,
    recentWebAuthn: body.session.recent_webauthn,
  }
}

export function beginLogin(returnTo = window.location.pathname) {
  window.location.assign(
    `/api/auth/login?return_to=${encodeURIComponent(returnTo)}`,
  )
}

export function beginStepUp(returnTo = window.location.pathname) {
  window.location.assign(
    `/api/auth/step-up?return_to=${encodeURIComponent(returnTo)}`,
  )
}

export function beginIdentitySetup() {
  const configured = import.meta.env.VITE_IDENTITY_PUBLIC_URL as
    string | undefined
  const base = (configured || 'http://localhost:7100').replace(/\/+$/, '')
  window.location.assign(`${base}/login`)
}

export async function endSession() {
  await controlRequest<void>('/auth/logout', { method: 'POST' })
}

export function collectionFrom(
  body: unknown,
  key: string,
): Record<string, unknown>[] {
  if (Array.isArray(body)) return body as Record<string, unknown>[]
  if (body && typeof body === 'object') {
    const value = (body as Record<string, unknown>)[key]
    if (Array.isArray(value)) return value as Record<string, unknown>[]
  }
  return []
}
