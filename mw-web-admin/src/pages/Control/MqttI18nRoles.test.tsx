import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { AppConfirmProvider } from '@/components/confirm-provider'
import { AppToaster } from '@/components/app-toaster'
import { ControlApiError, controlRequest } from '@/lib/control-api'
import { roleTemplates } from '@/lib/mqtt-templates'
import { PreferencesProvider } from '@/preferences/preferences-context'
import MqttClientsPage from './MqttClientsPage'
import MqttRolesPage from './MqttRolesPage'

// English UI, pinned so the test does not depend on the machine's language.
vi.hoisted(() => {
  Object.defineProperty(window.navigator, 'language', {
    configurable: true,
    value: 'en-US',
  })
})

vi.mock('@/lib/control-api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/lib/control-api')>()
  return { ...actual, controlRequest: vi.fn(), beginStepUp: vi.fn() }
})

// The source strings are Portuguese, so an accented letter in the English UI is a string
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
    ...[...document.querySelectorAll('option')].map(
      (el) => el.textContent ?? '',
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

const roleRow = (name: string, extra: Record<string, unknown> = {}) => ({
  name,
  description: '',
  acls: [],
  reserved: false,
  assignable: true,
  clientCount: 0,
  ...extra,
})

const nmnw = roleRow('nmnw', {
  description: 'Trusted gateways',
  clientCount: 2,
  acls: [
    { type: 'publishClientSend', topic: '#', allow: true, priority: 0 },
    { type: 'subscribePattern', topic: 'a/#', allow: false, priority: 4 },
  ],
})
const admin = roleRow('admin', {
  reserved: true,
  assignable: false,
  clientCount: 1,
})
const viewer = roleRow('viewer')
const legacy = roleRow('legacy', { assignable: false }) // outside an allow-list: "Restricted"

const api: Record<string, unknown> = {
  '/mqtt/roles': {
    roles: [nmnw, admin, viewer, legacy],
    assignable: ['nmnw', 'viewer'],
  },
  '/mqtt/roles/nmnw': { ...nmnw, clients: ['gw1', 'gw2'] },
  '/mqtt/roles/admin': { ...admin, clients: ['mqtt-admin'] },
  '/mqtt/roles/legacy': { ...legacy, clients: [] },
  '/mqtt/clients': {
    clients: [
      { username: 'gw1', disabled: false, roles: ['nmnw'] },
      { username: 'mqtt-admin', disabled: false, roles: ['admin'] },
    ],
  },
  '/mqtt/stats': { available: true, stale: false, clients: {}, store: {} },
}

function useApi(overrides: Record<string, unknown> = {}) {
  vi.mocked(controlRequest).mockImplementation((async (path: string) => {
    const value = path in overrides ? overrides[path] : api[path]
    if (value instanceof Error) throw value
    return value ?? null
  }) as never)
}

function renderPage(page: 'roles' | 'clients') {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <PreferencesProvider>
        <AppConfirmProvider>
          <AppToaster />
          <MemoryRouter>
            {page === 'roles' ? <MqttRolesPage /> : <MqttClientsPage />}
          </MemoryRouter>
        </AppConfirmProvider>
      </PreferencesProvider>
    </QueryClientProvider>,
  )
}

function rowOf(name: string) {
  return within(screen.getByText(name).closest('tr') as HTMLElement)
}

beforeEach(() => {
  vi.clearAllMocks()
  sessionStorage.clear()
  localStorage.clear()
})

