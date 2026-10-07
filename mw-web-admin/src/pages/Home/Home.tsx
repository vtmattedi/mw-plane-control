import { useQueries } from '@tanstack/react-query'
import {
  Activity,
  AppWindow,
  ArrowUpRight,
  KeyRound,
  Mail,
  Network,
  Radio,
  ShieldCheck,
  Users,
} from 'lucide-react'
import { Link } from 'react-router-dom'
import { MetricCard } from '@/components/control/metric-card'
import { StatusBadge } from '@/components/control/status-badge'
import { Card } from '@/components/ui/card'
import { controlRequest } from '@/lib/control-api'
import { usePreferences } from '@/preferences/preferences-context'

type Module = {
  name: string
  description: string
  icon: typeof ShieldCheck
  to: string
  endpoint: string
  accent: string
  // A reachable service is not always a healthy one: when set, the module counts as
  // online only if this accepts the response.
  healthy?: (body: unknown) => boolean
}

const modules: Module[] = [
  {
    name: 'Identity',
    description: 'Pessoas, aplicações e políticas de acesso.',
    icon: ShieldCheck,
    to: '/identity/users',
    endpoint: '/identity/whoami',
    accent: 'bg-[#ff9849]/12 text-[#d96d1d]',
  },
  {
    name: 'OAuth',
    description: 'Projetos, provedores e credenciais.',
    icon: Network,
    to: '/oauth/projects',
    endpoint: '/oauth/projects',
    accent: 'bg-[#6f7cff]/12 text-[#5664dc]',
  },
  {
    name: 'Mail',
    description: 'Entrega, projetos e consumo de e-mail.',
    icon: Mail,
    to: '/mail/projects',
    endpoint: '/mail/health',
    accent: 'bg-[#23a77b]/12 text-[#168361]',
  },
  {
    name: 'MQTT',
    description: 'Clientes, credenciais e estado do broker.',
    icon: Radio,
    to: '/mqtt/clients',
    endpoint: '/mqtt/server',
    accent: 'bg-[#2fa4d9]/12 text-[#1d7fb0]',
    // The admin API answers even while its broker connection is down.
    healthy: (body) =>
      (body as { brokerConnected?: unknown } | null)?.brokerConnected === true,
  },
]

export default function Home() {
  const { tr } = usePreferences()
  const queries = useQueries({
    queries: modules.map((module) => ({
      queryKey: ['module-health', module.name],
      queryFn: async () => {
        const body = await controlRequest<unknown>(module.endpoint)
        if (module.healthy && !module.healthy(body)) {
          throw new Error(`${module.name} is not healthy`)
        }
        return body
      },
      retry: 0,
      refetchInterval: 60_000,
    })),
  })
  const online = queries.filter((query) => query.isSuccess).length

  return (
    <main className="bg-background min-h-dvh px-4 py-7 sm:px-7 lg:px-10">
      <div className="mx-auto w-full max-w-[94rem]">
        <header className="bg-primary text-primary-foreground shadow-primary/10 relative overflow-hidden rounded-[2rem] px-6 py-8 shadow-2xl sm:px-9 sm:py-10">
          <div className="pointer-events-none absolute -top-28 -right-24 size-80 rounded-full border border-white/10" />
          <div className="pointer-events-none absolute -top-14 -right-8 size-56 rounded-full border border-white/10" />
          <div className="relative max-w-3xl">
            <div className="text-primary-foreground/65 mb-6 flex items-center gap-2 text-xs font-semibold tracking-[0.16em] uppercase">
              <span className="bg-brand-orange size-2 rounded-full shadow-[0_0_0_5px_color-mix(in_srgb,var(--brand-orange)_18%,transparent)]" />
              MattediWorks control plane
            </div>
            <h1 className="font-display text-4xl font-semibold tracking-[-0.045em] sm:text-5xl">
              {tr('Controle claro. Confiança por padrão.')}
            </h1>
            <p className="text-primary-foreground/70 mt-4 max-w-2xl text-sm leading-6 sm:text-base">
              {tr(
                'Uma visão central para identidades, autorização e comunicação em todos os produtos MattediWorks.',
              )}
            </p>
          </div>
        </header>

        <section
          className="mt-6 grid gap-4 sm:grid-cols-2 xl:grid-cols-4"
          aria-label={tr('Resumo da plataforma')}
        >
          <MetricCard
            icon={Activity}
            label={tr('Módulos online')}
            note={tr('Verificação automática a cada minuto')}
            value={`${online}/${modules.length}`}
            tone="accent"
          />
          <MetricCard
            icon={Users}
            label={tr('Identidade')}
            note={tr('Autorização centralizada')}
            value="OIDC"
          />
          <MetricCard
            icon={KeyRound}
            label={tr('Sessão')}
            note={tr('Tokens mantidos no BFF')}
            value={tr('Protegida')}
          />
          <MetricCard
            icon={AppWindow}
            label={tr('Plano')}
            note={tr('Aplicações MattediWorks')}
            value="v1.1"
          />
        </section>

        <section className="mt-9">
          <div className="mb-4 flex items-end justify-between">
            <div>
              <p className="text-primary text-xs font-bold tracking-[0.14em] uppercase">
                {tr('Espaços de trabalho')}
              </p>
              <h2 className="mt-1 text-xl font-semibold tracking-tight">
                {tr('Módulos do control plane')}
              </h2>
            </div>
            <span className="text-muted-foreground hidden text-xs sm:block">
              {tr('Dados atualizados em tempo real')}
            </span>
          </div>
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
            {modules.map((module, index) => {
              const Icon = module.icon
              const query = queries[index]
              return (
                <Link className="group" key={module.name} to={module.to}>
                  <Card className="border-border/70 group-hover:border-primary/25 h-full p-5 shadow-sm transition-all duration-200 group-hover:-translate-y-0.5 group-hover:shadow-lg">
                    <div className="flex items-start justify-between">
                      <span
                        className={`grid size-11 place-items-center rounded-2xl ${module.accent}`}
                      >
                        <Icon className="size-5" />
                      </span>
                      <ArrowUpRight className="text-muted-foreground size-4 transition-transform group-hover:translate-x-0.5 group-hover:-translate-y-0.5" />
                    </div>
                    <h3 className="mt-6 text-lg font-semibold">
                      {module.name}
                    </h3>
                    <p className="text-muted-foreground mt-1 min-h-10 text-sm leading-5">
                      {tr(module.description)}
                    </p>
                    <div className="border-border/60 mt-5 border-t pt-4">
                      <StatusBadge
                        tone={
                          query.isSuccess
                            ? 'positive'
                            : query.isLoading
                              ? 'neutral'
                              : 'warning'
                        }
                      >
                        {query.isSuccess
                          ? tr('Conectado')
                          : query.isLoading
                            ? tr('Verificando')
                            : tr('Indisponível')}
                      </StatusBadge>
                    </div>
                  </Card>
                </Link>
              )
            })}
          </div>
        </section>
      </div>
    </main>
  )
}
