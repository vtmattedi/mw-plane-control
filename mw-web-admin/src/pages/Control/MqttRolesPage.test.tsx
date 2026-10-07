import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { AppConfirmProvider } from '@/components/confirm-provider'
import { beginStepUp, ControlApiError, controlRequest } from '@/lib/control-api'
import { pendingOperationKey } from '@/lib/use-mqtt-operations'
import { PreferencesProvider } from '@/preferences/preferences-context'
import MqttRolesPage from './MqttRolesPage'

// Portuguese is the app's source language; the default is read at import time.
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

type Handler = (path: string, init?: RequestInit) => unknown

const role = (
  name: string,
  extra: Record<string, unknown> = {},
): Record<string, unknown> => ({
  name,
  description: '',
  acls: [],
  reserved: false,
  assignable: true,
  clientCount: 0,
  ...extra,
})

const nmnw = role('nmnw', {
  description: 'Trusted gateways',
  clientCount: 2,
  acls: [{ type: 'publishClientSend', topic: '#', allow: true, priority: 0 }],
})
const admin = role('admin', {
  reserved: true,
  assignable: false,
  clientCount: 1,
  acls: [
    {
      type: 'publishClientSend',
      topic: '$CONTROL/dynamic-security/#',
      allow: true,
      priority: 0,
    },
  ],
})
const viewer = role('viewer', { clientCount: 0 })

