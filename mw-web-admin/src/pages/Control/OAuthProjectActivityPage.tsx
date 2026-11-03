import { useQuery } from '@tanstack/react-query'
import { RefreshCw } from 'lucide-react'
import { useParams } from 'react-router-dom'
import { ResourceTable } from '@/components/control/resource-table'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { collectionFrom, controlRequest } from '@/lib/control-api'
import { usePreferences } from '@/preferences/preferences-context'

export default function OAuthProjectActivityPage() {
  const { projectID = '' } = useParams()
  const { tr } = usePreferences()
  const query = useQuery({
    queryKey: ['oauth-project-activity', projectID],
    queryFn: () =>
      controlRequest<unknown>(
        `/oauth/audit?project_id=${encodeURIComponent(projectID)}&limit=50`,
      ),
    enabled: Boolean(projectID),
  })
  return (
    <section>
      <div className="mb-5 flex items-start justify-between gap-4">
        <div>
          <h2 className="text-lg font-semibold">{tr('Atividade recente')}</h2>
          <p className="text-muted-foreground mt-1 text-sm">
            {tr('Últimos 50 eventos deste projeto.')}
          </p>
        </div>
        <Button
          aria-label={tr('Atualizar')}
          disabled={query.isFetching}
          onClick={() => void query.refetch()}
          size="icon"
          variant="outline"
        >
          <RefreshCw className={query.isFetching ? 'animate-spin' : ''} />
        </Button>
      </div>
      {query.isError ? (
        <p className="text-destructive text-sm" role="alert">
          {tr('Não foi possível carregar a atividade.')}
        </p>
      ) : (
        <Card className="overflow-hidden">
          <ResourceTable
            rows={collectionFrom(query.data, 'items')}
            columns={[
              { key: 'event_type', label: tr('Evento') },
              { key: 'metadata.actor_identity_sub', label: tr('Ator') },
              { key: 'status', label: tr('Status') },
              { key: 'created_at', label: tr('Data') },
            ]}
            emptyTitle={tr(
              query.isPending ? 'Carregando dados…' : 'Nenhum evento OAuth',
            )}
            emptyDescription={tr('As operações deste projeto aparecerão aqui.')}
          />
        </Card>
      )}
    </section>
  )
}
