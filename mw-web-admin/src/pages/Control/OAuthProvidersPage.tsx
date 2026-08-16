import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Network, Save } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { toast } from 'sonner'
import { PageHeader } from '@/components/control/page-header'
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
import { Select } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import {
  beginStepUp,
  collectionFrom,
  ControlApiError,
  controlRequest,
} from '@/lib/control-api'
import {
  googleConfigToForm,
  googleFormError,
  googleFormToConfig,
  type GoogleOAuthConfig,
  type GoogleOAuthForm,
} from '@/lib/oauth-provider-config'
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
  const [searchParams, setSearchParams] = useSearchParams()
  const [form, setForm] = useState<GoogleOAuthForm>(emptyForm)
  const projectsQuery = useQuery({
    queryKey: ['control-resource', '/oauth/projects'],
    queryFn: () => controlRequest<unknown>('/oauth/projects'),
  })
  const projects = useMemo(
    () => collectionFrom(projectsQuery.data, 'projects'),
    [projectsQuery.data],
  )
  const requestedProjectID = searchParams.get('project') ?? ''
  const projectID = projects.some(
    (project) => String(project.id) === requestedProjectID,
  )
    ? requestedProjectID
    : projects[0]?.id != null
      ? String(projects[0].id)
      : ''

  useEffect(() => {
    if (projectID && projectID !== requestedProjectID) {
      setSearchParams({ project: projectID }, { replace: true })
    }
  }, [projectID, requestedProjectID, setSearchParams])

  const configQuery = useQuery({
    queryKey: ['oauth-google-provider', projectID],
    queryFn: () =>
      controlRequest<GoogleOAuthConfig>(
        `/oauth/projects/${encodeURIComponent(projectID)}/oauth/google`,
      ),
    enabled: Boolean(projectID),
  })

  useEffect(() => {
    if (configQuery.data) setForm(googleConfigToForm(configQuery.data))
  }, [configQuery.data])

  const save = useMutation({
    mutationFn: ({ projectID: targetProjectID, config }: SaveOperation) =>
      controlRequest<GoogleOAuthConfig>(
        `/oauth/projects/${encodeURIComponent(targetProjectID)}/oauth/google`,
        { method: 'PUT', body: JSON.stringify(config) },
      ),
    onSuccess: (config) => {
      sessionStorage.removeItem(pendingKey)
      setForm(googleConfigToForm(config))
      toast.success(tr('Configuração do provedor salva.'))
      void queryClient.invalidateQueries({
        queryKey: ['oauth-google-provider', projectID],
      })
    },
    onError: (error, operation) => {
      if (
        error instanceof ControlApiError &&
        error.code === 'step_up_required'
      ) {
        sessionStorage.setItem(pendingKey, JSON.stringify(operation))
        beginStepUp(
          `/oauth/providers?project=${encodeURIComponent(operation.projectID)}`,
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
    sessionStorage.removeItem(pendingKey)
    try {
      save.mutate(JSON.parse(raw) as SaveOperation)
    } catch {
      // Invalid stale operation was discarded.
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
    <main className="bg-background min-h-dvh px-4 py-7 sm:px-7 lg:px-10">
      <div className="mx-auto w-full max-w-5xl">
        <PageHeader
          description={tr(
            'Configure políticas e limites dos provedores OAuth por projeto.',
          )}
          eyebrow="OAuth"
          icon={Network}
          title={tr('Provedores')}
        />

        <Card className="border-border/70 mt-7 shadow-sm">
          <CardHeader>
            <CardTitle>{tr('Projeto')}</CardTitle>
            <CardDescription>
              {tr('Cada projeto possui sua própria política de provedor.')}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Select
              aria-label={tr('Projeto')}
              disabled={projectsQuery.isLoading || projects.length === 0}
              onChange={(event) =>
                setSearchParams({ project: event.target.value })
              }
              value={projectID}
            >
              {projects.length === 0 && <option value="">—</option>}
              {projects.map((project) => (
                <option key={String(project.id)} value={String(project.id)}>
                  {String(project.name ?? project.id)}
                </option>
              ))}
            </Select>
          </CardContent>
        </Card>

        <Card className="border-border/70 mt-6 shadow-sm">
          <CardHeader>
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <CardTitle>Google OAuth</CardTitle>
                <CardDescription className="mt-2">
                  Authorization Code + PKCE
                </CardDescription>
              </div>
              <StatusBadge tone={form.enabled ? 'positive' : 'neutral'}>
                {tr(form.enabled ? 'Ativo' : 'Inativo')}
              </StatusBadge>
            </div>
          </CardHeader>
          <CardContent className="space-y-6">
            {configQuery.isError && (
              <p className="text-destructive text-sm">
                {tr('Não foi possível carregar a configuração do Google.')}
              </p>
            )}

            <p className="border-border/70 bg-muted/30 text-muted-foreground rounded-xl border p-3 text-sm leading-6">
              {tr(
                'O Client ID e o segredo do Google são credenciais globais do servidor MW OAuth e não são expostos aqui. Esta página controla somente a política de cada projeto.',
              )}
            </p>

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
                onChange={(allowedScopes) =>
                  setForm({ ...form, allowedScopes })
                }
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
                  !projectID ||
                  configQuery.isLoading ||
                  save.isPending ||
                  Boolean(validationError)
                }
                onClick={submit}
              >
                <Save />
                {save.isPending ? tr('Salvando…') : tr('Salvar configuração')}
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>
    </main>
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
