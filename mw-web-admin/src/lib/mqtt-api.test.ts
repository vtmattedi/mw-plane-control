import { beforeEach, describe, expect, it, vi } from 'vitest'
import { controlRequest } from '@/lib/control-api'
import {
  fetchMqttRole,
  fetchMqttRoles,
  runMqttOperation,
  validateOperation,
  type MqttOperation,
} from './mqtt-api'

vi.mock('@/lib/control-api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/lib/control-api')>()
  return { ...actual, controlRequest: vi.fn() }
})

const request = vi.mocked(controlRequest)

beforeEach(() => {
  request.mockReset()
  request.mockResolvedValue(null as never)
})

function lastCall() {
  const [path, init] = request.mock.calls[request.mock.calls.length - 1]
  return {
    path,
    method: init?.method ?? 'GET',
    body: init?.body ? JSON.parse(String(init.body)) : undefined,
  }
}

describe('runMqttOperation sends the expected request', () => {
  const cases: [MqttOperation, string, string, unknown][] = [
    [
      { kind: 'assignRole', username: 'gw1', role: 'viewer' },
      'PUT',
      '/mqtt/clients/gw1/roles/viewer',
      undefined,
    ],
    [
      { kind: 'unassignRole', username: 'gw1', role: 'viewer' },
      'DELETE',
      '/mqtt/clients/gw1/roles/viewer',
      undefined,
    ],
    [
      { kind: 'deleteRole', name: 'viewer', force: false },
      'DELETE',
      '/mqtt/roles/viewer',
      undefined,
    ],
    [
      { kind: 'deleteRole', name: 'viewer', force: true },
      'DELETE',
      '/mqtt/roles/viewer?force=true',
      undefined,
    ],
    [
      {
        kind: 'addAcl',
        role: 'viewer',
        acl: {
          type: 'subscribePattern',
          topic: 'a/#',
          allow: false,
          priority: 3,
        },
      },
      'POST',
      '/mqtt/roles/viewer/acls',
      { type: 'subscribePattern', topic: 'a/#', allow: false, priority: 3 },
    ],
    [
      {
        kind: 'addAcl',
        role: 'viewer',
        acl: { type: 'subscribePattern', topic: 'a/#' },
      },
      'POST',
      '/mqtt/roles/viewer/acls',
      { type: 'subscribePattern', topic: 'a/#' },
    ],
    [
      {
        kind: 'removeAcl',
        role: 'viewer',
        type: 'subscribePattern',
        topic: 'a/#',
      },
      'POST',
      '/mqtt/roles/viewer/acls/remove',
      { type: 'subscribePattern', topic: 'a/#' },
    ],
    [
      {
        kind: 'createRole',
        name: 'devices',
        description: 'Per-client',
        acls: [{ type: 'publishClientSend', topic: 'devices/%u/#' }],
      },
      'POST',
      '/mqtt/roles',
      {
        name: 'devices',
        description: 'Per-client',
        acls: [{ type: 'publishClientSend', topic: 'devices/%u/#' }],
      },
    ],
    [
      { kind: 'createRole', name: 'empty', acls: [] },
      'POST',
      '/mqtt/roles',
      { name: 'empty', acls: [] },
    ],
  ]
  for (const [operation, method, path, body] of cases) {
    it(`${operation.kind} -> ${method} ${path}`, async () => {
      await runMqttOperation(operation)
      expect(lastCall()).toEqual({ path, method, body })
    })
  }

  it('escapes names so they cannot change the path', async () => {
    await runMqttOperation({
      kind: 'unassignRole',
      username: 'a.b',
      role: 'r_1',
    })
    expect(lastCall().path).toBe('/mqtt/clients/a.b/roles/r_1')
  })
})

