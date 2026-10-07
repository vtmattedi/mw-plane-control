import type { AclInput } from './mqtt-acl'

export type RoleTemplate = {
  id: string
  /** Translation keys (Portuguese is the source language). */
  name: string
  description: string
  acls: AclInput[]
}

const everything = (topic: string): AclInput[] => [
  { type: 'publishClientSend', topic },
  { type: 'publishClientReceive', topic },
  { type: 'subscribePattern', topic },
  { type: 'unsubscribePattern', topic },
]

/**
 * Starting points for a new role. They are ordinary rules that can be edited afterwards;
 * the broker only ever sees the rules, never the template.
 */
export const roleTemplates: RoleTemplate[] = [
  {
    id: 'empty',
    name: 'Em branco',
    description: 'Sem regras. Adicione as regras depois.',
    acls: [],
  },
  {
    id: 'trusted',
    name: 'Acesso total (confiável)',
    description:
      'Publicar, assinar e receber em todos os tópicos. Use só para gateways e serviços confiáveis.',
    acls: everything('#'),
  },
  {
    id: 'readonly',
    name: 'Somente leitura',
    description: 'Assinar e receber em todos os tópicos, sem publicar.',
    acls: [
      { type: 'subscribePattern', topic: '#' },
      { type: 'publishClientReceive', topic: '#' },
      { type: 'unsubscribePattern', topic: '#' },
    ],
  },
  {
    id: 'per-client',
    name: 'Isolado por cliente',
    description:
      'Cada cliente só acessa devices/<seu usuário>/#. Um cliente não alcança os tópicos de outro.',
    acls: everything('devices/%u/#'),
  },
  {
    id: 'monitoring',
    name: 'Monitoramento do broker',
    description:
      'Ler as estatísticas do broker ($SYS), sem acesso a mensagens.',
    acls: [
      { type: 'subscribePattern', topic: '$SYS/#' },
      { type: 'publishClientReceive', topic: '$SYS/#' },
    ],
  },
]

export function templateById(id: string): RoleTemplate | undefined {
  return roleTemplates.find((template) => template.id === id)
}
