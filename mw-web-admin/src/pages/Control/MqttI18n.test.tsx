import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { AppConfirmProvider } from '@/components/confirm-provider'
import { controlRequest } from '@/lib/control-api'
import { PreferencesProvider } from '@/preferences/preferences-context'
import MqttClientsPage from './MqttClientsPage'
import MqttServerPage from './MqttServerPage'

// English UI. Pinned so the test does not depend on the machine's language.
vi.hoisted(() => {
  Object.defineProperty(window.navigator, 'language', {
    configurable: true,
    value: 'en-US',
  })
})

vi.mock('@/lib/control-api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/lib/control-api')>()
  return { ...actual, controlRequest: vi.fn() }
})

// Source strings are Portuguese, so any accented letter in the English UI is a string
// that skipped the translation table.
const accented = /[ÀÁÂÃÄÅÇÈÉÊËÌÍÎÏÑÒÓÔÕÖÙÚÛÜàáâãäåçèéêëìíîïñòóôõöùúûü]/

function expectNoPortuguese(where: string) {
  const texts = [
    document.body.textContent ?? '',
    ...[...document.querySelectorAll('[aria-label]')].map(
      (el) => el.getAttribute('aria-label') ?? '',
    ),
    ...[...document.querySelectorAll('[placeholder]')].map(
      (el) => el.getAttribute('placeholder') ?? '',
    ),
  ]
  for (const text of texts) {
    const hit = accented.exec(text)
    const context = hit
      ? text.slice(Math.max(0, hit.index - 40), hit.index + 40)
      : ''
    expect(context, `Portuguese text in the English UI (${where})`).toBe('')
  }
}

const api: Record<string, unknown> = {
  '/mqtt/clients': {
    clients: [
      { username: 'gw1', disabled: false, roles: ['nmnw'] },
      { username: 'gw-off', disabled: true, roles: ['nmnw'] },
      { username: 'mqtt-admin', disabled: false, roles: ['admin'] },
    ],
  },
  '/mqtt/roles': {
    roles: [
      {
        name: 'nmnw',
        acls: [
          { type: 'publishClientSend', topic: '#', allow: true, priority: 0 },
        ],
      },
    ],
    assignable: ['nmnw'],
  },
  '/mqtt/stats': {
    available: true,
    stale: true,
    updatedAt: '2026-10-06T23:34:50Z',
    version: '2.1.2',
    uptimeSeconds: 4000,
    clients: { connected: 3, total: 5, maximum: 9 },
    store: { subscriptions: 7, retainedMessages: 42 },
    messages: { received: 1000 },
    bytes: { received: 1536 },
    load: { messagesReceived1m: 36.1 },
    memory: { heapCurrent: 858_876 },
  },
  '/mqtt/server': { brokerConnected: true, dynsecReachable: true },
}

function useApi(overrides: Record<string, unknown> = {}) {
  vi.mocked(controlRequest).mockImplementation((async (
    path: string,
    init?: RequestInit,
  ) => {
    const value = path in overrides ? overrides[path] : api[path]
    if (init?.method === 'POST' && path === '/mqtt/clients') {
      return { username: 'gw2', role: 'nmnw', password: 'generated-secret' }
    }
    if (value instanceof Error) throw value
    return value ?? null
  }) as never)
}

function renderPage(page: 'clients' | 'server') {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <PreferencesProvider>
        <AppConfirmProvider>
          <MemoryRouter>
            {page === 'clients' ? <MqttClientsPage /> : <MqttServerPage />}
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

describe('MQTT pages respect the English language', () => {
  it('clients page, its dialogs and the one-time credentials', async () => {
    const user = userEvent.setup()
    useApi()
    renderPage('clients')
    await screen.findByText('gw1')
    expect(screen.getByRole('button', { name: 'New client' })).toBeVisible()
    expectNoPortuguese('clients table')

    // Create dialog, including its validation hint.
    await user.click(screen.getByRole('button', { name: 'New client' }))
    const create = await screen.findByRole('dialog', { name: 'New client' })
    await user.type(within(create).getByRole('textbox'), '-bad')
    expectNoPortuguese('create dialog with validation error')

    // Credentials dialog after a successful create.
    await user.clear(within(create).getByRole('textbox'))
    await user.type(within(create).getByRole('textbox'), 'gw2')
    await user.click(
      within(create).getByRole('button', { name: 'Create client' }),
    )
    const reveal = await screen.findByRole('dialog', {
      name: 'MQTT credentials',
    })
    expect(
      within(reveal).getByRole('button', { name: 'Done' }),
    ).toBeInTheDocument()
    expect(
      within(reveal).getByRole('button', { name: 'Close' }),
    ).toBeInTheDocument()
    expectNoPortuguese('credentials dialog')
    await user.click(within(reveal).getByRole('button', { name: 'Done' }))

    // Each confirmation dialog.
    for (const [button, title] of [
      ['Delete', 'Delete client?'],
      ['New password', 'Generate a new password?'],
      ['Disable', 'Disable client?'],
      ['Enable', 'Enable client?'],
    ] as const) {
      await user.click(screen.getAllByRole('button', { name: button })[0])
      const confirm = await screen.findByRole('alertdialog', { name: title })
      expect(
        within(confirm).getByRole('button', { name: 'Cancel' }),
      ).toBeInTheDocument()
      expectNoPortuguese(title)
      await user.click(within(confirm).getByRole('button', { name: 'Cancel' }))
      await waitFor(() =>
        expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument(),
      )
    }
  })

  it('translates the error shown when delegation fails', async () => {
    const { ControlApiError } = await import('@/lib/control-api')
    useApi({
      '/mqtt/clients': new ControlApiError(
        'Não foi possível obter autorização para o serviço MQTT. Saia e entre novamente.',
        502,
        'mqtt_delegation_failed',
      ),
    })
    renderPage('clients')
    expect(
      await screen.findByText(
        'Could not get authorization for the MQTT service. Sign out and sign in again.',
        {},
        { timeout: 4000 },
      ),
    ).toBeInTheDocument()
    expectNoPortuguese('delegation error')
  })

  it('broker page with statistics, stale notice and roles', async () => {
    useApi()
    renderPage('server')
    expect(await screen.findByText('Connected')).toBeInTheDocument()
    expect(await screen.findByText('Assignable')).toBeInTheDocument()
    expect(
      screen.getByText('No recent update from the broker'),
    ).toBeInTheDocument()
    expectNoPortuguese('broker page')
  })

  it('broker page when the broker is unavailable and has no statistics yet', async () => {
    useApi({
      '/mqtt/server': { brokerConnected: false, dynsecReachable: false },
      '/mqtt/stats': { available: false, stale: false },
    })
    renderPage('server')
    expect(await screen.findByText('Unavailable')).toBeInTheDocument()
    expect(
      await screen.findByText(/has not published statistics yet/),
    ).toBeInTheDocument()
    expectNoPortuguese('broker page without data')
  })
})
