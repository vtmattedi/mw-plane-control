import { useQuery } from '@tanstack/react-query'
import { StatusBadge } from '@/components/control/status-badge'
import { googleProviderQuery } from '@/lib/oauth-project'
import { usePreferences } from '@/preferences/preferences-context'

export function OAuthGoogleStatus({ projectID }: { projectID: string }) {
  const { tr } = usePreferences()
  const query = useQuery(googleProviderQuery(projectID))
  return (
    <StatusBadge
      tone={
        query.isError ? 'warning' : query.data?.enabled ? 'positive' : 'neutral'
      }
    >
      {tr(
        query.isError
          ? 'Estado indisponível'
          : query.isPending
            ? 'Carregando…'
            : query.data.enabled
              ? 'Google ativo'
              : 'Google inativo',
      )}
    </StatusBadge>
  )
}