describe('the roles feature respects the English language', () => {
  it('roles list, create dialog with every template, and its validation hints', async () => {
    const user = userEvent.setup()
    useApi()
    renderPage('roles')
    await screen.findByText('nmnw')
    expect(screen.getByRole('button', { name: 'New role' })).toBeVisible()
    expect(rowOf('admin').getByText('Reserved')).toBeInTheDocument()
    expect(rowOf('legacy').getByText('Restricted')).toBeInTheDocument()
    expectNoPortuguese('roles table')

    await user.click(screen.getByRole('button', { name: 'New role' }))
    const dialog = await screen.findByRole('dialog', { name: 'New role' })
    expectNoPortuguese('create dialog')

    const name = within(dialog).getByPlaceholderText('data-viewer')
    await user.type(name, '-bad')
    expectNoPortuguese('create dialog with an invalid name')
    await user.clear(name)
    await user.type(name, 'nmnw')
    expectNoPortuguese('create dialog with a taken name')

    // Every template, with its name, description and rule preview.
    for (const template of roleTemplates) {
      const buttons = within(dialog).getAllByRole('button', { pressed: false })
      const target = buttons.find(
        (b) => b.getAttribute('aria-pressed') === 'false' && b.textContent,
      )
      expect(target).toBeDefined()
      await user.click(target as HTMLElement)
      expectNoPortuguese(`template ${template.id}`)
    }
  })

  it('role details: reserved and editable roles, the add-rule form and its errors', async () => {
    const user = userEvent.setup()
    useApi()
    renderPage('roles')
    await screen.findByText('nmnw')

    await user.click(rowOf('admin').getByRole('button', { name: 'Details' }))
    const reserved = await screen.findByRole('dialog', { name: /Role · admin/ })
    await within(reserved).findByText('mqtt-admin')
    expectNoPortuguese('reserved role detail')
    await user.click(within(reserved).getByRole('button', { name: 'Done' }))

    await user.click(rowOf('nmnw').getByRole('button', { name: 'Details' }))
    const dialog = await screen.findByRole('dialog', { name: /Role · nmnw/ })
    await within(dialog).findByText('gw1')
    expect(
      within(within(dialog).getByRole('table')).getByText('Deny'),
    ).toBeInTheDocument()
    expectNoPortuguese('editable role detail')

    const topic = within(dialog).getByPlaceholderText('devices/%u/#')
    for (const bad of ['$CONTROL/#', 'a/#/b']) {
      await user.clear(topic)
      await user.type(topic, bad)
      expect(within(dialog).getByRole('alert')).toBeInTheDocument()
      expectNoPortuguese(`rule error for ${bad}`)
    }
    const typeSelect = within(dialog).getAllByRole('combobox')[0]
    await user.selectOptions(typeSelect, 'publishClientSend')
    await user.clear(topic)
    await user.type(topic, '$SYS/x')
    expectNoPortuguese('rule error for publishing into $SYS')
  })

  it('delete and remove-rule confirmations', async () => {
    const user = userEvent.setup()
    useApi()
    renderPage('roles')
    await screen.findByText('nmnw')

    await user.click(rowOf('nmnw').getByRole('button', { name: 'Delete' }))
    const deleting = await screen.findByRole('alertdialog', {
      name: 'Delete role?',
    })
    expect(within(deleting).getByText(/used by 2 clients/)).toBeInTheDocument()
    expectNoPortuguese('delete a role in use')
    await user.click(within(deleting).getByRole('button', { name: 'Cancel' }))
    await waitFor(() =>
      expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument(),
    )

    await user.click(rowOf('viewer').getByRole('button', { name: 'Delete' }))
    const unused = await screen.findByRole('alertdialog', {
      name: 'Delete role?',
    })
    expectNoPortuguese('delete an unused role')
    await user.click(within(unused).getByRole('button', { name: 'Cancel' }))
    await waitFor(() =>
      expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument(),
    )

    await user.click(rowOf('nmnw').getByRole('button', { name: 'Details' }))
    const dialog = await screen.findByRole('dialog', { name: /Role · nmnw/ })
    await user.click(
      await within(dialog).findByRole('button', {
        name: 'Remove rule publishClientSend #',
      }),
    )
    const removing = await screen.findByRole('alertdialog', {
      name: 'Remove rule?',
    })
    expect(
      within(removing).getByText(/2 clients use this role/),
    ).toBeInTheDocument()
    expectNoPortuguese('remove a rule')
  })

  it('the translated error for a refused request', async () => {
    const user = userEvent.setup()
    useApi()
    renderPage('roles')
    await screen.findByText('nmnw')
    vi.mocked(controlRequest).mockRejectedValueOnce(
      new ControlApiError(
        'Nenhuma função pode conceder o canal de controle do broker nem publicar em tópicos $.',
        403,
        'forbidden_topic',
      ),
    )
    await user.click(screen.getByRole('button', { name: 'New role' }))
    const dialog = await screen.findByRole('dialog', { name: 'New role' })
    await user.type(within(dialog).getByPlaceholderText('data-viewer'), 'blank')
    await user.click(
      within(dialog).getByRole('button', { name: 'Create role' }),
    )
    expect(
      await screen.findByText(
        "No role can grant the broker's control channel or publish to $ topics.",
      ),
    ).toBeInTheDocument()
    expectNoPortuguese('translated service error')
  })

  it('the clients page roles dialog and its confirmation', async () => {
    const user = userEvent.setup()
    useApi()
    renderPage('clients')
    await screen.findByText('gw1')
    expect(
      rowOf('gw1').getByRole('button', { name: 'Roles' }),
    ).toBeInTheDocument()

    await user.click(rowOf('gw1').getByRole('button', { name: 'Roles' }))
    const dialog = await screen.findByRole('dialog', {
      name: /Client roles · gw1/,
    })
    expect(
      within(dialog).getByRole('button', { name: 'Add' }),
    ).toBeInTheDocument()
    expectNoPortuguese('client roles dialog')

    await user.click(within(dialog).getByRole('button', { name: 'Remove' }))
    await screen.findByRole('alertdialog', { name: 'Remove role?' })
    expectNoPortuguese('remove a client role')
  })
})
