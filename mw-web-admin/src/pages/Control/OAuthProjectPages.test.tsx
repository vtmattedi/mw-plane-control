import { StrictMode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Link, MemoryRouter, Navigate, Route, Routes } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { AppConfirmProvider } from '@/components/confirm-provider'
import { beginStepUp, ControlApiError, controlRequest } from '@/lib/control-api'
import type { GoogleOAuthConfig } from '@/lib/oauth-provider-config'
import { PreferencesProvider } from '@/preferences/preferences-context'
import OAuthProjectLayout, {
  OAuthProvidersRedirect,
} from './OAuthProjectLayout'
import OAuthProjectConfigurationPage from './OAuthProjectConfigurationPage'
import OAuthProjectActivityPage from './OAuthProjectActivityPage'
import OAuthProvidersPage from './OAuthProvidersPage'
import { OAuthProjectsPage } from './pages'

vi.hoisted(() => {
  Object.defineProperty(window.navigator, 'language', {
    configurable: true,
    value: 'pt-BR',
  })
})
vi.mock('@/lib/control-api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/control-api')>()),
  controlRequest: vi.fn(),
  beginStepUp: vi.fn(),
}))

const projects = [
  { id: 7, name: 'Acme', enabled: true },
  { id: 8, name: 'Beta', enabled: true },
]
const policy: GoogleOAuthConfig = {
  enabled: true,
  allowed_scopes: ['openid'],
  allowed_callback_urls: ['https://acme.example.com/callback'],
  allowed_return_urls: ['https://acme.example.com/settings'],
  limits: { per_minute: 10, per_hour: 100, max_pending: 50 },
}

function mockApi() {
  vi.mocked(controlRequest).mockImplementation((async (
    path: string,
    init?: RequestInit,
  ) => {
    if (init?.method === 'PUT') return JSON.parse(String(init.body))
    if (path === '/oauth/projects') return { projects }
    if (/\/oauth\/google$/.test(path)) return policy
    if (path.startsWith('/oauth/audit?'))
      return {
        items: [{ id: 1, event_type: 'project.updated', status: 'success' }],
      }
    return {}
  }) as typeof controlRequest)
}

function renderProject(path = '/oauth/projects/7/providers/google') {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  return render(
    <StrictMode>
      <QueryClientProvider client={client}>
        <PreferencesProvider>
          <AppConfirmProvider>
            <MemoryRouter initialEntries={[path]}>
              <Link to="/oauth/projects/7/providers/google">Open Acme</Link>
              <Link to="/oauth/projects/8/providers/google">Open Beta</Link>
              <Routes>
                <Route path="/oauth/projects" element={<OAuthProjectsPage />} />
                <Route
                  path="/oauth/providers"
                  element={<OAuthProvidersRedirect />}
                />
                <Route
                  path="/oauth/projects/:projectID"
                  element={<OAuthProjectLayout />}
                >
                  <Route
                    index
                    element={<OAuthProjectConfigurationPage key="overview" />}
                  />
                  <Route
                    path="credentials"
                    element={
                      <OAuthProjectConfigurationPage
                        key="credentials"
                        section="credentials"
                      />
                    }
                  />
                  <Route
                    path="providers/google"
                    element={<OAuthProvidersPage />}
                  />
                  <Route
                    path="activity"
                    element={<OAuthProjectActivityPage />}
                  />
                  <Route
                    path="configuration"
                    element={<Navigate replace to="../credentials" />}
                  />
                </Route>
              </Routes>
            </MemoryRouter>
          </AppConfirmProvider>
        </PreferencesProvider>
      </QueryClientProvider>
    </StrictMode>,
  )
}

function writes() {
  return vi
    .mocked(controlRequest)
    .mock.calls.filter(([, init]) => init?.method && init.method !== 'GET')
}

beforeEach(() => {
  vi.clearAllMocks()
  localStorage.clear()
  sessionStorage.clear()
  mockApi()
})

