import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { AppConfirmProvider } from '@/components/confirm-provider'
import { beginStepUp, ControlApiError, controlRequest } from '@/lib/control-api'
import { PreferencesProvider } from '@/preferences/preferences-context'
import MqttClientsPage from './MqttClientsPage'

// Portuguese is the app's source language. The preferences module picks its default from
// navigator.language when it is first imported, so this must run before the imports.
vi.hoisted(() => {
  Object.defineProperty(window.navigator, 'language', {
    configurable: true,
    value: 'pt-BR',
  })
})

vi.mock('@/lib/control-api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/lib/control-api')>()
  return { ...actual, controlRequest: vi.fn(), beginStepUp: vi.fn() }
})

const pendingKey = 'mw-mqtt-pending-operation'

type Handler = (path: string, init?: RequestInit) => unknown

const baseHandler: Handler = (path) => {
  switch (path) {
    case '/mqtt/clients':
      return {
        clients: [
          { username: 'gw1', disabled: false, roles: ['nmnw'] },
          { username: 'mqtt-admin', disabled: false, roles: ['admin'] },
        ],
      }
    case '/mqtt/roles':
      return { roles: [], assignable: ['nmnw'] }
    case '/mqtt/stats':
      return {
        available: true,
        stale: false,
        clients: { connected: 3 },
        store: { subscriptions: 7, retainedMessages: 42 },
      }
    default:
      return null
  }
}

function useApi(handler: Handler = baseHandler) {
  vi.mocked(controlRequest).mockImplementation((async (
    path: string,
    init?: RequestInit,
  ) => {
    const result = handler(path, init)
    if (result instanceof Error) throw result
    return result
  }) as never)
}

function mutationCalls() {
  return vi
    .mocked(controlRequest)
    .mock.calls.filter(([, init]) => (init?.method ?? 'GET') !== 'GET')
}

function renderPage() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <PreferencesProvider>
        <AppConfirmProvider>
          <MemoryRouter>
            <MqttClientsPage />
          </MemoryRouter>
        </AppConfirmProvider>
      </PreferencesProvider>
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  sessionStorage.clear()
  localStorage.clear()
})

