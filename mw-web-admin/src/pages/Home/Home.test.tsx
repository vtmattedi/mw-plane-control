import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { controlRequest } from '@/lib/control-api'
import { PreferencesProvider } from '@/preferences/preferences-context'
import Home from './Home'

// Portuguese is the app's source language; the default is read at import time.
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

function useApi(responses: Record<string, unknown>) {
  vi.mocked(controlRequest).mockImplementation((async (path: string) => {
    const value = responses[path]
    if (value instanceof Error) throw value
    return value ?? {}
  }) as never)
}

function renderHome() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <PreferencesProvider>
        <MemoryRouter>
          <Home />
        </MemoryRouter>
      </PreferencesProvider>
    </QueryClientProvider>,
  )
}

function card(name: string) {
  const link = screen.getByRole('heading', { name }).closest('a')
  expect(link, `${name} card`).not.toBeNull()
  return link as HTMLElement
}

beforeEach(() => {
  vi.clearAllMocks()
  localStorage.clear()
})

describe('Home overview', () => {
  it('shows the plane version 1.1', async () => {
    useApi({ '/mqtt/server': { brokerConnected: true } })
    renderHome()
    expect(await screen.findByText('v1.1')).toBeInTheDocument()
    expect(screen.queryByText('v1.0')).not.toBeInTheDocument()
  })

  it('links the MQTT card to the clients page', async () => {
    useApi({ '/mqtt/server': { brokerConnected: true } })
    renderHome()
    const mqtt = card('MQTT')
    expect(mqtt).toHaveAttribute('href', '/mqtt/clients')
    expect(
      within(mqtt).getByText('Clientes, credenciais e estado do broker.'),
    ).toBeInTheDocument()
  })

  it('marks MQTT online only while the broker is connected', async () => {
    useApi({ '/mqtt/server': { brokerConnected: true } })
    const first = renderHome()
    expect(
      await within(card('MQTT')).findByText('Conectado'),
    ).toBeInTheDocument()
    expect(await screen.findByText('4/4')).toBeInTheDocument()
    first.unmount()

    // The admin API answers 200 while its broker connection is down: not healthy.
    useApi({
      '/mqtt/server': { brokerConnected: false, dynsecReachable: false },
    })
    renderHome()
    expect(
      await within(card('MQTT')).findByText('Indisponível'),
    ).toBeInTheDocument()
    expect(await screen.findByText('3/4')).toBeInTheDocument()
  })

  it('does not let an unreachable MQTT service affect the other modules', async () => {
    useApi({ '/mqtt/server': new Error('down') })
    renderHome()
    expect(
      await within(card('MQTT')).findByText('Indisponível'),
    ).toBeInTheDocument()
    expect(
      await within(card('OAuth')).findByText('Conectado'),
    ).toBeInTheDocument()
    expect(
      await within(card('Mail')).findByText('Conectado'),
    ).toBeInTheDocument()
  })
})
