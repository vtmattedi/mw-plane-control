import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AppWindow, KeyRound, Save, ShieldCheck, UserRound } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { useParams } from 'react-router-dom'
import { toast } from 'sonner'
import { useConfirm } from '@/components/confirm-context'
import { PageHeader } from '@/components/control/page-header'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import {
  beginStepUp,
  collectionFrom,
  ControlApiError,
  controlRequest,
} from '@/lib/control-api'
import { usePreferences } from '@/preferences/preferences-context'

type Application = {
  id: string
  name: string
  client_id: string
  application_type: string
  authentication_policy: string
  enabled: boolean
  archived_at?: string
  redirect_uris: string[]
  post_logout_redirect_uris: string[]
  scopes: string[]
  claims: string[]
  audiences: string[]
  created_at: string
  updated_at: string
}

type ApplicationForm = {
  name: string
  authentication_policy: string
  redirect_uris: string
  post_logout_redirect_uris: string
  scopes: string
  claims: string
  audiences: string
}

const pendingApplicationKey = 'mw-identity-application-pending'

function splitValues(value: string) {
  return value
    .split(/[\n,]/)
    .map((item) => item.trim())
    .filter(Boolean)
}

export function IdentityApplicationDetailPage() {
  const { applicationID = '' } = useParams()
  const { tr } = usePreferences()
  const confirm = useConfirm()
  const queryClient = useQueryClient()
  const [form, setForm] = useState<ApplicationForm | null>(null)
  const query = useQuery({
    queryKey: ['identity-application', applicationID],
    queryFn: () =>
      controlRequest<Application>(
        `/identity/applications/${encodeURIComponent(applicationID)}`,
      ),
    enabled: Boolean(applicationID),
  })

  useEffect(() => {
    if (!query.data) return
    const {
      name,
      authentication_policy,
      redirect_uris,
      post_logout_redirect_uris,
      scopes,
      claims,
      audiences,
    } = query.data
    setForm({
      name,
      authentication_policy,
      redirect_uris: (redirect_uris ?? []).join(', '),
      post_logout_redirect_uris: (post_logout_redirect_uris ?? []).join(', '),
      scopes: (scopes ?? []).join(', '),
      claims: (claims ?? []).join(', '),
      audiences: (audiences ?? []).join(', '),
    })
  }, [query.data])

  const mutation = useMutation({
    mutationFn: (operation: {
      path: string
      method: 'PATCH' | 'POST'
      body?: unknown
    }) =>
      controlRequest<Record<string, unknown>>(operation.path, {
        method: operation.method,
        body: JSON.stringify(operation.body ?? {}),
      }),
    onSuccess: (body) => {
      sessionStorage.removeItem(pendingApplicationKey)
      const secret = body.client_secret
      if (typeof secret === 'string') {
        toast.success(tr('Credencial rotacionada.'), {
          description: tr(
            'Este segredo é exibido apenas uma vez. Guarde-o agora.',
          ),
          action: {
            label: tr('Copiar segredo'),
            onClick: () => void navigator.clipboard.writeText(secret),
          },
          duration: 15_000,
        })
      } else {
        toast.success(tr('Configuração salva.'))
      }
      void query.refetch()
      void queryClient.invalidateQueries({
        queryKey: ['control-resource', '/identity/applications'],
      })
    },
    onError: (error, operation) => {
      if (
        error instanceof ControlApiError &&
        error.code === 'step_up_required'
      ) {
        sessionStorage.setItem(pendingApplicationKey, JSON.stringify(operation))
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
    const raw = sessionStorage.getItem(pendingApplicationKey)
    if (!raw) return
    sessionStorage.removeItem(pendingApplicationKey)
    try {
      mutation.mutate(
        JSON.parse(raw) as {
          path: string
          method: 'PATCH' | 'POST'
          body?: unknown
        },
      )
    } catch {
      // Invalid stale operation was already discarded.
    }
    // Resume only once after the Identity callback.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  async function changeState() {
    if (!query.data) return
    const enabled = query.data.enabled
    const accepted = await confirm({
      title: tr(enabled ? 'Desativar aplicação?' : 'Ativar aplicação?'),
      description: tr(
        enabled
          ? 'Novos logins para esta aplicação serão bloqueados.'
          : 'Novos logins para esta aplicação voltarão a ser permitidos.',
      ),
      confirmLabel: tr(enabled ? 'Desativar' : 'Ativar'),
    })
    if (accepted)
      mutation.mutate({
        path: `/identity/applications/${applicationID}/${enabled ? 'disable' : 'enable'}`,
        method: 'POST',
      })
  }

  async function rotateSecret() {
    const accepted = await confirm({
      title: tr('Rotacionar segredo?'),
      description: tr(
        'O segredo atual deixará de funcionar e o novo será exibido apenas uma vez.',
      ),
      confirmLabel: tr('Rotacionar segredo'),
    })
    if (accepted)
      mutation.mutate({
        path: `/identity/applications/${applicationID}/rotate-secret`,
        method: 'POST',
      })
  }

  return (
    <DetailShell
      icon={AppWindow}
      title={query.data?.name ?? tr('Aplicação')}
      description={tr(
        'Edite o cliente OIDC, suas URLs e sua política de autenticação.',
      )}
    >
      <Card className="border-border/70 shadow-sm">
        <CardHeader>
          <CardTitle>{tr('Configuração da aplicação')}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-5">
          {query.isError && (
            <p className="text-destructive text-sm">
              {tr('Não foi possível carregar esta aplicação.')}
            </p>
          )}
          {form && query.data && (
            <>
              <div className="grid gap-4 sm:grid-cols-2">
                <Field label={tr('Nome')}>
                  <Input
                    onChange={(name) => setForm({ ...form, name })}
                    value={form.name}
                  />
                </Field>
                <Field label="Client ID">
                  <Input
                    disabled
                    onChange={() => undefined}
                    value={query.data.client_id}
                  />
                </Field>
                <Field label={tr('Tipo')}>
                  <Input
                    disabled
                    onChange={() => undefined}
                    value={query.data.application_type}
                  />
                </Field>
                <Field label={tr('Política')}>
                  <Select
                    onChange={(event) =>
                      setForm({
                        ...form,
                        authentication_policy: event.target.value,
                      })
                    }
                    value={form.authentication_policy}
                  >
                    <option value="standard">Standard</option>
                    <option value="strong">Strong</option>
                  </Select>
                </Field>
                <Field className="sm:col-span-2" label="Redirect URIs">
                  <Input
                    onChange={(redirect_uris) =>
                      setForm({ ...form, redirect_uris })
                    }
                    value={form.redirect_uris}
                  />
                </Field>
                <Field
                  className="sm:col-span-2"
                  label="Post logout redirect URIs"
                >
                  <Input
                    onChange={(post_logout_redirect_uris) =>
                      setForm({ ...form, post_logout_redirect_uris })
                    }
                    value={form.post_logout_redirect_uris}
                  />
                </Field>
                <Field label="Scopes">
                  <Input
                    onChange={(scopes) => setForm({ ...form, scopes })}
                    value={form.scopes}
                  />
                </Field>
                <Field label="Audiences">
                  <Input
                    onChange={(audiences) => setForm({ ...form, audiences })}
                    value={form.audiences}
                  />
                </Field>
                <Field className="sm:col-span-2" label="Claims">
                  <Input
                    onChange={(claims) => setForm({ ...form, claims })}
                    value={form.claims}
                  />
                </Field>
              </div>
              <div className="flex flex-wrap gap-3">
                <Button
                  disabled={
                    mutation.isPending ||
                    !form.name.trim() ||
                    !form.redirect_uris.trim()
                  }
                  onClick={() =>
                    mutation.mutate({
                      path: `/identity/applications/${applicationID}`,
                      method: 'PATCH',
                      body: {
                        ...form,
                        redirect_uris: splitValues(form.redirect_uris),
                        post_logout_redirect_uris: splitValues(
                          form.post_logout_redirect_uris,
                        ),
                        scopes: splitValues(form.scopes),
                        claims: splitValues(form.claims),
                        audiences: splitValues(form.audiences),
                      },
                    })
                  }
                >
                  <Save /> {tr('Salvar')}
                </Button>
                <Button
                  disabled={mutation.isPending}
                  onClick={() => void changeState()}
                  variant="outline"
                >
                  {tr(query.data.enabled ? 'Desativar' : 'Ativar')}
                </Button>
                {query.data.application_type === 'confidential' && (
                  <Button
                    disabled={mutation.isPending}
                    onClick={() => void rotateSecret()}
                    variant="outline"
                  >
                    <KeyRound /> {tr('Rotacionar segredo')}
                  </Button>
                )}
              </div>
            </>
          )}
        </CardContent>
      </Card>
    </DetailShell>
  )
}

export function IdentityUserDetailPage() {
  const { userID = '' } = useParams()
  const { tr } = usePreferences()
  const confirm = useConfirm()
  const usersQuery = useQuery({
    queryKey: ['control-resource', '/identity/users'],
    queryFn: () => controlRequest<unknown>('/identity/users'),
  })
  const users = useMemo(
    () => collectionFrom(usersQuery.data, 'users'),
    [usersQuery.data],
  )
  const user = users.find((item) => String(item.id) === userID)
  const revoke = useMutation({
    mutationFn: () =>
      controlRequest<void>(
        `/identity/users/${encodeURIComponent(userID)}/revoke-sessions`,
        { method: 'POST', body: '{}' },
      ),
    onSuccess: () => toast.success(tr('Sessões revogadas.')),
    onError: (error) => {
      if (
        error instanceof ControlApiError &&
        error.code === 'step_up_required'
      ) {
        sessionStorage.setItem('mw-identity-user-revoke', userID)
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
    if (sessionStorage.getItem('mw-identity-user-revoke') !== userID) return
    sessionStorage.removeItem('mw-identity-user-revoke')
    revoke.mutate()
    // Resume only once after the Identity callback.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [userID])

  async function revokeSessions() {
    const accepted = await confirm({
      title: tr('Revogar todas as sessões?'),
      description: tr(
        'O usuário precisará entrar novamente em todos os aplicativos.',
      ),
      confirmLabel: tr('Revogar sessões'),
    })
    if (accepted) revoke.mutate()
  }

  return (
    <DetailShell
      icon={UserRound}
      title={String(user?.display_name ?? tr('Usuário'))}
      description={tr(
        'Consulte a identidade e encerre acessos ativos quando necessário.',
      )}
    >
      <Card className="border-border/70 shadow-sm">
        <CardHeader>
          <CardTitle>{tr('Informações da identidade')}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-5">
          {usersQuery.isError && (
            <p className="text-destructive text-sm">
              {tr('Não foi possível carregar este usuário.')}
            </p>
          )}
          {user && (
            <div className="border-border grid gap-4 rounded-xl border p-4 text-sm sm:grid-cols-2">
              <Value label="ID" value={String(user.id)} />
              <Value label={tr('Nome')} value={String(user.display_name)} />
              <Value label={tr('E-mail')} value={String(user.email)} />
              <Value
                label={tr('Status')}
                value={tr(user.enabled === true ? 'Ativo' : 'Inativo')}
              />
              <Value
                label="Superadmin"
                value={user.is_superadmin === true ? tr('Sim') : tr('Não')}
              />
              <Value
                label={tr('Criado em')}
                value={new Date(String(user.created_at)).toLocaleString()}
              />
            </div>
          )}
          <Button
            disabled={!user || revoke.isPending}
            onClick={() => void revokeSessions()}
            variant="destructive"
          >
            <ShieldCheck /> {tr('Revogar sessões')}
          </Button>
        </CardContent>
      </Card>
    </DetailShell>
  )
}

function DetailShell({
  children,
  description,
  icon: Icon,
  title,
}: {
  children: ReactNode
  description: string
  icon: LucideIcon
  title: string
}) {
  return (
    <main className="bg-background min-h-dvh px-4 py-7 sm:px-7 lg:px-10">
      <div className="mx-auto w-full max-w-5xl">
        <PageHeader
          description={description}
          eyebrow="Identity"
          icon={Icon}
          title={title}
        />
        <div className="mt-7">{children}</div>
      </div>
    </main>
  )
}

function Field({
  children,
  className,
  label,
}: {
  children: ReactNode
  className?: string
  label: string
}) {
  return (
    <label className={`space-y-2 text-sm font-medium ${className ?? ''}`}>
      <span>{label}</span>
      {children}
    </label>
  )
}

function Value({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <p className="text-muted-foreground text-xs">{label}</p>
      <p className="mt-1 font-medium break-all">{value}</p>
    </div>
  )
}
