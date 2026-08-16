import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { KeyRound, Network, Settings2, Trash2 } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { toast } from 'sonner'
import { useConfirm } from '@/components/confirm-context'
import { PageHeader } from '@/components/control/page-header'
import { OneTimeCredentialDialog } from '@/components/one-time-credential-dialog'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Select } from '@/components/ui/select'
import {
  beginStepUp,
  collectionFrom,
  ControlApiError,
  controlRequest,
} from '@/lib/control-api'
import { usePreferences } from '@/preferences/preferences-context'

export default function OAuthProjectConfigurationPage() {
  const { projectID = '' } = useParams()
  const navigate = useNavigate()
  const confirm = useConfirm()
  const { tr } = usePreferences()
  const queryClient = useQueryClient()
  const [revealedCredential, setRevealedCredential] = useState<string | null>(
    null,
  )
  const projectsQuery = useQuery({
    queryKey: ['control-resource', '/oauth/projects'],
    queryFn: () => controlRequest<unknown>('/oauth/projects'),
  })
  const projects = useMemo(
    () => collectionFrom(projectsQuery.data, 'projects'),
    [projectsQuery.data],
  )
  const project = projects.find((item) => String(item.id) === projectID)
  const pendingKey = 'mw-oauth-project-configuration-pending'

  const update = useMutation({
    mutationFn: (operation: {
      path: string
      method: 'PATCH' | 'POST' | 'DELETE'
      body?: unknown
    }) =>
      controlRequest<Record<string, unknown>>(operation.path, {
        method: operation.method,
        body: JSON.stringify(operation.body ?? {}),
      }),
    onSuccess: (data, operation) => {
      sessionStorage.removeItem(pendingKey)
      if (operation.method === 'DELETE') {
        toast.success(tr('Projeto excluído.'))
        void queryClient.invalidateQueries({ queryKey: ['control-resource'] })
        navigate('/oauth/projects', { replace: true })
        return
      }
      const secret = data.api_key ?? data.callback_secret ?? data.client_secret
      if (typeof secret === 'string') {
        setRevealedCredential(secret)
        toast.success(tr('Credencial rotacionada.'))
      } else {
        toast.success(tr('Configuração salva.'))
      }
      void queryClient.invalidateQueries({
        queryKey: ['control-resource', '/oauth/projects'],
      })
    },
    onError: (error, operation) => {
      if (
        error instanceof ControlApiError &&
        error.code === 'step_up_required'
      ) {
        sessionStorage.setItem(pendingKey, JSON.stringify(operation))
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
    const raw = sessionStorage.getItem(pendingKey)
    if (!raw) return
    try {
      sessionStorage.removeItem(pendingKey)
      update.mutate(
        JSON.parse(raw) as {
          path: string
          method: 'PATCH' | 'POST' | 'DELETE'
          body?: unknown
        },
      )
    } catch {
      sessionStorage.removeItem(pendingKey)
    }
    // Resume once after returning from Identity step-up.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  async function toggleProject() {
    if (!project) return
    const enabled = project.enabled === true
    const accepted = await confirm({
      title: tr(enabled ? 'Desativar projeto?' : 'Ativar projeto?'),
      description: tr(
        enabled
          ? 'As integrações deste projeto deixarão de funcionar até ele ser reativado.'
          : 'As integrações deste projeto voltarão a funcionar.',
      ),
      confirmLabel: tr(enabled ? 'Desativar' : 'Ativar'),
    })
    if (accepted) {
      update.mutate({
        path: `/oauth/projects/${projectID}`,
        method: 'PATCH',
        body: { enabled: !enabled },
      })
    }
  }

  async function rotateKey() {
    const accepted = await confirm({
      title: tr('Rotacionar chave?'),
      description: tr(
        'A chave atual deixará de funcionar e a nova será exibida apenas uma vez.',
      ),
      confirmLabel: tr('Rotacionar chave'),
    })
    if (accepted) {
      update.mutate({
        path: `/oauth/projects/${projectID}/rotate-key`,
        method: 'POST',
      })
    }
  }

  async function rotateCallbackSecret() {
    const accepted = await confirm({
      title: tr('Rotacionar segredo de callback?'),
      description: tr(
        'O segredo atual deixará de validar callbacks e o novo será exibido apenas uma vez.',
      ),
      confirmLabel: tr('Rotacionar segredo'),
    })
    if (accepted) {
      update.mutate({
        path: `/oauth/projects/${projectID}/rotate-callback-secret`,
        method: 'POST',
      })
    }
  }

  async function deleteProject() {
    const accepted = await confirm({
      title: tr('Excluir projeto?'),
      description: tr(
        'Esta ação é permanente. Credenciais e configurações deste projeto serão removidas.',
      ),
      confirmLabel: tr('Excluir projeto'),
    })
    if (accepted) {
      update.mutate({
        path: `/oauth/projects/${projectID}`,
        method: 'DELETE',
      })
    }
  }

  return (
    <main className="bg-background min-h-dvh px-4 py-7 sm:px-7 lg:px-10">
      <div className="mx-auto w-full max-w-5xl">
        <PageHeader
          description={tr(
            'Gerencie as credenciais e o estado deste projeto OAuth.',
          )}
          eyebrow="OAuth"
          icon={Settings2}
          title={tr('Configuração do projeto')}
        />

        <Card className="border-border/70 mt-7 shadow-sm">
          <CardHeader>
            <CardTitle>{String(project?.name ?? tr('Projeto'))}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-5">
            <label className="block max-w-md space-y-2 text-sm font-medium">
              <span>{tr('Projeto')}</span>
              <Select
                disabled={projectsQuery.isLoading}
                onChange={(event) =>
                  navigate(
                    `/oauth/projects/${event.target.value}/configuration`,
                  )
                }
                value={projectID}
              >
                {!projects.length && <option value="">—</option>}
                {projects.map((item) => (
                  <option key={String(item.id)} value={String(item.id)}>
                    {String(item.name ?? item.id)}
                  </option>
                ))}
              </Select>
            </label>

            {project && (
              <div className="border-border grid gap-3 rounded-xl border p-4 text-sm sm:grid-cols-2">
                <ProjectValue label="ID" value={String(project.id)} />
                <ProjectValue
                  label={tr('Status')}
                  value={tr(project.enabled === true ? 'Ativo' : 'Inativo')}
                />
              </div>
            )}

            <div className="flex flex-wrap gap-3">
              <Button
                disabled={!project || update.isPending}
                onClick={() => void toggleProject()}
                variant="outline"
              >
                {tr(project?.enabled === true ? 'Desativar' : 'Ativar')}
              </Button>
              <Button
                disabled={!project || update.isPending}
                onClick={() => void rotateKey()}
              >
                <KeyRound /> {tr('Rotacionar chave')}
              </Button>
              <Button
                disabled={!project || update.isPending}
                onClick={() => void rotateCallbackSecret()}
                variant="outline"
              >
                <KeyRound /> {tr('Rotacionar segredo de callback')}
              </Button>
              <Button
                disabled={!project}
                onClick={() =>
                  navigate(`/oauth/providers?project=${projectID}`)
                }
                variant="outline"
              >
                <Network /> {tr('Configurar Google')}
              </Button>
            </div>
          </CardContent>
        </Card>

        <Card className="border-destructive/25 mt-6 shadow-sm">
          <CardHeader>
            <CardTitle className="text-destructive flex items-center gap-2">
              <Trash2 className="size-4" /> {tr('Excluir projeto')}
            </CardTitle>
          </CardHeader>
          <CardContent className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
            <p className="text-muted-foreground max-w-2xl text-sm">
              {tr(
                'A exclusão remove permanentemente as credenciais e configurações deste projeto.',
              )}
            </p>
            <Button
              disabled={!project || update.isPending}
              onClick={() => void deleteProject()}
              variant="destructive"
            >
              <Trash2 /> {tr('Excluir projeto')}
            </Button>
          </CardContent>
        </Card>
        <OneTimeCredentialDialog
          credential={revealedCredential}
          onClose={() => setRevealedCredential(null)}
        />
      </div>
    </main>
  )
}

function ProjectValue({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <p className="text-muted-foreground text-xs">{label}</p>
      <p className="mt-1 font-medium">{value}</p>
    </div>
  )
}
