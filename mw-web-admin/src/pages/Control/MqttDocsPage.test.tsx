import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { controlRequest } from '@/lib/control-api'
import { PreferencesProvider } from '@/preferences/preferences-context'
import OAuthDocPage from './OAuthDocPage'

// The default language comes from the browser at import time; pin it to Portuguese and
// switch to English per test through the stored preference.
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

function renderDocs(module: 'MQTT' | 'Mail', path: string, basePath: string) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <PreferencesProvider>
        <MemoryRouter initialEntries={[path]}>
          <Routes>
            <Route
              path={`${basePath}/:version`}
              element={<OAuthDocPage module={module} basePath={basePath} />}
            />
          </Routes>
        </MemoryRouter>
      </PreferencesProvider>
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  localStorage.clear()
  vi.mocked(controlRequest).mockResolvedValue({
    version: 'v1',
    status: 'stable',
    content_type: 'text/markdown',
    documentation: '# Hello MQTT\n\n| A | B |\n| --- | --- |\n| one | two |\n',
  } as never)
})

describe('MQTT documentation page', () => {
  it('asks the service for the Portuguese text and renders the markdown', async () => {
    renderDocs('MQTT', '/mqtt/docs/v1', '/mqtt/docs')

    expect(
      await screen.findByRole('heading', { name: 'Hello MQTT' }),
    ).toBeInTheDocument()
    expect(screen.getByRole('cell', { name: 'two' })).toBeInTheDocument()
    expect(controlRequest).toHaveBeenCalledWith('/mqtt/docs/v1?lang=pt')
    expect(
      screen.getByText(
        'Guia de conexão e contrato de integração do serviço MQTT.',
      ),
    ).toBeInTheDocument()
  })

  it('asks for English when the interface is in English', async () => {
    localStorage.setItem(
      'mw-control-preferences-v1',
      JSON.stringify({ language: 'en' }),
    )
    renderDocs('MQTT', '/mqtt/docs/v1', '/mqtt/docs')

    await screen.findByRole('heading', { name: 'Hello MQTT' })
    expect(controlRequest).toHaveBeenCalledWith('/mqtt/docs/v1?lang=en')
    expect(
      screen.getByText(
        'Connection guide and integration contract for the MQTT service.',
      ),
    ).toBeInTheDocument()
    expect(screen.getByText('API documentation · v1')).toBeInTheDocument()
  })

  it('does not change how the Mail documentation is requested', async () => {
    renderDocs('Mail', '/mail/docs/v1', '/mail/docs')
    await screen.findByRole('heading', { name: 'Hello MQTT' })
    expect(controlRequest).toHaveBeenCalledWith('/mail/docs/v1')
  })
})
