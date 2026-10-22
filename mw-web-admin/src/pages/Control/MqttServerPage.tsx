import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Activity,
  AlertTriangle,
  Clock,
  Plug,
  RefreshCw,
  ServerCog,
  ShieldCheck,
} from 'lucide-react'
import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { MetricCard } from '@/components/control/metric-card'
import { PageHeader } from '@/components/control/page-header'
import { StatusBadge } from '@/components/control/status-badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import {
  fetchMqttRoles,
  fetchMqttServer,
  fetchMqttStats,
  type MqttStatsGroup,
} from '@/lib/mqtt-api'
import {
  formatBytes,
  formatCount,
  formatDuration,
  formatRate,
  type Locale,
} from '@/lib/mqtt-format'
import { usePreferences } from '@/preferences/preferences-context'
import styles from './resource-page.module.scss'

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <Card className="border-border/70 overflow-hidden shadow-sm">
      <div className="border-border/70 border-b p-4">
        <h2 className="text-sm font-semibold">{title}</h2>
      </div>
      <div className="overflow-x-auto">{children}</div>
    </Card>
  )
}

function KeyValueTable({ rows }: { rows: { label: string; value: string }[] }) {
  return (
    <table>
      <tbody>
        {rows.map((row) => (
          <tr key={row.label}>
            <td className="text-muted-foreground">{row.label}</td>
            <td className="text-right font-medium tabular-nums">{row.value}</td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}

const loadRows: { label: string; key: string; bytes?: boolean }[] = [
  { label: 'Mensagens recebidas', key: 'messagesReceived' },
  { label: 'Mensagens enviadas', key: 'messagesSent' },
  { label: 'Publicações recebidas', key: 'publishReceived' },
  { label: 'Publicações enviadas', key: 'publishSent' },
  { label: 'Bytes recebidos', key: 'bytesReceived', bytes: true },
  { label: 'Bytes enviados', key: 'bytesSent', bytes: true },
  { label: 'Conexões', key: 'connections' },
]

function LoadTable({
  load,
  locale,
  perMinute,
  tr,
}: {
  load: MqttStatsGroup
  locale: Locale
  perMinute: string
  tr: (text: string) => string
}) {
  const windows = ['1m', '5m', '15m']
  return (
    <table>
      <thead>
        <tr>
          <th>{perMinute}</th>
          {windows.map((window) => (
            <th className="text-right" key={window}>
              {window}
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {loadRows.map((row) => (
          <tr key={row.key}>
            <td className="text-muted-foreground">{tr(row.label)}</td>
            {windows.map((window) => {
              const value = load[`${row.key}${window}`]
              return (
                <td
                  className="text-right font-medium tabular-nums"
                  key={window}
                >
                  {row.bytes
                    ? formatBytes(value, locale)
                    : formatRate(value, locale)}
                </td>
              )
            })}
          </tr>
        ))}
      </tbody>
    </table>
  )
}

export default function MqttServerPage() {
  const { language, tr } = usePreferences()
  const locale: Locale = language === 'pt' ? 'pt-BR' : 'en-US'
  const queryClient = useQueryClient()

  const statsQuery = useQuery({
    queryKey: ['mqtt', 'stats'],
    queryFn: fetchMqttStats,
    refetchInterval: 10_000,
    retry: 1,
  })
  const serverQuery = useQuery({
    queryKey: ['mqtt', 'server'],
    queryFn: fetchMqttServer,
    refetchInterval: 15_000,
    retry: 1,
  })
  const rolesQuery = useQuery({
    queryKey: ['mqtt', 'roles'],
    queryFn: fetchMqttRoles,
    retry: 1,
  })

  const stats = statsQuery.data
  const server = serverQuery.data
  const hasStats = Boolean(stats?.available)
  const brokerUp = server?.brokerConnected === true && server.dynsecReachable
  const updated = stats?.updatedAt
    ? new Intl.DateTimeFormat(locale, { timeStyle: 'medium' }).format(
        new Date(stats.updatedAt),
      )
    : '—'

  return (
    <main className={styles.page}>
      <div className={styles.content}>
        <PageHeader
          description={tr(
            'Estado do broker MQTT: conexões, tráfego e funções de acesso. Os números vêm do próprio broker e são atualizados a cada 10 segundos.',
          )}
          eyebrow="MQTT"
          icon={ServerCog}
          title={tr('Broker MQTT')}
        />

        <section
          aria-label={tr('Resumo')}
          className="mt-7 grid gap-4 sm:grid-cols-2 xl:grid-cols-4"
        >
          <MetricCard
            icon={Plug}
            label={tr('Broker')}
            note={tr('Conexão do serviço de administração')}
            tone="accent"
            value={
              serverQuery.isLoading
                ? '…'
                : brokerUp
                  ? tr('Conectado')
                  : tr('Indisponível')
            }
          />
          <MetricCard
            icon={ShieldCheck}
            label={tr('Versão')}
            note="Mosquitto"
            value={stats?.version ?? '—'}
          />
          <MetricCard
            icon={Clock}
            label={tr('Tempo ativo')}
            note={tr('Desde a última reinicialização')}
            value={formatDuration(stats?.uptimeSeconds)}
          />
          <MetricCard
            icon={Activity}
            label={tr('Atualizado')}
            note={tr(
              stats?.stale ? 'Dados desatualizados' : 'Leitura mais recente',
            )}
            value={updated}
          />
        </section>

        <div className="mt-6 flex items-center justify-between gap-3">
          <div>
            {stats?.stale && (
              <StatusBadge tone="warning">
                {tr('Sem atualização recente do broker')}
              </StatusBadge>
            )}
          </div>
          <Button
            aria-label={tr('Atualizar')}
            disabled={statsQuery.isFetching}
            onClick={() =>
              void queryClient.invalidateQueries({ queryKey: ['mqtt'] })
            }
            size="icon"
            variant="outline"
          >
            <RefreshCw
              className={statsQuery.isFetching ? 'animate-spin' : ''}
            />
          </Button>
        </div>

        {(statsQuery.isError || (!statsQuery.isLoading && !hasStats)) && (
          <div className={styles.error} role="status">
            <AlertTriangle className="text-warning mt-0.5 size-4 shrink-0" />
            <span>
              {statsQuery.isError
                ? statsQuery.error instanceof Error
                  ? tr(statsQuery.error.message)
                  : tr('Não foi possível carregar as estatísticas.')
                : tr(
                    'O broker ainda não publicou estatísticas. Elas aparecem em até 10 segundos após a conexão.',
                  )}
            </span>
          </div>
        )}

        {hasStats && stats && (
          <div className="mt-4 grid gap-6 lg:grid-cols-2">
            <Section title={tr('Conexões e armazenamento')}>
              <KeyValueTable
                rows={[
                  {
                    label: tr('Clientes conectados'),
                    value: formatCount(stats.clients.connected, locale),
                  },
                  {
                    label: tr('Clientes (total)'),
                    value: formatCount(stats.clients.total, locale),
                  },
                  {
                    label: tr('Máximo de clientes'),
                    value: formatCount(stats.clients.maximum, locale),
                  },
                  {
                    label: tr('Assinaturas'),
                    value: formatCount(stats.store.subscriptions, locale),
                  },
                  {
                    label: tr('Mensagens retidas'),
                    value: formatCount(stats.store.retainedMessages, locale),
                  },
                  {
                    label: tr('Mensagens armazenadas'),
                    value: formatCount(stats.store.storedMessages, locale),
                  },
                  {
                    label: tr('Memória em uso'),
                    value: formatBytes(stats.memory.heapCurrent, locale),
                  },
                  {
                    label: tr('Pico de memória'),
                    value: formatBytes(stats.memory.heapMaximum, locale),
                  },
                ]}
              />
            </Section>

            <Section title={tr('Tráfego acumulado')}>
              <KeyValueTable
                rows={[
                  {
                    label: tr('Mensagens recebidas'),
                    value: formatCount(stats.messages.received, locale),
                  },
                  {
                    label: tr('Mensagens enviadas'),
                    value: formatCount(stats.messages.sent, locale),
                  },
                  {
                    label: tr('Publicações recebidas'),
                    value: formatCount(stats.messages.publishReceived, locale),
                  },
                  {
                    label: tr('Publicações enviadas'),
                    value: formatCount(stats.messages.publishSent, locale),
                  },
                  {
                    label: tr('Publicações descartadas'),
                    value: formatCount(stats.messages.publishDropped, locale),
                  },
                  {
                    label: tr('Bytes recebidos'),
                    value: formatBytes(stats.bytes.received, locale),
                  },
                  {
                    label: tr('Bytes enviados'),
                    value: formatBytes(stats.bytes.sent, locale),
                  },
                ]}
              />
            </Section>

            <div className="lg:col-span-2">
              <Section title={tr('Carga média')}>
                <LoadTable
                  load={stats.load}
                  locale={locale}
                  perMinute={tr('Por minuto')}
                  tr={tr}
                />
              </Section>
            </div>
          </div>
        )}

        <div className="mt-6">
          <Section title={tr('Funções de acesso')}>
            <div className="flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:justify-between">
              <p className="text-muted-foreground text-sm">
                {rolesQuery.isError
                  ? tr('Não foi possível carregar as funções.')
                  : `${rolesQuery.data?.roles.length ?? '…'} ${tr('funções')} · ${rolesQuery.data?.assignable.length ?? '…'} ${tr('atribuíveis')}`}
              </p>
              <Button asChild variant="outline">
                <Link to="/mqtt/roles">{tr('Gerenciar funções')}</Link>
              </Button>
            </div>
          </Section>
        </div>

        <aside className="text-muted-foreground mt-5 flex items-start gap-2 text-xs leading-5">
          <ShieldCheck className="text-primary mt-0.5 size-4 shrink-0" />
          {tr(
            'As funções definem o que cada cliente pode publicar e assinar. Gerencie-as na página de funções.',
          )}
        </aside>
      </div>
    </main>
  )
}
