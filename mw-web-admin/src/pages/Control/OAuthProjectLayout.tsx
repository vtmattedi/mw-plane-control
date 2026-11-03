import { useQuery } from '@tanstack/react-query'
import { ChevronRight, FolderKanban } from 'lucide-react'
import {
  Link,
  Navigate,
  NavLink,
  Outlet,
  useParams,
  useSearchParams,
} from 'react-router-dom'
import { StatusBadge } from '@/components/control/status-badge'
import { Button } from '@/components/ui/button'
import { collectionFrom, controlRequest } from '@/lib/control-api'
import { usePreferences } from '@/preferences/preferences-context'

export default function OAuthProjectLayout() {
  const { projectID = '' } = useParams()
  const { tr } = usePreferences()
  const query = useQuery({
    queryKey: ['control-resource', '/oauth/projects'],
    queryFn: () => controlRequest<unknown>('/oauth/projects'),
  })
  const project = collectionFrom(query.data, 'projects').find(
    (item) => String(item.id) === projectID,
  )
  const base = `/oauth/projects/${encodeURIComponent(projectID)}`
  const tabs = [
    { label: 'Visão geral', to: base, end: true },
    { label: 'Credenciais', to: `${base}/credentials` },
    { label: 'Provedores', to: `${base}/providers/google` },
    { label: 'Atividade', to: `${base}/activity` },
  ]

  return (
    <main className="bg-background min-h-dvh px-4 py-7 sm:px-7 lg:px-10">
      <div className="mx-auto w-full max-w-5xl">
        <div className="bg-background/95 sticky top-16 z-10 pt-1 backdrop-blur lg:top-0">
          <nav
            aria-label={tr('Localização')}
            className="text-muted-foreground mb-5 flex items-center gap-2 text-sm"
          >
            <Link
              className="hover:text-primary underline-offset-4 hover:underline"
              to="/oauth/projects"
            >
              {tr('Projetos OAuth')}
            </Link>
            <ChevronRight aria-hidden="true" className="size-3.5" />
            <span aria-current="page">
              {String(project?.name ?? tr('Projeto'))}
            </span>
          </nav>
          <div className="flex items-start gap-3">
            <FolderKanban
              aria-hidden="true"
              className="text-primary mt-1 size-6 shrink-0"
            />
            <div className="min-w-0 flex-1">
              <div className="flex flex-wrap items-center gap-3">
                <h1 className="font-display text-3xl font-semibold tracking-tight break-words">
                  {String(project?.name ?? tr('Projeto'))}
                </h1>
                {project && (
                  <StatusBadge
                    tone={project.enabled === true ? 'positive' : 'neutral'}
                  >
                    {tr(project.enabled === true ? 'Ativo' : 'Inativo')}
                  </StatusBadge>
                )}
              </div>
              <p className="text-muted-foreground mt-2 text-sm">
                {tr('Configuração e atividade deste projeto OAuth.')}
              </p>
              {project && (
                <p className="text-muted-foreground mt-1 text-xs">
                  ID: {String(project.id)}
                </p>
              )}
            </div>
          </div>
          {project && !query.isError && (
            <nav
              aria-label={tr('Navegação do projeto')}
              className="border-border mt-6 flex gap-5 overflow-x-auto border-b"
            >
              {tabs.map((tab) => (
                <NavLink
                  key={tab.label}
                  to={tab.to}
                  end={tab.end}
                  className={({ isActive }) =>
                    `shrink-0 border-b-2 px-1 py-3 text-sm font-medium transition-colors ${isActive ? 'border-primary text-primary' : 'text-muted-foreground hover:text-foreground border-transparent'}`
                  }
                >
                  {tr(tab.label)}
                </NavLink>
              ))}
            </nav>
          )}
        </div>

        {query.isPending ? (
          <p className="text-muted-foreground py-10 text-sm" role="status">
            {tr('Carregando dados…')}
          </p>
        ) : query.isError ? (
          <section className="py-10" role="alert">
            <p className="text-destructive mb-4 text-sm">
              {tr('Não foi possível carregar o projeto.')}
            </p>
            <Button onClick={() => void query.refetch()} variant="outline">
              {tr('Tentar novamente')}
            </Button>
          </section>
        ) : !project ? (
          <section className="py-10">
            <h2 className="text-lg font-semibold">
              {tr('Projeto não encontrado')}
            </h2>
            <p className="text-muted-foreground mt-2 mb-5 text-sm">
              {tr('Escolha um projeto para configurar seus provedores.')}
            </p>
            <Button asChild variant="outline">
              <Link to="/oauth/projects">{tr('Voltar aos projetos')}</Link>
            </Button>
          </section>
        ) : (
          <div className="pt-7">
            <Outlet key={projectID} context={{ project }} />
          </div>
        )}
      </div>
    </main>
  )
}

export function OAuthProvidersRedirect() {
  const [params] = useSearchParams()
  const projectID = params.get('project')
  return (
    <Navigate
      replace
      to={
        projectID
          ? `/oauth/projects/${encodeURIComponent(projectID)}/providers/google`
          : '/oauth/projects'
      }
    />
  )
}
