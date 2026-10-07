import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { controlRequest } from '@/lib/control-api'
import { PreferencesProvider } from '@/preferences/preferences-context'
import MqttServerPage from './MqttServerPage'

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
  return { ...actual, controlRequest: vi.fn() }
})

type Responses = Record<string, unknown>

function useApi(responses: Responses) {
  vi.mocked(controlRequest).mockImplementation((async (path: string) => {
    const value = responses[path]
    if (value instanceof Error) throw value
    return value ?? null
  }) as never)
}

function renderPage() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <PreferencesProvider>
        <MemoryRouter>
          <MqttServerPage />
        </MemoryRouter>
      </PreferencesProvider>
    </QueryClientProvider>,
  )
}

const roles = {
  roles: [
    {
      name: 'nmnw',
      acls: [
        { type: 'publishClientSend', topic: '#', allow: true, priority: 0 },
      ],
    },
    {
      name: 'admin',
      acls: [
        {
          type: 'publishClientSend',
          topic: '$CONTROL/dynamic-security/#',
          allow: true,
          priority: 0,
        },
      ],
    },
  ],
  assignable: ['nmnw'],
}

beforeEach(() => {
  vi.clearAllMocks()
  localStorage.clear()
})

describe('MqttServerPage', () => {
  it('shows broker health, formatted statistics and links to the roles', async () => {
    useApi({
      '/mqtt/server': {
        brokerConnected: true,
        dynsecReachable: true,
        clients: 2,
      },
      '/mqtt/stats': {
        available: true,
        stale: false,
        updatedAt: '2026-10-06T23:34:50Z',
        version: '2.1.2',
        uptimeSeconds: 3 * 3600 + 20 * 60,
        clients: { connected: 3, total: 5, maximum: 9 },
        store: { subscriptions: 7, retainedMessages: 1234 },
        messages: { received: 1000, sent: 2000 },
        bytes: { received: 1536, sent: 5 * 1024 * 1024 },
        load: { messagesReceived1m: 36.11, bytesSent15m: 2048 },
        memory: { heapCurrent: 858_876, heapMaximum: 879_858 },
      },
      '/mqtt/roles': roles,
    })
    renderPage()

    expect(await screen.findByText('Conectado')).toBeInTheDocument()
    expect(screen.getByText('2.1.2')).toBeInTheDocument()
    expect(screen.getByText('3h 20m')).toBeInTheDocument()
    expect(await screen.findByText('1.234')).toBeInTheDocument() // retained, pt-BR
    expect(screen.getByText('1,5 KB')).toBeInTheDocument()
    expect(screen.getByText('5 MB')).toBeInTheDocument()
    expect(screen.getByText('36,1')).toBeInTheDocument()
    expect(screen.getByText('839 KB')).toBeInTheDocument()

    // The page summarises the roles and links to where they are managed.
    expect(
      await screen.findByText('2 funções · 1 atribuíveis'),
    ).toBeInTheDocument()
    expect(
      screen.getByRole('link', { name: 'Gerenciar funções' }),
    ).toHaveAttribute('href', '/mqtt/roles')
    expect(screen.queryByRole('button', { name: /excluir|criar/i })).toBeNull()
  })

  it('flags stale statistics', async () => {
    useApi({
      '/mqtt/server': { brokerConnected: true, dynsecReachable: true },
      '/mqtt/stats': {
        available: true,
        stale: true,
        updatedAt: '2026-10-06T23:00:00Z',
        clients: { connected: 1 },
      },
      '/mqtt/roles': roles,
    })
    renderPage()
    expect(
      await screen.findByText('Sem atualização recente do broker'),
    ).toBeInTheDocument()
  })

  it('reports an unreachable broker and waits for the first statistics', async () => {
    useApi({
      '/mqtt/server': { brokerConnected: false, dynsecReachable: false },
      '/mqtt/stats': { available: false, stale: false },
      '/mqtt/roles': roles,
    })
    renderPage()
    expect(await screen.findByText('Indisponível')).toBeInTheDocument()
    expect(
      await screen.findByText(/O broker ainda não publicou estatísticas/),
    ).toBeInTheDocument()
  })
})
