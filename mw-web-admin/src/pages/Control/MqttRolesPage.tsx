import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  AlertTriangle,
  KeyRound,
  Lock,
  RefreshCw,
  ShieldCheck,
  Users,
} from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useConfirm } from '@/components/confirm-context'
import { MetricCard } from '@/components/control/metric-card'
import { PageHeader } from '@/components/control/page-header'
import { ResourceTable } from '@/components/control/resource-table'
import { StatusBadge } from '@/components/control/status-badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { ControlApiError } from '@/lib/control-api'
import {
  aclProblem,
  aclProblemMessage,
  aclTypeHelp,
  aclTypes,
  mqttRolePattern,
  type AclType,
} from '@/lib/mqtt-acl'
import { fetchMqttRole, fetchMqttRoles, type MqttRole } from '@/lib/mqtt-api'
import { roleTemplates, templateById } from '@/lib/mqtt-templates'
import { useMqttOperations } from '@/lib/use-mqtt-operations'
import { usePreferences } from '@/preferences/preferences-context'
import styles from './resource-page.module.scss'

const fieldClass =
  'border-input bg-background focus-visible:ring-ring h-11 w-full rounded-xl border px-3 text-sm focus-visible:outline-none focus-visible:ring-2'

/** "{n}" placeholders in translated sentences. */
function withCount(text: string, count: number) {
  return text.replace('{n}', String(count))
}

