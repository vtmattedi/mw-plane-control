import { afterEach, expect, test, vi } from 'vitest'
import {
  ControlApiError,
  collectionFrom,
  controlRequest,
  currentIdentity,
} from './control-api'

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

test('mapeia a sessão BFF para a identidade do frontend', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn().mockResolvedValue(
      jsonResponse({
        identity: {
          sub: 'usr_123',
          email: 'admin@mattediworks.com',
          name: 'Admin MW',
          auth_time: 123,
          auth_method: 'webauthn',
          assurance: 'strong',
        },
        session: { expires_at: 456, recent_webauthn: true },
      }),
    ),
  )

  await expect(currentIdentity()).resolves.toMatchObject({
    sub: 'usr_123',
    authMethod: 'webauthn',
    recentWebAuthn: true,
  })
})

test('preserva o código de erro do control plane', async () => {
  vi.stubGlobal(
    'fetch',
    vi
      .fn()
      .mockResolvedValue(
        jsonResponse({ error: { code: 'insufficient_scope' } }, 403),
      ),
  )

  await expect(controlRequest('/identity/users')).rejects.toBeInstanceOf(
    ControlApiError,
  )
})

test('extrai coleções envelopadas pelo BFF', () => {
  expect(collectionFrom({ users: [{ id: 'usr_1' }] }, 'users')).toEqual([
    { id: 'usr_1' },
  ])
})

test('envia o cookie CSRF do BFF no header de mutação', async () => {
  document.cookie = 'mw_admin_csrf=csrf-e2e; path=/'
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ ok: true }))
  vi.stubGlobal('fetch', fetchMock)

  await controlRequest('/oauth/projects/1', {
    method: 'PATCH',
    body: JSON.stringify({ enabled: false }),
  })

  expect(fetchMock).toHaveBeenCalledWith(
    '/api/oauth/projects/1',
    expect.objectContaining({
      headers: expect.objectContaining({}),
      method: 'PATCH',
    }),
  )
  const headers = fetchMock.mock.calls[0][1].headers as Headers
  expect(headers.get('X-MW-CSRF')).toBe('csrf-e2e')
})

test('expõe step_up_required para a retomada da operação', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn().mockResolvedValue(
      jsonResponse(
        {
          error: {
            code: 'step_up_required',
            step_up_url: '/api/auth/step-up',
          },
        },
        403,
      ),
    ),
  )

  await expect(
    controlRequest('/oauth/projects/1/rotate-key'),
  ).rejects.toMatchObject({
    code: 'step_up_required',
    stepUpUrl: '/api/auth/step-up',
  })
})
