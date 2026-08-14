import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Globe2, Settings2, Trash2 } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { toast } from 'sonner'
import { useConfirm } from '@/components/confirm-context'
import { PageHeader } from '@/components/control/page-header'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Select } from '@/components/ui/select'
import { Input } from '@/components/ui/input'
import {
  beginStepUp,
  collectionFrom,
  ControlApiError,
  controlRequest,
} from '@/lib/control-api'
import { usePreferences } from '@/preferences/preferences-context'

type DomainRule = {
  project_id: number
  allowed_request_domains: string
}

export default function MailConfigurationPage() {
  const { tr } = usePreferences()
  const confirm = useConfirm()
  const { projectID: routeProjectID } = useParams()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [projectID, setProjectID] = useState(routeProjectID ?? '')
  const [rule, setRule] = useState('')
  const projectsQuery = useQuery({
    queryKey: ['control-resource', '/mail/projects'],
    queryFn: () => controlRequest<unknown>('/mail/projects'),
  })
  const projects = useMemo(
    () => collectionFrom(projectsQuery.data, 'projects'),
    [projectsQuery.data],
  )

  useEffect(() => {
    if (routeProjectID) setProjectID(routeProjectID)
  }, [routeProjectID])

  useEffect(() => {
    if (!projectID && projects[0]?.id != null) {
      setProjectID(String(projects[0].id))
    }
  }, [projectID, projects])

  const domainQuery = useQuery({
    queryKey: ['mail-request-domains', projectID],
    queryFn: () =>
      controlRequest<DomainRule>(
        `/mail/projects/${encodeURIComponent(projectID)}/request-domains`,
      ),
    enabled: Boolean(projectID),
  })

  useEffect(() => {
    if (domainQuery.data) {
      setRule(domainQuery.data.allowed_request_domains)
    }
  }, [domainQuery.data])

  const save = useMutation({
    mutationFn: () =>
      controlRequest<DomainRule>(
        `/mail/projects/${encodeURIComponent(projectID)}/request-domains`,
        {
          method: 'PUT',
          body: JSON.stringify({ allowed_request_domains: rule }),
        },
      ),
    onSuccess: (data) => {
      setRule(data.allowed_request_domains)
      toast.success(tr('Configuração salva.'))
      void queryClient.invalidateQueries({
        queryKey: ['mail-request-domains', projectID],
      })
    },
    onError: (error) =>
      toast.error(
        error instanceof Error
          ? error.message
          : tr('Não foi possível salvar a configuração.'),
      ),
  })

  const deletePendingKey = 'mw-mail-project-delete-pending'
  const remove = useMutation({
    mutationFn: () =>
      controlRequest<void>(`/mail/projects/${encodeURIComponent(projectID)}`, {
        method: 'DELETE',
      }),
    onSuccess: () => {
      sessionStorage.removeItem(deletePendingKey)
      toast.success(tr('Projeto excluído.'))
      void queryClient.invalidateQueries({ queryKey: ['control-resource'] })
      navigate('/mail/projects', { replace: true })
    },
    onError: (error) => {
      if (
        error instanceof ControlApiError &&
        error.code === 'step_up_required'
      ) {
        sessionStorage.setItem(deletePendingKey, projectID)
        beginStepUp(window.location.pathname)
        return
      }
      toast.error(
        error instanceof Error
          ? error.message
          : tr('Não foi possível concluir a operação.'),
      )
    },
  })

  useEffect(() => {
    const pendingProjectID = sessionStorage.getItem(deletePendingKey)
    if (!pendingProjectID || pendingProjectID !== projectID) return
    sessionStorage.removeItem(deletePendingKey)
    remove.mutate()
    // Resume once after returning from Identity step-up.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [projectID])

  async function deleteProject() {
    const accepted = await confirm({
      title: tr('Excluir projeto?'),
      description: tr(
        'Esta ação é permanente. Credenciais e configurações deste projeto serão removidas.',
      ),
      confirmLabel: tr('Excluir projeto'),
    })
    if (accepted) remove.mutate()
  }

  return (
    <main className="bg-background min-h-dvh px-4 py-7 sm:px-7 lg:px-10">
      <div className="mx-auto w-full max-w-5xl">
        <PageHeader
          description={tr(
            'Domínios de requisição, credenciais e políticas de entrega por projeto.',
          )}
          eyebrow="Mail"
          icon={Settings2}
          title={tr('Configuração')}
        />

        <Card className="border-border/70 mt-7 shadow-sm">
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Globe2 className="text-brand-orange size-4" />
              {tr('Domínios permitidos')}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="grid gap-5 sm:grid-cols-2">
              <label className="space-y-2 text-sm font-medium">
                <span>{tr('Projeto')}</span>
                <Select
                  disabled={projectsQuery.isLoading}
                  onChange={(event) => {
                    const nextProjectID = event.target.value
                    setProjectID(nextProjectID)
                    if (nextProjectID) {
                      navigate(`/mail/configuration/${nextProjectID}`)
                    }
                  }}
                  value={projectID}
                >
                  {!projects.length && <option value="">—</option>}
                  {projects.map((project) => (
                    <option key={String(project.id)} value={String(project.id)}>
                      {String(project.name ?? project.id)}
                    </option>
                  ))}
                </Select>
              </label>

              <label className="space-y-2 text-sm font-medium">
                <span>{tr('Domínios permitidos')}</span>
                <Input
                  disabled={!projectID || domainQuery.isLoading}
                  onChange={setRule}
                  placeholder="localhost,example.com,*.example.com"
                  value={rule}
                />
              </label>
            </div>

            <p className="text-muted-foreground mt-3 text-xs leading-5">
              {tr(
                'Use somente hostnames separados por vírgula, sem http:// ou porta. Para o MailTester local, inclua localhost.',
              )}
            </p>

            <div className="mt-5 flex items-center gap-3">
              <Button
                disabled={!projectID || !rule.trim() || save.isPending}
                onClick={() => save.mutate()}
              >
                {save.isPending ? tr('Salvando…') : tr('Salvar')}
              </Button>
            </div>
          </CardContent>
        </Card>

        <Card className="border-destructive/25 mt-6 shadow-sm">
          <CardHeader>
            <CardTitle className="text-destructive flex items-center gap-2">
              <Trash2 className="size-4" />
              {tr('Excluir projeto')}
            </CardTitle>
          </CardHeader>
          <CardContent className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
            <p className="text-muted-foreground max-w-2xl text-sm">
              {tr(
                'A exclusão remove permanentemente as credenciais e configurações deste projeto.',
              )}
            </p>
            <Button
              disabled={!projectID || remove.isPending}
              onClick={() => void deleteProject()}
              variant="destructive"
            >
              <Trash2 /> {tr('Excluir projeto')}
            </Button>
          </CardContent>
        </Card>
      </div>
    </main>
  )
}
