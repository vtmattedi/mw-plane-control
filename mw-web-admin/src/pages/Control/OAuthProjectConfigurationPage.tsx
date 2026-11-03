import { useMutation, useQueryClient } from '@tanstack/react-query'
import { KeyRound, Network, Trash2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Link, useLocation, useNavigate, useParams } from 'react-router-dom'
import { toast } from 'sonner'
import { useConfirm } from '@/components/confirm-context'
import { OneTimeCredentialDialog } from '@/components/one-time-credential-dialog'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { beginStepUp, ControlApiError, controlRequest } from '@/lib/control-api'
import { useOAuthProject } from '@/lib/oauth-project'
import { usePreferences } from '@/preferences/preferences-context'
import { OAuthGoogleStatus } from './OAuthGoogleStatus'

type ProjectOperation = {
  projectID: string
  returnTo: string
  path: string
  method: 'PATCH' | 'POST' | 'DELETE'
  body?: unknown
}
const pendingKey = 'mw-oauth-project-configuration-pending'

export default function OAuthProjectConfigurationPage({
  section = 'overview',
}: {
  section?: 'overview' | 'credentials'
}) {
  const { projectID = '' } = useParams()
  const { project } = useOAuthProject()
  const navigate = useNavigate()
  const location = useLocation()
  const confirm = useConfirm()
  const { tr } = usePreferences()
  const queryClient = useQueryClient()
  const [revealedCredential, setRevealedCredential] = useState<string | null>(
    null,
  )
  const base = `/oauth/projects/${encodeURIComponent(projectID)}`

  const update = useMutation({
    mutationFn: (operation: ProjectOperation) =>
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
        beginStepUp(operation.returnTo)
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
      const operation = JSON.parse(raw) as ProjectOperation
      if (
        operation.projectID !== projectID ||
        operation.returnTo !== location.pathname
      )
        return
      sessionStorage.removeItem(pendingKey)
      update.mutate(operation)
    } catch {
      sessionStorage.removeItem(pendingKey)
    }
    // Resume once, only on the project and tab that requested step-up.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  function runOperation(
    operation: Omit<ProjectOperation, 'projectID' | 'returnTo'>,
  ) {
    update.mutate({ ...operation, projectID, returnTo: location.pathname })
  }

  async function toggleProject() {
    const enabled = project.enabled === true
    if (
      await confirm({
        title: tr(enabled ? 'Desativar projeto?' : 'Ativar projeto?'),
        description: tr(
          enabled
            ? 'As integrações deste projeto deixarão de funcionar até ele ser reativado.'
            : 'As integrações deste projeto voltarão a funcionar.',
        ),
        confirmLabel: tr(enabled ? 'Desativar' : 'Ativar'),
      })
    )
      runOperation({ path: base, method: 'PATCH', body: { enabled: !enabled } })
  }

  async function rotateKey() {
    if (
      await confirm({
        title: tr('Rotacionar chave?'),
        description: tr(
          'A chave atual deixará de funcionar e a nova será exibida apenas uma vez.',
        ),
        confirmLabel: tr('Rotacionar chave'),
      })
    )
      runOperation({ path: `${base}/rotate-key`, method: 'POST' })
  }

  async function rotateCallbackSecret() {
    if (
      await confirm({
        title: tr('Rotacionar segredo de callback?'),
        description: tr(
          'O segredo atual deixará de validar callbacks e o novo será exibido apenas uma vez.',
        ),
        confirmLabel: tr('Rotacionar segredo'),
      })
    )
      runOperation({ path: `${base}/rotate-callback-secret`, method: 'POST' })
  }

  async function deleteProject() {
    if (
      await confirm({
        title: tr('Excluir projeto?'),
        description: tr(
          'Esta ação é permanente. Credenciais e configurações deste projeto serão removidas.',
        ),
        confirmLabel: tr('Excluir projeto'),
      })
    )
      runOperation({ path: base, method: 'DELETE' })
  }

  return (
    <>
      {section === 'credentials' ? (
        <Card className="border-border/70 shadow-sm">
          <CardHeader>
            <CardTitle>{tr('Credenciais')}</CardTitle>
            <CardDescription>
              {tr(
                'Gerencie as credenciais de integração deste projeto. Novos segredos são exibidos apenas uma vez.',
              )}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            <div className="flex flex-wrap items-center justify-between gap-4 border-b pb-6">
              <div>
                <h2 className="text-sm font-semibold">{tr('Chave de API')}</h2>
                <p className="text-muted-foreground mt-1 text-sm">
                  {tr('Autentica as chamadas deste projeto ao serviço OAuth.')}
                </p>
              </div>
              <Button
                disabled={update.isPending}
                onClick={() => void rotateKey()}
              >
                <KeyRound />
                {tr('Rotacionar chave')}
              </Button>
            </div>
            <div className="flex flex-wrap items-center justify-between gap-4">
              <div>
                <h2 className="text-sm font-semibold">
                  {tr('Segredo de callback')}
                </h2>
                <p className="text-muted-foreground mt-1 text-sm">
                  {tr('Valida os callbacks assinados enviados a este projeto.')}
                </p>
              </div>
              <Button
                disabled={update.isPending}
                onClick={() => void rotateCallbackSecret()}
                variant="outline"
              >
                <KeyRound />
                {tr('Rotacionar segredo de callback')}
              </Button>
            </div>
          </CardContent>
        </Card>
      ) : (
        <>
          <Card className="border-border/70 shadow-sm">
            <CardHeader>
              <CardTitle>{tr('Visão geral')}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-5">
              <dl className="grid gap-4 text-sm sm:grid-cols-2">
                <div>
                  <dt className="text-muted-foreground">ID</dt>
                  <dd className="mt-1 font-medium">{String(project.id)}</dd>
                </div>
                <div>
                  <dt className="text-muted-foreground">{tr('Status')}</dt>
                  <dd className="mt-1 font-medium">
                    {tr(project.enabled === true ? 'Ativo' : 'Inativo')}
                  </dd>
                </div>
              </dl>
              <Button
                disabled={update.isPending}
                onClick={() => void toggleProject()}
                variant="outline"
              >
                {tr(project.enabled === true ? 'Desativar' : 'Ativar')}
              </Button>
            </CardContent>
          </Card>
          <Card className="border-border/70 mt-6 shadow-sm">
            <CardHeader>
              <CardTitle>{tr('Provedores')}</CardTitle>
              <CardDescription>
                {tr('Políticas de autorização deste projeto.')}
              </CardDescription>
            </CardHeader>
            <CardContent className="flex flex-wrap items-center justify-between gap-4">
              <div className="flex items-center gap-3">
                <Network className="text-muted-foreground size-5" />
                <span className="font-medium">Google OAuth</span>
                <OAuthGoogleStatus projectID={projectID} />
              </div>
              <Button asChild variant="outline">
                <Link to={`${base}/providers/google`}>
                  {tr('Configurar Google')}
                </Link>
              </Button>
            </CardContent>
          </Card>
          <Card className="border-destructive/25 mt-6 shadow-sm">
            <CardHeader>
              <CardTitle className="text-destructive flex items-center gap-2">
                <Trash2 className="size-4" />
                {tr('Excluir projeto')}
              </CardTitle>
            </CardHeader>
            <CardContent className="flex flex-wrap items-center justify-between gap-4">
              <p className="text-muted-foreground max-w-2xl text-sm">
                {tr(
                  'A exclusão remove permanentemente as credenciais e configurações deste projeto.',
                )}
              </p>
              <Button
                disabled={update.isPending}
                onClick={() => void deleteProject()}
                variant="destructive"
              >
                <Trash2 />
                {tr('Excluir projeto')}
              </Button>
            </CardContent>
          </Card>
        </>
      )}
      <OneTimeCredentialDialog
        credential={revealedCredential}
        onClose={() => setRevealedCredential(null)}
      />
    </>
  )
}
