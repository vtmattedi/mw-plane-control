// Client-side checks for role rules. They mirror what the MQTT admin service enforces so the
// console can explain a problem before sending anything. The service remains the authority:
// nothing here is a security boundary.

export const aclTypes = [
  'publishClientSend',
  'publishClientReceive',
  'subscribeLiteral',
  'subscribePattern',
  'unsubscribeLiteral',
  'unsubscribePattern',
] as const

export type AclType = (typeof aclTypes)[number]

export type AclInput = {
  type: AclType
  topic: string
  allow?: boolean
  priority?: number
}

/** What each rule type controls, as translation keys (Portuguese is the source language). */
export const aclTypeHelp: Record<AclType, string> = {
  publishClientSend: 'O cliente publica',
  publishClientReceive: 'O broker entrega ao cliente',
  subscribeLiteral: 'O cliente assina (filtro exato)',
  subscribePattern: 'O cliente assina (filtros que combinam)',
  unsubscribeLiteral: 'O cliente remove assinatura (filtro exato)',
  unsubscribePattern: 'O cliente remove assinatura (filtros que combinam)',
}

export const mqttRolePattern = /^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/

export const maxTopicBytes = 1024
export const maxPriority = 1000

export function isAclType(value: unknown): value is AclType {
  return (
    typeof value === 'string' && (aclTypes as readonly string[]).includes(value)
  )
}

/**
 * A well-formed topic filter: non-empty, bounded, printable, with '+' only as a whole level
 * and '#' only as the whole last level. %u and %c are ordinary characters here.
 */
export function validTopicFilter(topic: string): boolean {
  if (topic === '' || new TextEncoder().encode(topic).length > maxTopicBytes) {
    return false
  }
  for (const char of topic) {
    const code = char.codePointAt(0) ?? 0
    if (code < 0x20 || code === 0x7f) return false
  }
  const levels = topic.split('/')
  for (let i = 0; i < levels.length; i++) {
    const level = levels[i]
    if (level.includes('#')) {
      if (level !== '#' || i !== levels.length - 1) return false
    } else if (level.includes('+') && level !== '+') {
      return false
    }
  }
  return true
}

export type AclProblem =
  | 'invalid_type'
  | 'invalid_topic'
  | 'invalid_priority'
  | 'control_channel'
  | 'reserved_publish'

/** Why a rule would be refused, or null when it is acceptable. */
export function aclProblem(rule: {
  type: string
  topic: string
  priority?: number
}): AclProblem | null {
  if (!isAclType(rule.type)) return 'invalid_type'
  if (!validTopicFilter(rule.topic)) return 'invalid_topic'
  const priority = rule.priority ?? 0
  if (
    !Number.isInteger(priority) ||
    priority < -maxPriority ||
    priority > maxPriority
  ) {
    return 'invalid_priority'
  }
  // The Dynamic Security channel: granting it would let a role's holders rewrite every
  // user and role.
  if (rule.topic.startsWith('$CONTROL')) return 'control_channel'
  // Nothing may publish into the broker's reserved '$' namespace.
  if (rule.type === 'publishClientSend' && rule.topic.startsWith('$')) {
    return 'reserved_publish'
  }
  return null
}

/** Messages for each problem, as translation keys. */
export const aclProblemMessage: Record<AclProblem, string> = {
  invalid_type: 'Escolha um tipo de regra.',
  invalid_topic:
    'Filtro de tópico inválido: use + e # só como nível inteiro, e # apenas no fim.',
  invalid_priority:
    'A prioridade deve ser um número inteiro entre -1000 e 1000.',
  control_channel:
    'O canal de controle do broker ($CONTROL) não pode ser concedido a uma função.',
  reserved_publish:
    'Ninguém pode publicar em tópicos que começam com $. Para ler estatísticas, use assinar e receber.',
}
