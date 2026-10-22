import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  AlertTriangle,
  Plug,
  Radio,
  RefreshCw,
  Search,
  ShieldCheck,
  Users,
} from 'lucide-react'
import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { useConfirm } from '@/components/confirm-context'
import { MetricCard } from '@/components/control/metric-card'
import { PageHeader } from '@/components/control/page-header'
import { ResourceTable } from '@/components/control/resource-table'
import { StatusBadge } from '@/components/control/status-badge'
import {
  OneTimeCredentialDialog,
  type OneTimeCredential,
} from '@/components/one-time-credential-dialog'
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
import { Select } from '@/components/ui/select'
import { ControlApiError } from '@/lib/control-api'
import {
  fetchMqttClients,
  fetchMqttRoles,
  fetchMqttStats,
  isManageable,
  mqttUsernamePattern,
  type MqttCredential,
} from '@/lib/mqtt-api'
import { formatCount } from '@/lib/mqtt-format'
import { useMqttOperations } from '@/lib/use-mqtt-operations'
import { usePreferences } from '@/preferences/preferences-context'
import styles from './resource-page.module.scss'

export default function MqttClientsPage() {
  const { language, tr } = usePreferences()
  const locale = language === 'pt' ? 'pt-BR' : 'en-US'
  const confirm = useConfirm()
  const queryClient = useQueryClient()
  const [search, setSearch] = useState('')
  const [createOpen, setCreateOpen] = useState(false)
  const [newUsername, setNewUsername] = useState('')
  const [newRole, setNewRole] = useState('')
  const [revealed, setRevealed] = useState<MqttCredential | null>(null)
  const [rolesFor, setRolesFor] = useState<string | null>(null)

  const clientsQuery = useQuery({
    queryKey: ['mqtt', 'clients'],
    queryFn: fetchMqttClients,
    retry: 1,
  })
  const rolesQuery = useQuery({
    queryKey: ['mqtt', 'roles'],
    queryFn: fetchMqttRoles,
    retry: 1,
  })
  const statsQuery = useQuery({
    queryKey: ['mqtt', 'stats'],
    queryFn: fetchMqttStats,
    refetchInterval: 15_000,
    retry: 1,
  })

  const assignable = useMemo(
    () => rolesQuery.data?.assignable ?? [],
    [rolesQuery.data],
  )
  const clients = useMemo(() => clientsQuery.data ?? [], [clientsQuery.data])
  const assignableRoles = useMemo(
    () => (rolesQuery.data?.roles ?? []).filter((role) => role.assignable),
    [rolesQuery.data],
  )
  const editing = clients.find((client) => client.username === rolesFor)

  const { run, pending } = useMqttOperations({
    onCredential: setRevealed,
    onDone: () => {
      setCreateOpen(false)
      setNewUsername('')
    },
  })

  const rows = useMemo(() => {
    const needle = search.trim().toLocaleLowerCase('pt-BR')
    return clients
      .filter(
        (client) =>
          !needle ||
          client.username.toLocaleLowerCase('pt-BR').includes(needle) ||
          client.roles.some((role) =>
            role.toLocaleLowerCase('pt-BR').includes(needle),
          ),
      )
      .map((client) => ({
        id: client.username,
        username: client.username,
        roles: client.roles,
        enabled: !client.disabled,
      }))
  }, [clients, search])

  const stats = statsQuery.data
  const statsReady = Boolean(stats?.available) && !stats?.stale
  const metric = (value: number | undefined) =>
    statsReady ? formatCount(value, locale) : '—'

  const usernameValid = mqttUsernamePattern.test(newUsername)
  const roleToCreate = newRole || assignable[0] || ''
  const brokerDown =
    clientsQuery.error instanceof ControlApiError &&
    clientsQuery.error.code === 'broker_unavailable'

  const credentials: OneTimeCredential[] | null = revealed
    ? [
        { label: 'MQTT_USER', value: revealed.username },
        { label: 'MQTT_PASSWD', value: revealed.password },
      ]
    : null

  return (
    <main className={styles.page}>
      <div className={styles.content}>
        <PageHeader
          action={tr('Novo cliente')}
          description={tr(
            'Usuários do broker MQTT: gateways, dispositivos e serviços. A senha é gerada automaticamente e exibida uma única vez.',
          )}
          eyebrow="MQTT"
          icon={Radio}
          onAction={() => setCreateOpen(true)}
          title={tr('Clientes MQTT')}
        />

        <section
          aria-label={tr('Resumo')}
          className="mt-7 grid gap-4 sm:grid-cols-2 xl:grid-cols-4"
        >
          <MetricCard
            icon={Users}
            label={tr('Clientes')}
            note={tr('Cadastrados no broker')}
            tone="accent"
            value={clientsQuery.isLoading ? '…' : clients.length}
          />
          <MetricCard
            icon={Plug}
            label={tr('Conectados')}
            note={tr('Sessões abertas agora')}
            value={metric(stats?.clients.connected)}
          />
          <MetricCard
            icon={Radio}
            label={tr('Assinaturas')}
            note={tr('Ativas no broker')}
            value={metric(stats?.store.subscriptions)}
          />
          <MetricCard
            icon={ShieldCheck}
            label={tr('Mensagens retidas')}
            note={tr('Estado guardado pelo broker')}
            value={metric(stats?.store.retainedMessages)}
          />
        </section>

        <Card className="border-border/70 mt-6 overflow-hidden shadow-sm">
          <div className="border-border/70 flex flex-col gap-4 border-b p-4 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h2 className="text-sm font-semibold">{tr('Clientes MQTT')}</h2>
              <p className="text-muted-foreground mt-0.5 text-xs">
                {tr('Gerencie e acompanhe este recurso pelo control plane.')}
              </p>
            </div>
            <div className={styles.toolbar}>
              <label className={styles.search}>
                <span className="sr-only">{tr('Buscar')}</span>
                <Search aria-hidden="true" />
                <input
                  id="mqtt-search"
                  onChange={(event) => setSearch(event.target.value)}
                  placeholder={tr('Buscar registros…')}
                  type="search"
                  value={search}
                />
              </label>
              <Button
                aria-label={tr('Atualizar')}
                disabled={clientsQuery.isFetching}
                onClick={() =>
                  void queryClient.invalidateQueries({ queryKey: ['mqtt'] })
                }
                size="icon"
                variant="outline"
              >
                <RefreshCw
                  className={clientsQuery.isFetching ? 'animate-spin' : ''}
                />
              </Button>
            </div>
          </div>

          {clientsQuery.isError && (
            <div className={styles.error} role="status">
              <AlertTriangle className="text-warning mt-0.5 size-4 shrink-0" />
              <span>
                {brokerDown
                  ? tr('O broker MQTT não está acessível no momento.')
                  : clientsQuery.error instanceof Error
                    ? tr(clientsQuery.error.message)
                    : tr('Não foi possível carregar os clientes.')}
              </span>
            </div>
          )}

          <ResourceTable
            columns={[
              { key: 'username', label: tr('Cliente') },
              { key: 'roles', label: tr('Funções') },
              { key: 'enabled', label: tr('Status') },
            ]}
            emptyDescription={tr(
              'Crie um cliente para um gateway, dispositivo ou serviço.',
            )}
            emptyTitle={
              clientsQuery.isLoading
                ? tr('Carregando dados…')
                : tr('Nenhum cliente MQTT')
            }
            renderActions={(row) => {
              const client = clients.find((c) => c.username === row.id)
              if (!client) return null
              if (!isManageable(client, assignable)) {
                return (
                  <div className="flex justify-end">
                    <StatusBadge tone="info">{tr('Protegido')}</StatusBadge>
                  </div>
                )
              }
              const username = client.username
              return (
                <div className="flex flex-wrap justify-end gap-2">
                  <Button
                    disabled={pending}
                    onClick={() => setRolesFor(username)}
                    size="sm"
                    variant="outline"
                  >
                    {tr('Funções')}
                  </Button>
                  <Button
                    disabled={pending}
                    onClick={async () => {
                      const enable = client.disabled
                      const accepted = await confirm({
                        title: tr(
                          enable ? 'Ativar cliente?' : 'Desativar cliente?',
                        ),
                        description: tr(
                          enable
                            ? 'O cliente poderá se conectar novamente com a senha atual.'
                            : 'O cliente será desconectado e não poderá se conectar até ser reativado.',
                        ),
                        confirmLabel: tr(enable ? 'Ativar' : 'Desativar'),
                      })
                      if (accepted)
                        run({ kind: enable ? 'enable' : 'disable', username })
                    }}
                    size="sm"
                    variant="ghost"
                  >
                    {tr(client.disabled ? 'Ativar' : 'Desativar')}
                  </Button>
                  <Button
                    disabled={pending}
                    onClick={async () => {
                      const accepted = await confirm({
                        title: tr('Gerar nova senha?'),
                        description: tr(
                          'A senha atual deixará de funcionar e a nova será exibida apenas uma vez.',
                        ),
                        confirmLabel: tr('Gerar nova senha'),
                      })
                      if (accepted) run({ kind: 'rotate', username })
                    }}
                    size="sm"
                    variant="outline"
                  >
                    {tr('Nova senha')}
                  </Button>
                  <Button
                    disabled={pending}
                    onClick={async () => {
                      const accepted = await confirm({
                        title: tr('Excluir cliente?'),
                        description: tr(
                          'O cliente será desconectado e removido do broker. Esta ação não pode ser desfeita.',
                        ),
                        confirmLabel: tr('Excluir'),
                      })
                      if (accepted) run({ kind: 'delete', username })
                    }}
                    size="sm"
                    variant="destructive"
                  >
                    {tr('Excluir')}
                  </Button>
                </div>
              )
            }}
            rows={rows}
          />
        </Card>

        <aside className="text-muted-foreground mt-5 flex items-start gap-2 text-xs leading-5">
          <ShieldCheck className="text-primary mt-0.5 size-4 shrink-0" />
          {tr(
            'Criar, reativar, trocar senha, excluir e alterar funções exigem autenticação forte recente. Desativar é imediato. Clientes protegidos do broker não podem ser alterados aqui.',
          )}
        </aside>

        <Dialog onOpenChange={setCreateOpen} open={createOpen}>
          <DialogContent closeDisabled={pending}>
            <DialogHeader>
              <DialogTitle>{tr('Novo cliente')}</DialogTitle>
              <DialogDescription>
                {tr(
                  'Informe o nome do cliente. A senha será gerada e exibida uma única vez.',
                )}
              </DialogDescription>
            </DialogHeader>
            <div className="grid gap-4">
              <label className="grid gap-2 text-sm font-medium">
                {tr('Nome do cliente')}
                <Input
                  autoFocus
                  disabled={pending}
                  maxLength={128}
                  onChange={setNewUsername}
                  placeholder="nmnw-gateway-aabbccddeeff"
                  value={newUsername}
                />
                {newUsername !== '' && !usernameValid && (
                  <span className="text-destructive text-xs font-normal">
                    {tr(
                      'Use letras, números, ponto, hífen ou sublinhado (até 128 caracteres).',
                    )}
                  </span>
                )}
              </label>
              {assignable.length > 1 && (
                <label className="grid gap-2 text-sm font-medium">
                  {tr('Função')}
                  <Select
                    onChange={(event) => setNewRole(event.target.value)}
                    value={roleToCreate}
                  >
                    {assignable.map((role) => (
                      <option key={role} value={role}>
                        {role}
                      </option>
                    ))}
                  </Select>
                </label>
              )}
              {assignable.length === 1 && (
                <p className="text-muted-foreground text-xs">
                  {tr('Função atribuída:')} <strong>{assignable[0]}</strong>
                </p>
              )}
            </div>
            <DialogFooter>
              <Button
                disabled={pending}
                onClick={() => setCreateOpen(false)}
                variant="outline"
              >
                {tr('Cancelar')}
              </Button>
              <Button
                disabled={pending || !usernameValid || assignable.length === 0}
                onClick={() =>
                  run({
                    kind: 'create',
                    username: newUsername,
                    role: roleToCreate || undefined,
                  })
                }
              >
                {tr('Criar cliente')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        <Dialog
          onOpenChange={(open) => {
            if (!open) setRolesFor(null)
          }}
          open={editing !== undefined}
        >
          <DialogContent className="sm:max-w-xl" closeDisabled={pending}>
            <DialogHeader>
              <DialogTitle>
                {tr('Funções do cliente')}
                {editing ? ` · ${editing.username}` : ''}
              </DialogTitle>
              <DialogDescription>
                {tr(
                  'As funções definem o que o cliente pode publicar e assinar. Alterar exige autenticação forte recente.',
                )}
              </DialogDescription>
            </DialogHeader>
            {assignableRoles.length === 0 ? (
              <p className="text-muted-foreground text-sm">
                {tr('Ainda não há funções que possam ser atribuídas.')}{' '}
                <Link className="text-primary underline" to="/mqtt/roles">
                  {tr('Criar uma função')}
                </Link>
              </p>
            ) : (
              <ul className="border-border/70 divide-border/70 divide-y rounded-xl border">
                {assignableRoles.map((role) => {
                  const held = editing?.roles.includes(role.name) ?? false
                  return (
                    <li
                      className="flex items-center justify-between gap-3 p-3"
                      key={role.name}
                    >
                      <div className="min-w-0">
                        <p className="text-sm font-medium">{role.name}</p>
                        <p className="text-muted-foreground truncate text-xs">
                          {role.description ||
                            `${role.acls.length} ${tr('regras')}`}
                        </p>
                      </div>
                      <Button
                        disabled={pending}
                        onClick={async () => {
                          if (!editing) return
                          if (!held) {
                            run({
                              kind: 'assignRole',
                              username: editing.username,
                              role: role.name,
                            })
                            return
                          }
                          const accepted = await confirm({
                            title: tr('Remover função?'),
                            description: tr(
                              'O cliente perde imediatamente o acesso que esta função concede.',
                            ),
                            confirmLabel: tr('Remover'),
                          })
                          if (accepted) {
                            run({
                              kind: 'unassignRole',
                              username: editing.username,
                              role: role.name,
                            })
                          }
                        }}
                        size="sm"
                        variant={held ? 'outline' : 'default'}
                      >
                        {tr(held ? 'Remover' : 'Adicionar')}
                      </Button>
                    </li>
                  )
                })}
              </ul>
            )}
            <DialogFooter>
              <Button onClick={() => setRolesFor(null)}>
                {tr('Concluir')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        <OneTimeCredentialDialog
          credentials={credentials}
          onClose={() => setRevealed(null)}
          title={tr('Credenciais MQTT')}
        />
      </div>
    </main>
  )
}