const baseHandler: Handler = (path) => {
  switch (path) {
    case '/mqtt/roles':
      return { roles: [nmnw, admin, viewer], assignable: ['nmnw', 'viewer'] }
    case '/mqtt/roles/nmnw':
      return { ...nmnw, clients: ['gw1', 'gw2'] }
    case '/mqtt/roles/viewer':
      return { ...viewer, clients: [] }
    case '/mqtt/roles/admin':
      return { ...admin, clients: ['mqtt-admin'] }
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

function lastMutation() {
  const calls = mutationCalls()
  const [path, init] = calls[calls.length - 1]
  return {
    path,
    method: init?.method,
    body: init?.body ? JSON.parse(String(init.body)) : undefined,
  }
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
            <MqttRolesPage />
          </MemoryRouter>
        </AppConfirmProvider>
      </PreferencesProvider>
    </QueryClientProvider>,
  )
}

function rowOf(name: string) {
  const cell = screen.getAllByText(name).find((el) => el.closest('tr'))
  expect(cell, `row for ${name}`).toBeDefined()
  return within(cell!.closest('tr') as HTMLElement)
}

beforeEach(() => {
  vi.clearAllMocks()
  sessionStorage.clear()
  localStorage.clear()
})

describe('role list', () => {
  it('shows every role with its flags, and no way to delete a reserved one', async () => {
    useApi()
    renderPage()

    await screen.findByText('nmnw')
    expect(rowOf('nmnw').getByText('Atribuível')).toBeInTheDocument()
    expect(rowOf('admin').getByText('Reservada')).toBeInTheDocument()
    expect(
      rowOf('nmnw').getByRole('button', { name: 'Excluir' }),
    ).toBeInTheDocument()
    expect(
      rowOf('viewer').getByRole('button', { name: 'Excluir' }),
    ).toBeInTheDocument()
    expect(rowOf('admin').queryByRole('button', { name: 'Excluir' })).toBeNull()
    // Every role can be inspected, even a reserved one.
    for (const name of ['nmnw', 'admin', 'viewer']) {
      expect(
        rowOf(name).getByRole('button', { name: 'Detalhes' }),
      ).toBeInTheDocument()
    }
  })
})

describe('creating a role', () => {
  it('validates the name and creates the role with the chosen template', async () => {
    const user = userEvent.setup()
    useApi()
    renderPage()
    await screen.findByText('nmnw')

    await user.click(screen.getByRole('button', { name: 'Nova função' }))
    const dialog = await screen.findByRole('dialog', { name: 'Nova função' })
    const create = within(dialog).getByRole('button', { name: 'Criar função' })
    const name = within(dialog).getByPlaceholderText('data-viewer')

    expect(create).toBeDisabled()
    await user.type(name, '-bad name')
    expect(create).toBeDisabled()
    expect(within(dialog).getByText(/Use letras, números/)).toBeInTheDocument()

    await user.clear(name)
    await user.type(name, 'nmnw')
    expect(
      within(dialog).getByText('Já existe uma função com este nome.'),
    ).toBeInTheDocument()
    expect(create).toBeDisabled()

    await user.clear(name)
    await user.type(name, 'devices')
    expect(create).toBeEnabled()

    // The default is an empty role; picking a template previews and sends its rules.
    expect(within(dialog).getByText('Regras iniciais (0)')).toBeInTheDocument()
    await user.click(
      within(dialog).getByRole('button', { name: /Isolado por cliente/ }),
    )
    expect(within(dialog).getByText('Regras iniciais (4)')).toBeInTheDocument()
    expect(within(dialog).getAllByText('devices/%u/#')).toHaveLength(4)
    expect(
      within(dialog).getByRole('button', { name: /Isolado por cliente/ }),
    ).toHaveAttribute('aria-pressed', 'true')

    await user.type(within(dialog).getAllByRole('textbox')[1], '  Per client  ')
    await user.click(create)

    await waitFor(() => expect(mutationCalls()).toHaveLength(1))
    const sent = lastMutation()
    expect(sent.path).toBe('/mqtt/roles')
    expect(sent.method).toBe('POST')
    expect(sent.body.name).toBe('devices')
    expect(sent.body.description).toBe('Per client')
    expect(sent.body.acls).toHaveLength(4)
    expect(sent.body.acls.map((a: { type: string }) => a.type).sort()).toEqual([
      'publishClientReceive',
      'publishClientSend',
      'subscribePattern',
      'unsubscribePattern',
    ])
  })

  it('creates an empty role when no template is chosen', async () => {
    const user = userEvent.setup()
    useApi()
    renderPage()
    await screen.findByText('nmnw')
    await user.click(screen.getByRole('button', { name: 'Nova função' }))
    const dialog = await screen.findByRole('dialog', { name: 'Nova função' })
    await user.type(within(dialog).getByPlaceholderText('data-viewer'), 'blank')
    await user.click(
      within(dialog).getByRole('button', { name: 'Criar função' }),
    )
    await waitFor(() => expect(mutationCalls()).toHaveLength(1))
    expect(lastMutation().body).toEqual({ name: 'blank', acls: [] })
  })
})

describe('deleting a role', () => {
  it('warns how many clients lose access and then forces the delete', async () => {
    const user = userEvent.setup()
    useApi()
    renderPage()
    await screen.findByText('nmnw')

    await user.click(rowOf('nmnw').getByRole('button', { name: 'Excluir' }))
    const confirm = await screen.findByRole('alertdialog', {
      name: 'Excluir função?',
    })
    expect(
      within(confirm).getByText(/Esta função está em uso por 2 clientes/),
    ).toBeInTheDocument()
    expect(mutationCalls()).toHaveLength(0)

    await user.click(within(confirm).getByRole('button', { name: 'Excluir' }))
    await waitFor(() => expect(mutationCalls()).toHaveLength(1))
    expect(lastMutation()).toMatchObject({
      path: '/mqtt/roles/nmnw?force=true',
      method: 'DELETE',
    })
  })

  it('deletes an unused role without force', async () => {
    const user = userEvent.setup()
    useApi()
    renderPage()
    await screen.findByText('viewer')
    await user.click(rowOf('viewer').getByRole('button', { name: 'Excluir' }))
    const confirm = await screen.findByRole('alertdialog', {
      name: 'Excluir função?',
    })
    expect(
      within(confirm).getByText(/Nenhum cliente usa esta função/),
    ).toBeInTheDocument()
    await user.click(within(confirm).getByRole('button', { name: 'Excluir' }))
    await waitFor(() => expect(mutationCalls()).toHaveLength(1))
    expect(lastMutation()).toMatchObject({
      path: '/mqtt/roles/viewer',
      method: 'DELETE',
    })
  })

  it('sends nothing when the confirmation is cancelled', async () => {
    const user = userEvent.setup()
    useApi()
    renderPage()
    await screen.findByText('nmnw')
    await user.click(rowOf('nmnw').getByRole('button', { name: 'Excluir' }))
    const confirm = await screen.findByRole('alertdialog', {
      name: 'Excluir função?',
    })
    await user.click(within(confirm).getByRole('button', { name: 'Cancelar' }))
    expect(mutationCalls()).toHaveLength(0)
  })
})

describe('role details and rules', () => {
  async function openDetail(
    user: ReturnType<typeof userEvent.setup>,
    name: string,
  ) {
    await screen.findByText(name)
    await user.click(rowOf(name).getByRole('button', { name: 'Detalhes' }))
    return screen.findByRole('dialog', { name: new RegExp(`Função · ${name}`) })
  }

  it('shows the rules and the clients that hold the role', async () => {
    const user = userEvent.setup()
    useApi()
    renderPage()
    const dialog = await openDetail(user, 'nmnw')
    expect(await within(dialog).findByText('gw1')).toBeInTheDocument()
    expect(within(dialog).getByText('gw2')).toBeInTheDocument()
    expect(within(dialog).getByText('Clientes (2)')).toBeInTheDocument()
    expect(within(dialog).getByText('Regras (1)')).toBeInTheDocument()
    // Scoped to the rules table: the add-rule form lists the same type names.
    const rules = within(within(dialog).getByRole('table'))
    expect(rules.getByText('publishClientSend')).toBeInTheDocument()
    expect(rules.getByText('O cliente publica')).toBeInTheDocument()
  })

  it('offers no editing for a reserved role', async () => {
    const user = userEvent.setup()
    useApi()
    renderPage()
    const dialog = await openDetail(user, 'admin')
    await within(dialog).findByText('mqtt-admin')
    expect(
      within(dialog).getByText('Função do broker: não pode ser alterada aqui.'),
    ).toBeInTheDocument()
    expect(within(dialog).queryByRole('button', { name: /Remover/ })).toBeNull()
    expect(within(dialog).queryByText('Adicionar regra')).toBeNull()
  })

  it('refuses the control channel in the form before sending anything', async () => {
    const user = userEvent.setup()
    useApi()
    renderPage()
    const dialog = await openDetail(user, 'viewer')
    const topic = await within(dialog).findByPlaceholderText('devices/%u/#')
    const add = within(dialog).getByRole('button', { name: 'Adicionar regra' })

    expect(add).toBeDisabled()
    await user.type(topic, '$CONTROL/dynamic-security/#')
    expect(within(dialog).getByRole('alert')).toHaveTextContent(
      /canal de controle do broker/,
    )
    expect(add).toBeDisabled()

    await user.clear(topic)
    await user.type(topic, 'a/#/b')
    expect(within(dialog).getByRole('alert')).toHaveTextContent(
      /Filtro de tópico inválido/,
    )
    expect(add).toBeDisabled()
    expect(mutationCalls()).toHaveLength(0)
  })

  it('adds a rule', async () => {
    const user = userEvent.setup()
    useApi()
    renderPage()
    const dialog = await openDetail(user, 'viewer')
    const topic = await within(dialog).findByPlaceholderText('devices/%u/#')
    await user.type(topic, 'data/#')
    expect(within(dialog).queryByRole('alert')).toBeNull()
    await user.click(
      within(dialog).getByRole('button', { name: 'Adicionar regra' }),
    )

    await waitFor(() => expect(mutationCalls()).toHaveLength(1))
    expect(lastMutation()).toEqual({
      path: '/mqtt/roles/viewer/acls',
      method: 'POST',
      body: {
        type: 'subscribePattern',
        topic: 'data/#',
        allow: true,
        priority: 0,
      },
    })
  })

  it('removes a rule after warning how many clients are affected', async () => {
    const user = userEvent.setup()
    useApi()
    renderPage()
    const dialog = await openDetail(user, 'nmnw')
    await user.click(
      await within(dialog).findByRole('button', {
        name: 'Remover regra publishClientSend #',
      }),
    )
    const confirm = await screen.findByRole('alertdialog', {
      name: 'Remover regra?',
    })
    expect(
      within(confirm).getByText(/2 clientes usam esta função/),
    ).toBeInTheDocument()
    expect(mutationCalls()).toHaveLength(0)

    await user.click(within(confirm).getByRole('button', { name: 'Remover' }))
    await waitFor(() => expect(mutationCalls()).toHaveLength(1))
    expect(lastMutation()).toEqual({
      path: '/mqtt/roles/nmnw/acls/remove',
      method: 'POST',
      body: { type: 'publishClientSend', topic: '#' },
    })
  })
})

describe('step-up', () => {
  it('saves a role change and resumes it exactly once after the step-up', async () => {
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
    await screen.findByText('nmnw')
    await user.click(screen.getByRole('button', { name: 'Nova função' }))
    const dialog = await screen.findByRole('dialog', { name: 'Nova função' })
    await user.type(within(dialog).getByPlaceholderText('data-viewer'), 'blank')
    await user.click(
      within(dialog).getByRole('button', { name: 'Criar função' }),
    )

    await waitFor(() => expect(beginStepUp).toHaveBeenCalledWith('/'))
    expect(
      JSON.parse(sessionStorage.getItem(pendingOperationKey) ?? 'null'),
    ).toEqual({
      operation: { kind: 'createRole', name: 'blank', acls: [] },
      returnTo: '/',
    })
  })

  it('resumes a saved role operation when the page loads after step-up', async () => {
    useApi()
    sessionStorage.setItem(
      pendingOperationKey,
      JSON.stringify({
        operation: { kind: 'deleteRole', name: 'viewer', force: false },
        returnTo: '/',
      }),
    )
    renderPage()
    await waitFor(() => expect(mutationCalls()).toHaveLength(1))
    expect(lastMutation()).toMatchObject({
      path: '/mqtt/roles/viewer',
      method: 'DELETE',
    })
    expect(sessionStorage.getItem(pendingOperationKey)).toBeNull()
  })

  it('discards a saved operation that does not validate', async () => {
    useApi()
    sessionStorage.setItem(
      pendingOperationKey,
      JSON.stringify({
        operation: {
          kind: 'addAcl',
          role: 'viewer',
          acl: { type: 'subscribePattern', topic: '$CONTROL/#/x' },
        },
        returnTo: '/',
      }),
    )
    renderPage()
    await screen.findByText('nmnw')
    expect(mutationCalls()).toHaveLength(0)
    expect(sessionStorage.getItem(pendingOperationKey)).toBeNull()
  })
})