export default function MqttRolesPage() {
  const { tr } = usePreferences()
  const confirm = useConfirm()
  const queryClient = useQueryClient()
  const [createOpen, setCreateOpen] = useState(false)
  const [detail, setDetail] = useState<string | null>(null)

  const rolesQuery = useQuery({
    queryKey: ['mqtt', 'roles'],
    queryFn: fetchMqttRoles,
    retry: 1,
  })
  const { run, pending } = useMqttOperations({
    onDone: () => setCreateOpen(false),
  })

  const roles = useMemo(() => rolesQuery.data?.roles ?? [], [rolesQuery.data])
  const reserved = roles.filter((role) => role.reserved).length
  const brokerDown =
    rolesQuery.error instanceof ControlApiError &&
    rolesQuery.error.code === 'broker_unavailable'

  const rows = roles.map((role) => ({
    id: role.name,
    name: role.name,
    description: role.description,
    rules: role.acls.length,
    clients: role.clientCount,
    type: role.reserved
      ? tr('Reservada')
      : role.assignable
        ? tr('Atribuível')
        : tr('Restrita'),
  }))

  async function removeRole(role: MqttRole) {
    const inUse = role.clientCount > 0
    const accepted = await confirm({
      title: tr('Excluir função?'),
      description: inUse
        ? withCount(
            tr(
              role.clientCount === 1
                ? 'Esta função está em uso por {n} cliente. Ele perderá imediatamente o acesso que ela concede.'
                : 'Esta função está em uso por {n} clientes. Eles perderão imediatamente o acesso que ela concede.',
            ),
            role.clientCount,
          )
        : tr(
            'Nenhum cliente usa esta função. Esta ação não pode ser desfeita.',
          ),
      confirmLabel: tr('Excluir'),
    })
    if (accepted) run({ kind: 'deleteRole', name: role.name, force: inUse })
  }

  return (
    <main className={styles.page}>
      <div className={styles.content}>
        <PageHeader
          action={tr('Nova função')}
          description={tr(
            'Funções definem o que cada cliente pode publicar e assinar. Crie, edite e atribua funções aqui; as funções do próprio broker são reservadas.',
          )}
          eyebrow="MQTT"
          icon={KeyRound}
          onAction={() => setCreateOpen(true)}
          title={tr('Funções de acesso')}
        />

        <section
          aria-label={tr('Resumo')}
          className="mt-7 grid gap-4 sm:grid-cols-3"
        >
          <MetricCard
            icon={KeyRound}
            label={tr('Funções')}
            note={tr('Cadastradas no broker')}
            tone="accent"
            value={rolesQuery.isLoading ? '…' : roles.length}
          />
          <MetricCard
            icon={Users}
            label={tr('Atribuíveis')}
            note={tr('Podem ser dadas a clientes')}
            value={
              rolesQuery.isLoading
                ? '…'
                : (rolesQuery.data?.assignable.length ?? 0)
            }
          />
          <MetricCard
            icon={Lock}
            label={tr('Reservadas')}
            note={tr('Administração do broker')}
            value={rolesQuery.isLoading ? '…' : reserved}
          />
        </section>

        <Card className="border-border/70 mt-6 overflow-hidden shadow-sm">
          <div className="border-border/70 flex items-center justify-between gap-4 border-b p-4">
            <div>
              <h2 className="text-sm font-semibold">
                {tr('Funções de acesso')}
              </h2>
              <p className="text-muted-foreground mt-0.5 text-xs">
                {tr('Gerencie e acompanhe este recurso pelo control plane.')}
              </p>
            </div>
            <Button
              aria-label={tr('Atualizar')}
              disabled={rolesQuery.isFetching}
              onClick={() =>
                void queryClient.invalidateQueries({ queryKey: ['mqtt'] })
              }
              size="icon"
              variant="outline"
            >
              <RefreshCw
                className={rolesQuery.isFetching ? 'animate-spin' : ''}
              />
            </Button>
          </div>

          {rolesQuery.isError && (
            <div className={styles.error} role="status">
              <AlertTriangle className="text-warning mt-0.5 size-4 shrink-0" />
              <span>
                {brokerDown
                  ? tr('O broker MQTT não está acessível no momento.')
                  : rolesQuery.error instanceof Error
                    ? tr(rolesQuery.error.message)
                    : tr('Não foi possível carregar as funções.')}
              </span>
            </div>
          )}

          <ResourceTable
            columns={[
              { key: 'name', label: tr('Função') },
              { key: 'description', label: tr('Descrição') },
              { key: 'rules', label: tr('Regras') },
              { key: 'clients', label: tr('Clientes') },
              { key: 'type', label: tr('Tipo') },
            ]}
            emptyDescription={tr(
              'Crie uma função para definir o que um cliente pode fazer.',
            )}
            emptyTitle={
              rolesQuery.isLoading
                ? tr('Carregando dados…')
                : tr('Nenhuma função')
            }
            renderActions={(row) => {
              const role = roles.find((r) => r.name === row.id)
              if (!role) return null
              return (
                <div className="flex flex-wrap justify-end gap-2">
                  <Button
                    onClick={() => setDetail(role.name)}
                    size="sm"
                    variant="outline"
                  >
                    {tr('Detalhes')}
                  </Button>
                  {!role.reserved && (
                    <Button
                      disabled={pending}
                      onClick={() => void removeRole(role)}
                      size="sm"
                      variant="destructive"
                    >
                      {tr('Excluir')}
                    </Button>
                  )}
                </div>
              )
            }}
            rows={rows}
          />
        </Card>

        <aside className="text-muted-foreground mt-5 flex items-start gap-2 text-xs leading-5">
          <ShieldCheck className="text-primary mt-0.5 size-4 shrink-0" />
          {tr(
            'Criar, editar e excluir funções, e dá-las a clientes, exigem autenticação forte recente. Nenhuma regra pode conceder o canal de controle do broker ($CONTROL).',
          )}
        </aside>

        <CreateRoleDialog
          onClose={() => setCreateOpen(false)}
          open={createOpen}
          pending={pending}
          run={run}
          takenNames={roles.map((role) => role.name)}
        />
        <RoleDetailDialog
          name={detail}
          onClose={() => setDetail(null)}
          pending={pending}
          run={run}
        />
      </div>
    </main>
  )
}

type Run = ReturnType<typeof useMqttOperations>['run']