describe('MqttClientsPage', () => {
  it('lists clients, shows broker stats and protects reserved clients', async () => {
    useApi()
    renderPage()

    expect(await screen.findByText('gw1')).toBeInTheDocument()
    expect(screen.getByText('mqtt-admin')).toBeInTheDocument()
    // One manageable client: a single set of actions. The admin client is read-only.
    expect(screen.getAllByRole('button', { name: 'Excluir' })).toHaveLength(1)
    expect(screen.getAllByRole('button', { name: 'Nova senha' })).toHaveLength(
      1,
    )
    expect(screen.getByText('Protegido')).toBeInTheDocument()
    expect(await screen.findByText('42')).toBeInTheDocument()
    expect(screen.getByText('7')).toBeInTheDocument()
  })

  it('creates a client and shows the generated password once', async () => {
    const user = userEvent.setup()
    useApi((path, init) =>
      path === '/mqtt/clients' && init?.method === 'POST'
        ? { username: 'gw2', role: 'nmnw', password: 'generated-secret-value' }
        : baseHandler(path, init),
    )
    renderPage()
    await screen.findByText('gw1')

    await user.click(screen.getByRole('button', { name: 'Novo cliente' }))
    const dialog = await screen.findByRole('dialog', { name: 'Novo cliente' })
    const create = within(dialog).getByRole('button', { name: 'Criar cliente' })

    await user.type(within(dialog).getByRole('textbox'), '-bad name')
    expect(create).toBeDisabled()
    await user.clear(within(dialog).getByRole('textbox'))
    await user.type(within(dialog).getByRole('textbox'), 'gw2')
    expect(create).toBeEnabled()
    await user.click(create)

    const reveal = await screen.findByRole('dialog', {
      name: 'Credenciais MQTT',
    })
    expect(within(reveal).getByDisplayValue('gw2')).toBeInTheDocument()
    expect(
      within(reveal).getByDisplayValue('generated-secret-value'),
    ).toBeInTheDocument()

    const [call] = mutationCalls()
    expect(call[0]).toBe('/mqtt/clients')
    expect(JSON.parse(String(call[1]?.body))).toEqual({
      username: 'gw2',
      role: 'nmnw',
    })
    // The password is never persisted anywhere in the browser.
    expect(
      JSON.stringify({ ...sessionStorage, ...localStorage }),
    ).not.toContain('generated-secret-value')
  })

  it('deletes only after explicit confirmation', async () => {
    const user = userEvent.setup()
    useApi()
    renderPage()
    await screen.findByText('gw1')

    await user.click(screen.getByRole('button', { name: 'Excluir' }))
    const confirm = await screen.findByRole('alertdialog', {
      name: 'Excluir cliente?',
    })
    expect(mutationCalls()).toHaveLength(0)

    await user.click(within(confirm).getByRole('button', { name: 'Excluir' }))
    await waitFor(() => expect(mutationCalls()).toHaveLength(1))
    const [path, init] = mutationCalls()[0]
    expect(path).toBe('/mqtt/clients/gw1')
    expect(init?.method).toBe('DELETE')
  })

  it('cancelling the confirmation sends nothing', async () => {
    const user = userEvent.setup()
    useApi()
    renderPage()
    await screen.findByText('gw1')

    await user.click(screen.getByRole('button', { name: 'Nova senha' }))
    const confirm = await screen.findByRole('alertdialog', {
      name: 'Gerar nova senha?',
    })
    await user.click(within(confirm).getByRole('button', { name: 'Cancelar' }))
    expect(mutationCalls()).toHaveLength(0)
  })

  it('stores the operation and starts step-up when fresh WebAuthn is required', async () => {
    const user = userEvent.setup()
    useApi((path, init) =>
      init?.method === 'POST'
        ? new ControlApiError(
            'step',
            403,
            'step_up_required',
            '/api/auth/step-up',
          )
        : baseHandler(path, init),
    )
    renderPage()
    await screen.findByText('gw1')

    await user.click(screen.getByRole('button', { name: 'Nova senha' }))
    const confirm = await screen.findByRole('alertdialog', {
      name: 'Gerar nova senha?',
    })
    await user.click(
      within(confirm).getByRole('button', { name: 'Gerar nova senha' }),
    )

    await waitFor(() => expect(beginStepUp).toHaveBeenCalledWith('/'))
    expect(JSON.parse(sessionStorage.getItem(pendingKey) ?? 'null')).toEqual({
      operation: { kind: 'rotate', username: 'gw1' },
      returnTo: '/',
    })
  })

  it('resumes the pending operation exactly once after step-up', async () => {
    useApi()
    sessionStorage.setItem(
      pendingKey,
      JSON.stringify({
        operation: { kind: 'delete', username: 'gw1' },
        returnTo: '/',
      }),
    )
    renderPage()

    await waitFor(() => expect(mutationCalls()).toHaveLength(1))
    expect(mutationCalls()[0][0]).toBe('/mqtt/clients/gw1')
    expect(sessionStorage.getItem(pendingKey)).toBeNull()
  })

  it('discards a malformed or tampered pending operation without sending it', async () => {
    useApi()
    sessionStorage.setItem(
      pendingKey,
      JSON.stringify({
        operation: { kind: 'delete', username: '../etc/passwd' },
        returnTo: '/',
      }),
    )
    renderPage()
    await screen.findByText('gw1')

    expect(mutationCalls()).toHaveLength(0)
    expect(sessionStorage.getItem(pendingKey)).toBeNull()
  })

  it('does not replay an operation meant for another page', async () => {
    useApi()
    sessionStorage.setItem(
      pendingKey,
      JSON.stringify({
        operation: { kind: 'delete', username: 'gw1' },
        returnTo: '/somewhere-else',
      }),
    )
    renderPage()
    await screen.findByText('gw1')
    expect(mutationCalls()).toHaveLength(0)
  })

  it('explains a broker outage instead of a generic failure', async () => {
    useApi((path) =>
      path === '/mqtt/clients'
        ? new ControlApiError('x', 503, 'broker_unavailable')
        : null,
    )
    renderPage()
    expect(
      await screen.findByText(
        'O broker MQTT não está acessível no momento.',
        {},
        // The page retries a failed query once, one second later.
        { timeout: 4000 },
      ),
    ).toBeInTheDocument()
  })
})
