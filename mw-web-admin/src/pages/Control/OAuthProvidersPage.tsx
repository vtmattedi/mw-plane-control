import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Save } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { toast } from 'sonner'
import { StatusBadge } from '@/components/control/status-badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { beginStepUp, ControlApiError, controlRequest } from '@/lib/control-api'
import {
  googleConfigToForm,
  googleFormError,
  googleFormToConfig,
  type GoogleOAuthConfig,
  type GoogleOAuthForm,
} from '@/lib/oauth-provider-config'
import { googleProviderQuery, useOAuthProject } from '@/lib/oauth-project'
import { usePreferences } from '@/preferences/preferences-context'

const pendingKey = 'mw-oauth-google-provider-pending'

const emptyForm: GoogleOAuthForm = {
  enabled: false,
  allowedScopes: '',
  allowedCallbackURLs: '',
  allowedReturnURLs: '',
  perMinute: '10',
  perHour: '100',
  maxPending: '50',
}

type SaveOperation = {
  projectID: string
  config: GoogleOAuthConfig
}

export default function OAuthProvidersPage() {
  const { tr } = usePreferences()
  const queryClient = useQueryClient()
  const { projectID = '' } = useParams()
  const { project } = useOAuthProject()
  const configQuery = useQuery(googleProviderQuery(projectID))
  const [draft, setForm] = useState<GoogleOAuthForm | null>(null)
  const form =
    draft ??
    (configQuery.data ? googleConfigToForm(configQuery.data) : emptyForm)

  const save = useMutation({
    mutationFn: ({ projectID: targetProjectID, config }: SaveOperation) =>
      controlRequest<GoogleOAuthConfig>(
        `/oauth/projects/${encodeURIComponent(targetProjectID)}/oauth/google`,
        { method: 'PUT', body: JSON.stringify(config) },
      ),
    onSuccess: (config, operation) => {
      sessionStorage.removeItem(pendingKey)
      setForm(googleConfigToForm(config))
      toast.success(tr('Configuração do provedor salva.'))
      void queryClient.invalidateQueries({
        queryKey: ['oauth-google-provider', operation.projectID],
      })
    },
    onError: (error, operation) => {
      if (
        error instanceof ControlApiError &&
        error.code === 'step_up_required'
      ) {
        sessionStorage.setItem(pendingKey, JSON.stringify(operation))
        beginStepUp(
          `/oauth/projects/${encodeURIComponent(operation.projectID)}/providers/google`,
        )
        return
      }
      toast.error(
        error instanceof Error
          ? error.message
          : tr('Não foi possível salvar a configuração.'),
      )
    },
  })

  useEffect(() => {
    const raw = sessionStorage.getItem(pendingKey)
    if (!raw) return
    try {
      const operation = JSON.parse(raw) as SaveOperation
      if (operation.projectID !== projectID) return
      sessionStorage.removeItem(pendingKey)
      save.mutate(operation)
    } catch {
      sessionStorage.removeItem(pendingKey)
    }
    // Resume once after returning from Identity step-up.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const validationError = googleFormError(form)

  function submit() {
    if (!projectID) return
    try {
      save.mutate({ projectID, config: googleFormToConfig(form) })
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : tr('Revise a configuração do provedor.'),
      )
    }
  }

  return (
    <Card className="border-border/70 shadow-sm">
      <CardHeader>
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <CardTitle>Google OAuth</CardTitle>
            <CardDescription className="mt-2">
              {tr('Políticas do Google OAuth para {project}.').replace(
                '{project}',
                String(project.name),
              )}
            </CardDescription>
          </div>
          <StatusBadge
            tone={
              configQuery.isError
                ? 'warning'
                : form.enabled
                  ? 'positive'
                  : 'neutral'
            }
          >
            {tr(
              configQuery.isError
                ? 'Estado indisponível'
                : configQuery.isPending
                  ? 'Carregando…'
                  : form.enabled
                    ? 'Ativo'
                    : 'Inativo',
            )}
          </StatusBadge>
        </div>
      </CardHeader>
      <CardContent className="space-y-6">
        {configQuery.isError && (
          <div role="alert" className="space-y-3">
            <p className="text-destructive text-sm">
              {tr('Não foi possível carregar a configuração do Google.')}
            </p>
            <Button
              onClick={() => void configQuery.refetch()}
              variant="outline"
            >
              {tr('Tentar novamente')}
            </Button>
          </div>
        )}

        <fieldset
          disabled={!configQuery.isSuccess || save.isPending}
          className="space-y-6 disabled:opacity-60"
        >
          <div className="border-border bg-muted/30 flex items-center justify-between gap-4 rounded-xl border p-4">
            <div>
              <p className="text-sm font-semibold">{tr('Ativar Google')}</p>
              <p className="text-muted-foreground mt-1 text-xs leading-5">
                {tr(
                  'Quando desativado, novas autorizações deste projeto serão recusadas.',
                )}
              </p>
            </div>
            <Switch
              aria-label={tr('Ativar Google')}
              checked={form.enabled}
              disabled={!projectID || configQuery.isLoading}
              onCheckedChange={(enabled) => setForm({ ...form, enabled })}
            />
          </div>

          <div className="grid gap-5">
            <PolicyField
              label={tr('Escopos permitidos')}
              hint={tr('Um escopo por linha ou separado por vírgula.')}
              placeholder="https://www.googleapis.com/auth/calendar"
              value={form.allowedScopes}
              onChange={(allowedScopes) => setForm({ ...form, allowedScopes })}
            />
            <PolicyField
              label={tr('URLs de callback permitidas')}
              hint={tr(
                'Endpoints backend que receberão o POST assinado do MW OAuth. Correspondência exata.',
              )}
              placeholder="https://app.example.com/api/oauth/google/result"
              value={form.allowedCallbackURLs}
              onChange={(allowedCallbackURLs) =>
                setForm({ ...form, allowedCallbackURLs })
              }
            />
            <PolicyField
              label={tr('URLs de retorno permitidas')}
              hint={tr(
                'Páginas para onde o usuário poderá retornar após a integração. Correspondência exata.',
              )}
              placeholder="https://app.example.com/settings/integrations"
              value={form.allowedReturnURLs}
              onChange={(allowedReturnURLs) =>
                setForm({ ...form, allowedReturnURLs })
              }
            />
          </div>

          <div>
            <h3 className="text-sm font-semibold">{tr('Limites')}</h3>
            <div className="mt-3 grid gap-4 sm:grid-cols-3">
              <NumberField
                label={tr('Por minuto')}
                value={form.perMinute}
                onChange={(perMinute) => setForm({ ...form, perMinute })}
              />
              <NumberField
                label={tr('Por hora')}
                value={form.perHour}
                onChange={(perHour) => setForm({ ...form, perHour })}
              />
              <NumberField
                label={tr('Máximo pendente')}
                value={form.maxPending}
                onChange={(maxPending) => setForm({ ...form, maxPending })}
              />
            </div>
          </div>

          {validationError && (
            <p className="text-destructive text-sm">{tr(validationError)}</p>
          )}

          <div className="flex items-center gap-3">
            <Button
              disabled={
                !configQuery.isSuccess ||
                save.isPending ||
                Boolean(validationError)
              }
              onClick={submit}
            >
              <Save />
              {save.isPending
                ? tr('Salvando…')
                : tr('Salvar alterações para {project}').replace(
                    '{project}',
                    String(project.name),
                  )}
            </Button>
          </div>
        </fieldset>
      </CardContent>
    </Card>
  )
}

function PolicyField({
  hint,
  label,
  onChange,
  placeholder,
  value,
}: {
  hint: string
  label: string
  onChange: (value: string) => void
  placeholder: string
  value: string
}) {
  return (
    <label className="space-y-2 text-sm font-medium">
      <span>{label}</span>
      <textarea
        aria-label={label}
        className="border-input bg-card text-foreground focus-visible:border-ring focus-visible:ring-ring/20 min-h-24 w-full resize-y rounded-xl border px-3 py-2 font-mono text-sm outline-none focus-visible:ring-4"
        placeholder={placeholder}
        value={value}
        onChange={(event) => onChange(event.target.value)}
      />
      <span className="text-muted-foreground block text-xs leading-5 font-normal">
        {hint}
      </span>
    </label>
  )
}

function NumberField({
  label,
  onChange,
  value,
}: {
  label: string
  onChange: (value: string) => void
  value: string
}) {
  return (
    <label className="space-y-2 text-sm font-medium">
      <span>{label}</span>
      <Input min={1} type="number" value={value} onChange={onChange} />
    </label>
  )
}