describe('OAuth project navigation and policy ownership', () => {
  it('names the project, provides scoped tabs and saves only its policy', async () => {
    const user = userEvent.setup()
    renderProject()
    expect(
      await screen.findByRole('heading', { name: 'Acme' }),
    ).toBeInTheDocument()
    const nav = screen.getByRole('navigation', { name: 'Navegação do projeto' })
    expect(
      within(nav).getByRole('link', { name: 'Provedores' }),
    ).toHaveAttribute('aria-current', 'page')
    expect(
      within(nav).getByRole('link', { name: 'Credenciais' }),
    ).toHaveAttribute('href', '/oauth/projects/7/credentials')
    expect(screen.queryByRole('combobox')).not.toBeInTheDocument()
    await waitFor(() =>
      expect(screen.getByLabelText('Escopos permitidos')).toHaveValue('openid'),
    )
    await user.clear(screen.getByLabelText('Escopos permitidos'))
    await user.type(
      screen.getByLabelText('Escopos permitidos'),
      'openid, email',
    )
    await user.click(
      screen.getByRole('button', { name: 'Salvar alterações para Acme' }),
    )
    await waitFor(() => expect(writes()).toHaveLength(1))
    expect(writes()[0][0]).toBe('/oauth/projects/7/oauth/google')
    expect(JSON.parse(String(writes()[0][1]?.body)).allowed_scopes).toEqual([
      'openid',
      'email',
    ])
    await user.click(within(nav).getByRole('link', { name: 'Credenciais' }))
    expect(
      await screen.findByRole('button', { name: 'Rotacionar chave' }),
    ).toBeInTheDocument()
    expect(
      screen.queryByLabelText('Escopos permitidos'),
    ).not.toBeInTheDocument()
  })

  it('never substitutes the first project for an unknown project', async () => {
    renderProject('/oauth/projects/999/providers/google')
    expect(
      await screen.findByRole('heading', { name: 'Projeto não encontrado' }),
    ).toBeInTheDocument()
    expect(
      screen.getByRole('link', { name: 'Voltar aos projetos' }),
    ).toHaveAttribute('href', '/oauth/projects')
    expect(controlRequest).not.toHaveBeenCalledWith(
      '/oauth/projects/7/oauth/google',
    )
    expect(
      screen.queryByLabelText('Escopos permitidos'),
    ).not.toBeInTheDocument()
    expect(writes()).toHaveLength(0)
  })

  it('keeps policy editing disabled when loading fails', async () => {
    vi.mocked(controlRequest).mockImplementation((async (path: string) => {
      if (path === '/oauth/projects') return { projects }
      throw new Error('service unavailable')
    }) as typeof controlRequest)
    renderProject()
    expect(
      await screen.findByRole('alert', {}, { timeout: 3000 }),
    ).toHaveTextContent('Não foi possível carregar a configuração do Google.')
    expect(screen.getByLabelText('Escopos permitidos')).toBeDisabled()
    expect(
      screen.getByRole('button', { name: 'Salvar alterações para Acme' }),
    ).toBeDisabled()
    expect(screen.getByText('Estado indisponível')).toBeInTheDocument()
    expect(writes()).toHaveLength(0)
  })

  it('discards the previous project draft when navigating while the next policy loads', async () => {
    const user = userEvent.setup()
    let resolveBeta!: (value: GoogleOAuthConfig) => void
    const beta = new Promise<GoogleOAuthConfig>((resolve) => {
      resolveBeta = resolve
    })
    const original = vi.mocked(controlRequest).getMockImplementation()!
    vi.mocked(controlRequest).mockImplementation(((
      path: string,
      init?: RequestInit,
    ) => {
      if (path === '/oauth/projects/8/oauth/google' && !init?.method)
        return beta
      return original(path, init)
    }) as typeof controlRequest)
    renderProject()
    await waitFor(() =>
      expect(screen.getByLabelText('Escopos permitidos')).toHaveValue('openid'),
    )
    await user.type(screen.getByLabelText('Escopos permitidos'), ', email')
    await user.click(screen.getByRole('link', { name: 'Open Beta' }))
    expect(
      await screen.findByRole('heading', { name: 'Beta' }),
    ).toBeInTheDocument()
    expect(screen.getByLabelText('Escopos permitidos')).toHaveValue('')
    expect(
      screen.getByRole('button', { name: 'Salvar alterações para Beta' }),
    ).toBeDisabled()
    await act(async () =>
      resolveBeta({ ...policy, allowed_scopes: ['profile'] }),
    )
    await waitFor(() =>
      expect(screen.getByLabelText('Escopos permitidos')).toHaveValue(
        'profile',
      ),
    )
    await user.click(
      screen.getByRole('button', { name: 'Salvar alterações para Beta' }),
    )
    await waitFor(() => expect(writes()).toHaveLength(1))
    expect(writes()[0][0]).toBe('/oauth/projects/8/oauth/google')
    expect(JSON.parse(String(writes()[0][1]?.body)).allowed_scopes).toEqual([
      'profile',
    ])
  })

  it('returns step-up to the selected project and resumes once only in that project', async () => {
    const user = userEvent.setup()
    const original = vi.mocked(controlRequest).getMockImplementation()!
    let requireStepUp = true
    vi.mocked(controlRequest).mockImplementation(((
      path: string,
      init?: RequestInit,
    ) => {
      if (init?.method === 'PUT' && requireStepUp) {
        requireStepUp = false
        return Promise.reject(
          new ControlApiError('Step-up required', 403, 'step_up_required'),
        )
      }
      return original(path, init)
    }) as typeof controlRequest)
    const first = renderProject()
    await waitFor(() =>
      expect(
        screen.getByRole('button', { name: 'Salvar alterações para Acme' }),
      ).toBeEnabled(),
    )
    await user.click(
      screen.getByRole('button', { name: 'Salvar alterações para Acme' }),
    )
    await waitFor(() =>
      expect(beginStepUp).toHaveBeenCalledWith(
        '/oauth/projects/7/providers/google',
      ),
    )
    first.unmount()
    renderProject('/oauth/projects/8/providers/google')
    await waitFor(() =>
      expect(
        screen.getByRole('button', { name: 'Salvar alterações para Beta' }),
      ).toBeEnabled(),
    )
    expect(writes()).toHaveLength(1)
    await user.click(screen.getByRole('link', { name: 'Open Acme' }))
    await waitFor(() => expect(writes()).toHaveLength(2))
    expect(writes().map(([path]) => path)).toEqual([
      '/oauth/projects/7/oauth/google',
      '/oauth/projects/7/oauth/google',
    ])
    expect(
      sessionStorage.getItem('mw-oauth-google-provider-pending'),
    ).toBeNull()
  })

  it('keeps credential rotation tied to its originating project and credentials tab through step-up', async () => {
    const user = userEvent.setup()
    const original = vi.mocked(controlRequest).getMockImplementation()!
    let requireStepUp = true
    vi.mocked(controlRequest).mockImplementation(((
      path: string,
      init?: RequestInit,
    ) => {
      if (init?.method === 'POST') {
        if (requireStepUp) {
          requireStepUp = false
          return Promise.reject(
            new ControlApiError('Step-up required', 403, 'step_up_required'),
          )
        }
        return Promise.resolve({ api_key: 'test-rotated-key' })
      }
      return original(path, init)
    }) as typeof controlRequest)
    const first = renderProject('/oauth/projects/7/credentials')
    await user.click(
      await screen.findByRole('button', { name: 'Rotacionar chave' }),
    )
    const confirmation = await screen.findByRole('alertdialog')
    await user.click(
      within(confirmation).getByRole('button', { name: 'Rotacionar chave' }),
    )
    await waitFor(() =>
      expect(beginStepUp).toHaveBeenCalledWith('/oauth/projects/7/credentials'),
    )
    first.unmount()
    renderProject('/oauth/projects/8/credentials')
    await screen.findByRole('heading', { name: 'Beta' })
    expect(writes()).toHaveLength(1)
    await user.click(screen.getByRole('link', { name: 'Open Acme' }))
    await screen.findByRole('button', { name: 'Salvar alterações para Acme' })
    expect(writes()).toHaveLength(1)
    await user.click(screen.getByRole('link', { name: 'Credenciais' }))
    await waitFor(() => expect(writes()).toHaveLength(2))
    expect(writes().map(([path]) => path)).toEqual([
      '/oauth/projects/7/rotate-key',
      '/oauth/projects/7/rotate-key',
    ])
  })

  it('redirects old provider URLs to their project and old configuration URLs to credentials', async () => {
    const first = renderProject('/oauth/providers?project=8')
    expect(
      await screen.findByRole('heading', { name: 'Beta' }),
    ).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Provedores' })).toHaveAttribute(
      'aria-current',
      'page',
    )
    first.unmount()
    renderProject('/oauth/projects/7/configuration')
    expect(
      await screen.findByRole('button', { name: 'Rotacionar chave' }),
    ).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Credenciais' })).toHaveAttribute(
      'aria-current',
      'page',
    )
  })

  it('uses the project list when an old provider link has no selection, showing provider status per row', async () => {
    renderProject('/oauth/providers')
    expect(
      await screen.findByRole('heading', { name: 'Projetos OAuth', level: 1 }),
    ).toBeInTheDocument()
    await waitFor(() =>
      expect(screen.getAllByText('Google ativo')).toHaveLength(2),
    )
    expect(screen.getByRole('link', { name: 'Acme' })).toHaveAttribute(
      'href',
      '/oauth/projects/7',
    )
    expect(
      screen.queryByLabelText('Escopos permitidos'),
    ).not.toBeInTheDocument()
  })

  it('requests activity using the selected project filter', async () => {
    renderProject('/oauth/projects/7/activity')
    expect(
      await screen.findByRole('cell', { name: 'project.updated' }),
    ).toBeInTheDocument()
    expect(controlRequest).toHaveBeenCalledWith(
      '/oauth/audit?project_id=7&limit=50',
    )
    expect(controlRequest).not.toHaveBeenCalledWith('/oauth/audit')
  })

  it('translates project navigation and the scoped save action into English', async () => {
    localStorage.setItem(
      'mw-control-preferences-v1',
      JSON.stringify({ language: 'en' }),
    )
    renderProject()
    expect(
      await screen.findByRole('heading', { name: 'Acme' }),
    ).toBeInTheDocument()
    expect(
      screen.getByRole('navigation', { name: 'Project navigation' }),
    ).toBeInTheDocument()
    expect(
      screen.getByRole('link', { name: 'Credentials' }),
    ).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Activity' })).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Save changes for Acme' }),
    ).toBeInTheDocument()
  })
})