describe('validateOperation', () => {
  const good: unknown[] = [
    { kind: 'create', username: 'gw1' },
    { kind: 'create', username: 'gw1', role: 'nmnw' },
    { kind: 'rotate', username: 'gw1' },
    { kind: 'enable', username: 'gw1' },
    { kind: 'disable', username: 'gw1' },
    { kind: 'delete', username: 'gw1' },
    { kind: 'assignRole', username: 'gw1', role: 'viewer' },
    { kind: 'unassignRole', username: 'gw1', role: 'viewer' },
    { kind: 'deleteRole', name: 'viewer', force: true },
    {
      kind: 'addAcl',
      role: 'viewer',
      acl: { type: 'subscribePattern', topic: 'a' },
    },
    { kind: 'removeAcl', role: 'viewer', type: 'subscribePattern', topic: 'a' },
    {
      kind: 'createRole',
      name: 'r1',
      description: 'd',
      acls: [{ type: 'subscribePattern', topic: 'a' }],
    },
  ]
  for (const op of good) {
    it(`accepts ${(op as { kind: string }).kind}`, () => {
      expect(validateOperation(op)).toEqual(op)
    })
  }

  it('rejects anything malformed or tampered with', () => {
    const bad: unknown[] = [
      null,
      'delete',
      42,
      {},
      { kind: 'unknown' },
      { kind: 'delete' },
      { kind: 'delete', username: '../etc/passwd' },
      { kind: 'delete', username: 'a b' },
      { kind: 'assignRole', username: 'gw1' },
      { kind: 'assignRole', username: 'gw1', role: '-bad' },
      { kind: 'deleteRole', name: 'viewer' },
      { kind: 'deleteRole', name: 'viewer', force: 'yes' },
      { kind: 'addAcl', role: 'viewer', acl: { type: 'bogus', topic: 'a' } },
      {
        kind: 'addAcl',
        role: 'viewer',
        acl: { type: 'subscribePattern', topic: 'a/#/b' },
      },
      {
        kind: 'addAcl',
        role: 'viewer',
        acl: { type: 'subscribePattern', topic: 'a', priority: 5000 },
      },
      {
        kind: 'addAcl',
        role: 'viewer',
        acl: { type: 'subscribePattern', topic: 'a', allow: 'no' },
      },
      { kind: 'removeAcl', role: 'viewer', type: 'x', topic: 'a' },
      { kind: 'createRole', name: 'r1' },
      { kind: 'createRole', name: 'r1', acls: 'nope' },
      { kind: 'createRole', name: 'r1', acls: [{ type: 'bogus', topic: 'a' }] },
      {
        kind: 'createRole',
        name: 'r1',
        acls: new Array(51).fill({ type: 'subscribePattern', topic: 'a' }),
      },
      {
        kind: 'createRole',
        name: 'r1',
        description: 'x'.repeat(201),
        acls: [],
      },
    ]
    for (const op of bad) {
      expect(validateOperation(op), JSON.stringify(op)).toBeNull()
    }
  })

  it('rebuilds the operation so unknown fields are dropped', () => {
    const op = validateOperation({
      kind: 'assignRole',
      username: 'gw1',
      role: 'viewer',
      injected: { path: '/admin' },
    })
    expect(op).toEqual({ kind: 'assignRole', username: 'gw1', role: 'viewer' })
    expect(op).not.toHaveProperty('injected')
  })
})

describe('role parsing', () => {
  it('reads flags, counts and members from the service', async () => {
    request.mockResolvedValueOnce({
      roles: [
        {
          name: 'nmnw',
          description: 'Trusted',
          acls: [
            { type: 'publishClientSend', topic: '#', allow: true, priority: 0 },
          ],
          reserved: false,
          assignable: true,
          clientCount: 2,
        },
        {
          name: 'admin',
          acls: [],
          reserved: true,
          assignable: false,
          clientCount: 1,
        },
      ],
      assignable: ['nmnw'],
    } as never)
    const roles = await fetchMqttRoles()
    expect(roles.assignable).toEqual(['nmnw'])
    expect(roles.roles[0]).toMatchObject({
      name: 'nmnw',
      description: 'Trusted',
      reserved: false,
      assignable: true,
      clientCount: 2,
    })
    expect(roles.roles[1]).toMatchObject({
      name: 'admin',
      reserved: true,
      assignable: false,
    })
  })

  it('reads one role with its members', async () => {
    request.mockResolvedValueOnce({
      name: 'nmnw',
      acls: [],
      reserved: false,
      assignable: true,
      clientCount: 2,
      clients: ['gw1', 'gw2'],
    } as never)
    const role = await fetchMqttRole('nmnw')
    expect(role.clients).toEqual(['gw1', 'gw2'])
    expect(request).toHaveBeenCalledWith('/mqtt/roles/nmnw')
  })

  it('tolerates missing fields from an older service', async () => {
    request.mockResolvedValueOnce({
      roles: [{ name: 'x', acls: [{ type: 'a', topic: 'b' }] }],
    } as never)
    const roles = await fetchMqttRoles()
    expect(roles.roles[0]).toMatchObject({
      reserved: false,
      assignable: false,
      clientCount: 0,
      description: '',
    })
    expect(roles.assignable).toEqual([])
  })
})
