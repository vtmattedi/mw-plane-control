import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { beforeEach, expect, it, vi } from 'vitest'
import { AppConfirmProvider } from '@/components/confirm-provider'
import { controlRequest } from '@/lib/control-api'
import { PreferencesProvider } from '@/preferences/preferences-context'
import { IdentityApplicationDetailPage } from './IdentityDetailPages'

vi.hoisted(() => {
  Object.defineProperty(window.navigator, 'language', {
    configurable: true,
    value: 'pt-BR',
  })
})
vi.mock('@/lib/control-api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/control-api')>()),
  controlRequest: vi.fn(),
}))
// Exercise the page's access decisions through native select interactions.
vi.mock('@/components/ui/select', () => ({
  Select: (props: React.SelectHTMLAttributes<HTMLSelectElement>) => (
    <select {...props} />
  ),
}))

beforeEach(() => {
  vi.clearAllMocks()
  localStorage.clear()
  sessionStorage.clear()
})

async function renderAccess(administration: boolean) {
  vi.mocked(controlRequest).mockImplementation((async (
    path: string,
    init?: RequestInit,
  ) => {
    if (init?.method === 'PUT') return {}
    if (path === '/identity/users')
      return {
        users: [
          {
            id: 'global',
            display_name: 'Global',
            email: 'global@example.test',
            is_superadmin: true,
          },
          {
            id: 'staff',
            display_name: 'Staff',
            email: 'staff@example.test',
            is_superadmin: false,
          },
        ],
      }
    if (path.endsWith('/assignments'))
      return {
        assignments: [
          { user_id: 'global', role: 'user', access_effect: 'allow' },
          { user_id: 'staff', role: 'admin', access_effect: 'allow' },
        ],
      }
    return {
      id: 'app',
      name: 'Application',
      client_id: 'app',
      application_type: 'public',
      access_policy: 'default_allow',
      enabled: true,
      internal: administration,
      audiences: administration ? ['mw-identity-admin'] : [],
      scopes: ['openid'],
      claims: [],
      redirect_uris: [],
      post_logout_redirect_uris: [],
      created_at: '2026-11-03T00:00:00Z',
      updated_at: '2026-11-03T00:00:00Z',
    }
  }) as typeof controlRequest)
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <PreferencesProvider>
        <AppConfirmProvider>
          <MemoryRouter initialEntries={['/identity/applications/app']}>
            <Routes>
              <Route
                path="/identity/applications/:applicationID"
                element={<IdentityApplicationDetailPage />}
              />
            </Routes>
          </MemoryRouter>
        </AppConfirmProvider>
      </PreferencesProvider>
    </QueryClientProvider>,
  )
  const title = await screen.findByText('Exceções de acesso')
  const card = within(title.parentElement!.parentElement!)
  await card.findByText('Global')
  return card
}

it('allows ordinary-app superadmin denial through the form and preserves administrator roles when blocking', async () => {
  const user = userEvent.setup()
  const card = await renderAccess(false)
  const [subject, role, effect] = card.getAllByRole('combobox')
  await user.selectOptions(subject, 'global')
  await user.selectOptions(role, 'admin')
  expect(effect).toBeEnabled()
  await user.selectOptions(effect, 'deny')
  await user.click(card.getByRole('button', { name: 'Salvar exceção' }))
  await waitFor(() =>
    expect(controlRequest).toHaveBeenCalledWith(
      '/identity/applications/app/assignments/global',
      expect.objectContaining({
        method: 'PUT',
        body: JSON.stringify({ role: 'admin', access_effect: 'deny' }),
      }),
    ),
  )
  const buttons = card.getAllByRole('button', { name: 'Bloquear' })
  expect(buttons[0]).toBeEnabled()
  expect(buttons[1]).toBeEnabled()
  await user.click(buttons[1])
  await waitFor(() =>
    expect(controlRequest).toHaveBeenCalledWith(
      '/identity/applications/app/assignments/staff',
      expect.objectContaining({
        method: 'PUT',
        body: JSON.stringify({ role: 'admin', access_effect: 'deny' }),
      }),
    ),
  )
})

it('preserves MW administration protections for global and existing application administrators', async () => {
  const user = userEvent.setup()
  const card = await renderAccess(true)
  for (const button of card.getAllByRole('button', { name: 'Bloquear' })) {
    expect(button).toBeDisabled()
  }
  const [subject, , effect] = card.getAllByRole('combobox')
  await user.selectOptions(subject, 'global')
  expect(effect).toBeDisabled()
  await user.selectOptions(subject, 'staff')
  expect(effect).toBeDisabled()
})
