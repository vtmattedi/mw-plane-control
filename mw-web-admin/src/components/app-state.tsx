import { CircleAlert, FileQuestion, LoaderCircle, ShieldX } from 'lucide-react'
import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'

type StateKind = 'loading' | 'empty' | 'error' | 'unauthorized'

const icons = {
  loading: LoaderCircle,
  empty: FileQuestion,
  error: CircleAlert,
  unauthorized: ShieldX,
} satisfies Record<StateKind, typeof CircleAlert>

interface AppStateProps {
  action?: ReactNode
  className?: string
  description: string
  fullscreen?: boolean
  kind: StateKind
  title: string
}

export function AppState({
  action,
  className,
  description,
  fullscreen = false,
  kind,
  title,
}: AppStateProps) {
  const Icon = icons[kind]

  return (
    <section
      aria-live={kind === 'loading' ? 'polite' : undefined}
      className={cn(
        'app-state',
        `app-state-${kind}`,
        fullscreen && 'app-state-fullscreen',
        className,
      )}
      role={kind === 'error' ? 'alert' : 'status'}
    >
      <span className="app-state-icon" aria-hidden="true">
        <Icon />
      </span>
      <div>
        <h1>{title}</h1>
        <p>{description}</p>
      </div>
      {action && <div className="app-state-action">{action}</div>}
    </section>
  )
}

export function LoadingState({
  description = 'Aguarde só um instante.',
  fullscreen = false,
  title = 'Carregando',
}: Partial<Pick<AppStateProps, 'description' | 'fullscreen' | 'title'>>) {
  return (
    <AppState
      description={description}
      fullscreen={fullscreen}
      kind="loading"
      title={title}
    />
  )
}

export function EmptyState({
  description,
  title = 'Nada por aqui',
}: Pick<AppStateProps, 'description'> & Partial<Pick<AppStateProps, 'title'>>) {
  return <AppState description={description} kind="empty" title={title} />
}

export function ErrorState({
  description,
  title = 'Não foi possível carregar',
}: Pick<AppStateProps, 'description'> & Partial<Pick<AppStateProps, 'title'>>) {
  return <AppState description={description} kind="error" title={title} />
}

export function UnauthorizedState({
  action,
  description,
  title = 'Acesso negado',
}: Pick<AppStateProps, 'action' | 'description'> &
  Partial<Pick<AppStateProps, 'title'>>) {
  return (
    <AppState
      action={action}
      description={description}
      kind="unauthorized"
      title={title}
    />
  )
}
