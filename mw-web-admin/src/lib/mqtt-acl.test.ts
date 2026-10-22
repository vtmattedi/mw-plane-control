import { describe, expect, it } from 'vitest'
import {
  aclProblem,
  aclTypes,
  mqttRolePattern,
  validTopicFilter,
} from './mqtt-acl'
import { roleTemplates, templateById } from './mqtt-templates'

describe('topic filters', () => {
  it('accepts ordinary filters, wildcards and substitutions', () => {
    for (const ok of [
      '#',
      '+',
      'a',
      'a/b/c',
      'a/+/c',
      'a/#',
      'devices/%u/#',
      'devices/%c/events/+',
      '$SYS/#',
      'Control/time',
    ]) {
      expect(validTopicFilter(ok), ok).toBe(true)
    }
  })

  it('rejects malformed filters', () => {
    for (const bad of [
      '',
      'a/#/b',
      'a#',
      'a/b#',
      'a+/b',
      'a/+b',
      'a\u0000b',
      'a\nb',
      'a'.repeat(1025),
    ]) {
      expect(validTopicFilter(bad), JSON.stringify(bad)).toBe(false)
    }
  })

  it('counts bytes, not characters', () => {
    expect(validTopicFilter('é'.repeat(512))).toBe(true)
    expect(validTopicFilter('é'.repeat(513))).toBe(false)
  })
})

describe('rule problems mirror the service', () => {
  it('refuses the control channel for every rule type', () => {
    for (const type of aclTypes) {
      expect(aclProblem({ type, topic: '$CONTROL/#' })).toBe('control_channel')
      expect(aclProblem({ type, topic: '$CONTROL/dynamic-security/v1' })).toBe(
        'control_channel',
      )
    }
  })

  it('refuses publishing into $ topics but allows reading them', () => {
    expect(aclProblem({ type: 'publishClientSend', topic: '$SYS/x' })).toBe(
      'reserved_publish',
    )
    expect(aclProblem({ type: 'subscribePattern', topic: '$SYS/#' })).toBeNull()
    expect(
      aclProblem({ type: 'publishClientReceive', topic: '$SYS/#' }),
    ).toBeNull()
  })

  it('reports the first problem in a stable order', () => {
    expect(aclProblem({ type: 'nope', topic: '' })).toBe('invalid_type')
    expect(aclProblem({ type: 'subscribePattern', topic: 'a/#/b' })).toBe(
      'invalid_topic',
    )
    expect(
      aclProblem({ type: 'subscribePattern', topic: 'a', priority: 1001 }),
    ).toBe('invalid_priority')
    expect(
      aclProblem({ type: 'subscribePattern', topic: 'a', priority: 1.5 }),
    ).toBe('invalid_priority')
    expect(
      aclProblem({ type: 'subscribePattern', topic: 'a', priority: -1000 }),
    ).toBeNull()
  })
})

describe('role names', () => {
  it('follows the service rule', () => {
    for (const ok of ['nmnw', 'a', 'app-1', 'Team.A_b', 'a'.repeat(64)]) {
      expect(mqttRolePattern.test(ok), ok).toBe(true)
    }
    for (const bad of ['', '-a', 'a b', 'a/b', 'ação', 'a'.repeat(65)]) {
      expect(mqttRolePattern.test(bad), bad).toBe(false)
    }
  })
})

describe('role templates', () => {
  it('only offer rules the service accepts', () => {
    for (const template of roleTemplates) {
      for (const acl of template.acls) {
        expect(
          aclProblem(acl),
          `${template.id}: ${acl.type} ${acl.topic}`,
        ).toBeNull()
      }
    }
  })

  it('are small enough to create in one request and have unique ids', () => {
    expect(new Set(roleTemplates.map((t) => t.id)).size).toBe(
      roleTemplates.length,
    )
    for (const template of roleTemplates) {
      expect(template.acls.length).toBeLessThanOrEqual(50)
    }
  })

  it('include per-client isolation using the username substitution', () => {
    const template = templateById('per-client')
    expect(template?.acls.length).toBe(4)
    expect(template?.acls.every((acl) => acl.topic === 'devices/%u/#')).toBe(
      true,
    )
  })

  it('never grant publishing in the read-only and monitoring templates', () => {
    for (const id of ['readonly', 'monitoring']) {
      expect(
        templateById(id)?.acls.some((acl) => acl.type === 'publishClientSend'),
        id,
      ).toBe(false)
    }
  })

  it('resolve by id', () => {
    expect(templateById('empty')?.acls).toEqual([])
    expect(templateById('missing')).toBeUndefined()
  })
})