function CreateRoleDialog({
  open,
  onClose,
  run,
  pending,
  takenNames,
}: {
  open: boolean
  onClose: () => void
  run: Run
  pending: boolean
  takenNames: string[]
}) {
  const { tr } = usePreferences()
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [templateId, setTemplateId] = useState('empty')

  // A fresh form each time the dialog opens.
  useEffect(() => {
    if (open) {
      setName('')
      setDescription('')
      setTemplateId('empty')
    }
  }, [open])

  const template = templateById(templateId) ?? roleTemplates[0]
  const nameValid = mqttRolePattern.test(name)
  const taken = takenNames.includes(name)

  return (
    <Dialog onOpenChange={(next) => !next && onClose()} open={open}>
      <DialogContent
        className="max-h-[90dvh] overflow-y-auto sm:max-w-2xl"
        closeDisabled={pending}
      >
        <DialogHeader>
          <DialogTitle>{tr('Nova função')}</DialogTitle>
          <DialogDescription>
            {tr(
              'Escolha um ponto de partida. As regras podem ser editadas depois.',
            )}
          </DialogDescription>
        </DialogHeader>

        <div className="grid gap-4">
          <label className="grid gap-2 text-sm font-medium">
            {tr('Nome da função')}
            <Input
              autoFocus
              disabled={pending}
              maxLength={64}
              onChange={setName}
              placeholder="data-viewer"
              value={name}
            />
            {name !== '' && !nameValid && (
              <span className="text-destructive text-xs font-normal">
                {tr(
                  'Use letras, números, ponto, hífen ou sublinhado (até 64 caracteres).',
                )}
              </span>
            )}
            {nameValid && taken && (
              <span className="text-destructive text-xs font-normal">
                {tr('Já existe uma função com este nome.')}
              </span>
            )}
          </label>
          <label className="grid gap-2 text-sm font-medium">
            {tr('Descrição (opcional)')}
            <Input
              disabled={pending}
              maxLength={200}
              onChange={setDescription}
              value={description}
            />
          </label>

          <fieldset className="grid gap-2">
            <legend className="mb-1 text-sm font-medium">
              {tr('Ponto de partida')}
            </legend>
            <div className="grid gap-2 sm:grid-cols-2">
              {roleTemplates.map((option) => (
                <button
                  aria-pressed={option.id === templateId}
                  className={`rounded-xl border p-3 text-left text-sm transition-colors ${
                    option.id === templateId
                      ? 'border-primary bg-primary/5'
                      : 'border-border/70 hover:bg-accent'
                  }`}
                  disabled={pending}
                  key={option.id}
                  onClick={() => setTemplateId(option.id)}
                  type="button"
                >
                  <span className="block font-medium">{tr(option.name)}</span>
                  <span className="text-muted-foreground mt-1 block text-xs leading-5">
                    {tr(option.description)}
                  </span>
                </button>
              ))}
            </div>
          </fieldset>

          <div>
            <p className="mb-2 text-sm font-medium">
              {tr('Regras iniciais')} ({template.acls.length})
            </p>
            {template.acls.length === 0 ? (
              <p className="text-muted-foreground text-xs">
                {tr(
                  'Sem regras: o cliente não poderá fazer nada até você adicionar.',
                )}
              </p>
            ) : (
              <ul className="text-muted-foreground space-y-1 text-xs">
                {template.acls.map((acl) => (
                  <li
                    className="flex justify-between gap-3"
                    key={`${acl.type}:${acl.topic}`}
                  >
                    <span>{acl.type}</span>
                    <code className="text-foreground break-all">
                      {acl.topic}
                    </code>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </div>

        <DialogFooter>
          <Button disabled={pending} onClick={onClose} variant="outline">
            {tr('Cancelar')}
          </Button>
          <Button
            disabled={pending || !nameValid || taken}
            onClick={() =>
              run({
                kind: 'createRole',
                name,
                ...(description.trim()
                  ? { description: description.trim() }
                  : {}),
                acls: template.acls,
              })
            }
          >
            {tr('Criar função')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function RoleDetailDialog({
  name,
  onClose,
  run,
  pending,
}: {
  name: string | null
  onClose: () => void
  run: Run
  pending: boolean
}) {
  const { tr } = usePreferences()
  const confirm = useConfirm()
  const [type, setType] = useState<AclType>('subscribePattern')
  const [topic, setTopic] = useState('')
  const [mode, setMode] = useState<'allow' | 'deny'>('allow')
  const [priority, setPriority] = useState('0')

  const query = useQuery({
    queryKey: ['mqtt', 'role', name],
    queryFn: () => fetchMqttRole(name as string),
    enabled: name !== null,
    retry: 1,
  })
  const role = query.data

  // Clear the topic once a rule has been added.
  const ruleCount = role?.acls.length
  useEffect(() => {
    setTopic('')
  }, [ruleCount, name])

  const editable = Boolean(role && !role.reserved)
  const problem = topic.trim()
    ? aclProblem({ type, topic: topic.trim(), priority: Number(priority) })
    : null

  async function removeRule(acl: { type: string; topic: string }) {
    if (!role) return
    const accepted = await confirm({
      title: tr('Remover regra?'),
      description:
        role.clientCount > 0
          ? withCount(
              tr(
                role.clientCount === 1
                  ? '{n} cliente usa esta função e perde esse acesso imediatamente.'
                  : '{n} clientes usam esta função e perdem esse acesso imediatamente.',
              ),
              role.clientCount,
            )
          : tr('Nenhum cliente usa esta função.'),
      confirmLabel: tr('Remover'),
    })
    if (accepted && aclTypes.includes(acl.type as AclType)) {
      run({
        kind: 'removeAcl',
        role: role.name,
        type: acl.type as AclType,
        topic: acl.topic,
      })
    }
  }

  return (
    <Dialog onOpenChange={(next) => !next && onClose()} open={name !== null}>
      <DialogContent
        className="max-h-[90dvh] overflow-y-auto sm:max-w-3xl"
        closeDisabled={pending}
      >
        <DialogHeader>
          <DialogTitle>
            {tr('Função')} · {name}
          </DialogTitle>
          <DialogDescription>
            {role?.description || tr('Regras que esta função concede.')}
          </DialogDescription>
        </DialogHeader>

        {query.isLoading && (
          <p className="text-muted-foreground text-sm">
            {tr('Carregando dados…')}
          </p>
        )}
        {query.isError && (
          <p className="text-destructive text-sm">
            {query.error instanceof Error
              ? tr(query.error.message)
              : tr('Não foi possível carregar as funções.')}
          </p>
        )}

        {role && (
          <div className="grid gap-5">
            <div className="flex flex-wrap items-center gap-2">
              {role.reserved ? (
                <StatusBadge tone="info">{tr('Reservada')}</StatusBadge>
              ) : role.assignable ? (
                <StatusBadge tone="positive">{tr('Atribuível')}</StatusBadge>
              ) : (
                <StatusBadge tone="neutral">{tr('Restrita')}</StatusBadge>
              )}
              {role.reserved && (
                <span className="text-muted-foreground text-xs">
                  {tr('Função do broker: não pode ser alterada aqui.')}
                </span>
              )}
            </div>

            <section>
              <h3 className="mb-2 text-sm font-semibold">
                {tr('Clientes')} ({role.clientCount})
              </h3>
              {role.clients.length === 0 ? (
                <p className="text-muted-foreground text-xs">
                  {tr('Nenhum cliente usa esta função.')}
                </p>
              ) : (
                <ul className="flex flex-wrap gap-2">
                  {role.clients.map((client) => (
                    <li
                      className="border-border/70 rounded-full border px-2.5 py-1 text-xs"
                      key={client}
                    >
                      {client}
                    </li>
                  ))}
                </ul>
              )}
            </section>

            <section>
              <h3 className="mb-2 text-sm font-semibold">
                {tr('Regras')} ({role.acls.length})
              </h3>
              {role.acls.length === 0 ? (
                <p className="text-muted-foreground text-xs">
                  {tr('Sem regras: nenhum acesso é concedido.')}
                </p>
              ) : (
                <div className="border-border/70 overflow-x-auto rounded-xl border">
                  <table>
                    <thead>
                      <tr>
                        <th>{tr('Tipo')}</th>
                        <th>{tr('Tópico')}</th>
                        <th>{tr('Efeito')}</th>
                        <th>{tr('Prioridade')}</th>
                        <th>
                          <span className="sr-only">{tr('Ações')}</span>
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {role.acls.map((acl) => (
                        <tr key={`${acl.type}:${acl.topic}`}>
                          <td>
                            <span className="font-medium">{acl.type}</span>
                            <span className="text-muted-foreground block text-xs">
                              {tr(aclTypeHelp[acl.type as AclType] ?? '')}
                            </span>
                          </td>
                          <td>
                            <code className="break-all">{acl.topic}</code>
                          </td>
                          <td>
                            <StatusBadge
                              tone={acl.allow ? 'positive' : 'danger'}
                            >
                              {tr(acl.allow ? 'Permitir' : 'Negar')}
                            </StatusBadge>
                          </td>
                          <td>{acl.priority}</td>
                          <td className="text-right">
                            {editable && (
                              <Button
                                aria-label={`${tr('Remover regra')} ${acl.type} ${acl.topic}`}
                                disabled={pending}
                                onClick={() => void removeRule(acl)}
                                size="sm"
                                variant="outline"
                              >
                                {tr('Remover')}
                              </Button>
                            )}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </section>

            {editable && (
              <section className="border-border/70 rounded-xl border p-4">
                <h3 className="mb-3 text-sm font-semibold">
                  {tr('Adicionar regra')}
                </h3>
                <div className="grid gap-3 sm:grid-cols-2">
                  <label className="grid gap-1.5 text-xs font-medium">
                    {tr('Tipo')}
                    <select
                      className={fieldClass}
                      disabled={pending}
                      onChange={(event) =>
                        setType(event.target.value as AclType)
                      }
                      value={type}
                    >
                      {aclTypes.map((option) => (
                        <option key={option} value={option}>
                          {option}
                        </option>
                      ))}
                    </select>
                    <span className="text-muted-foreground font-normal">
                      {tr(aclTypeHelp[type])}
                    </span>
                  </label>
                  <label className="grid gap-1.5 text-xs font-medium">
                    {tr('Efeito')}
                    <select
                      className={fieldClass}
                      disabled={pending}
                      onChange={(event) =>
                        setMode(event.target.value as 'allow' | 'deny')
                      }
                      value={mode}
                    >
                      <option value="allow">{tr('Permitir')}</option>
                      <option value="deny">{tr('Negar')}</option>
                    </select>
                  </label>
                  <label className="grid gap-1.5 text-xs font-medium sm:col-span-2">
                    {tr('Filtro de tópico')}
                    <Input
                      disabled={pending}
                      maxLength={1024}
                      onChange={setTopic}
                      placeholder="devices/%u/#"
                      value={topic}
                    />
                    <span className="text-muted-foreground font-normal">
                      {tr(
                        '+ e # são curingas; %u e %c valem o usuário e o client ID do próprio cliente.',
                      )}
                    </span>
                  </label>
                  <label className="grid gap-1.5 text-xs font-medium">
                    {tr('Prioridade')}
                    <Input
                      disabled={pending}
                      onChange={setPriority}
                      type="number"
                      value={priority}
                    />
                  </label>
                </div>
                {problem && (
                  <p className="text-destructive mt-3 text-xs" role="alert">
                    {tr(aclProblemMessage[problem])}
                  </p>
                )}
                <div className="mt-4 flex justify-end">
                  <Button
                    disabled={
                      pending || topic.trim() === '' || problem !== null
                    }
                    onClick={() =>
                      run({
                        kind: 'addAcl',
                        role: role.name,
                        acl: {
                          type,
                          topic: topic.trim(),
                          allow: mode === 'allow',
                          priority: Number(priority) || 0,
                        },
                      })
                    }
                  >
                    {tr('Adicionar regra')}
                  </Button>
                </div>
              </section>
            )}
          </div>
        )}

        <DialogFooter>
          <Button onClick={onClose}>{tr('Concluir')}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
