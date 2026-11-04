import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  AlertTriangle,
  Archive,
  AppWindow,
  KeyRound,
  Save,
  ShieldCheck,
  Trash2,
  UserRound,
} from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { toast } from 'sonner'
import { useConfirm } from '@/components/confirm-context'
import { PageHeader } from '@/components/control/page-header'
import { OneTimeCredentialDialog } from '@/components/one-time-credential-dialog'
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
import { splitFormValues, trimPastedValue } from '@/lib/form-values'
import { usePreferences } from '@/preferences/preferences-context'

type Application = {
  id: string
  name: string
  client_id: string
  application_type: string
  authentication_policy: string
  access_policy: string
  access_policy_v1?: boolean
  enabled: boolean
  archived_at?: string
  redirect_uris: string[]
  post_logout_redirect_uris: string[]
  account_management_uri?: string
  account_deletion_uri?: string
  delete_callback_uri?: string
  allow_public_signup: boolean
  internal?: boolean
  needs_delete_callback?: boolean
  scopes: string[]
  claims: string[]
  audiences: string[]
  created_at: string
  updated_at: string
}

type ApplicationForm = {
  name: string
  authentication_policy: string
  access_policy: string
  redirect_uris: string
  post_logout_redirect_uris: string
  account_management_uri: string
  account_deletion_uri: string
  delete_callback_uri: string
  allow_public_signup: boolean
  internal: boolean
  scopes: string
  claims: string
  audiences: string
}

type IdentityUser = {
  id: string
  email: string
  display_name: string
  is_superadmin: boolean
}

type Assignment = {
  application_id: string
  user_id: string
  role: 'user' | 'admin' | 'superadmin'
  access_effect: 'allow' | 'deny'
}

const pendingApplicationKey = 'mw-identity-application-pending'

export function IdentityApplicationDetailPage() {
  const { applicationID = '' } = useParams()
  const { tr, language } = usePreferences()
  const confirm = useConfirm()
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const [form, setForm] = useState<ApplicationForm | null>(null)
  const [exceptionUserID, setExceptionUserID] = useState('')
  const [exceptionRole, setExceptionRole] = useState<Assignment['role']>('user')
  const [exceptionEffect, setExceptionEffect] =
    useState<Assignment['access_effect']>('allow')
  const [revealedCredential, setRevealedCredential] = useState<string | null>(
    null,
  )
  const query = useQuery({
    queryKey: ['identity-application', applicationID],
    queryFn: () =>
      controlRequest<Application>(
        `/identity/applications/${encodeURIComponent(applicationID)}`,
      ),
    enabled: Boolean(applicationID),
  })
  const usersQuery = useQuery({
    queryKey: ['identity-users-for-access'],
    queryFn: async () =>
      collectionFrom(
        await controlRequest<unknown>('/identity/users'),
        'users',
      ) as IdentityUser[],
  })
  const assignmentsQuery = useQuery({
    queryKey: ['identity-application-assignments', applicationID],
    queryFn: async () =>
      collectionFrom(
        await controlRequest<unknown>(
          `/identity/applications/${encodeURIComponent(applicationID)}/assignments`,
        ),
        'assignments',
      ) as Assignment[],
    enabled: Boolean(applicationID),
  })

  useEffect(() => {
    if (!query.data) return
    const {
      name,
      authentication_policy,
      access_policy,
      redirect_uris,
      post_logout_redirect_uris,
      delete_callback_uri,
      allow_public_signup,
      internal,
      scopes,
      claims,
      audiences,
    } = query.data
    setForm({
      name,
      authentication_policy,
      access_policy: access_policy || 'default_deny',
      redirect_uris: (redirect_uris ?? []).join(', '),
      post_logout_redirect_uris: (post_logout_redirect_uris ?? []).join(', '),
      delete_callback_uri: delete_callback_uri ?? '',
      account_management_uri: query.data.account_management_uri ?? '',
      account_deletion_uri: query.data.account_deletion_uri ?? '',
      allow_public_signup: allow_public_signup === true,
      internal: internal === true,
      scopes: (scopes ?? []).join(', '),
      claims: (claims ?? []).join(', '),
      audiences: (audiences ?? []).join(', '),
    })
  }, [query.data])

  const accessMutation = useMutation({
    mutationFn: (operation: {
      userID: string
      method: 'PUT' | 'DELETE'
      role?: Assignment['role']
      accessEffect?: Assignment['access_effect']
    }) =>
      controlRequest(
        `/identity/applications/${encodeURIComponent(applicationID)}/assignments/${encodeURIComponent(operation.userID)}`,
        {
          method: operation.method,
          body:
            operation.method === 'PUT'
              ? JSON.stringify({
                  role: operation.role ?? 'user',
                  access_effect: operation.accessEffect ?? 'allow',
                })
              : undefined,
        },
      ),
    onSuccess: () => {
      toast.success(tr('Exceção de acesso atualizada.'))
      setExceptionUserID('')
      void assignmentsQuery.refetch()
    },
    onError: (error) =>
      toast.error(
        error instanceof Error
          ? error.message
          : tr('Não foi possível atualizar a exceção de acesso.'),
      ),
  })

  const users = useMemo(() => usersQuery.data ?? [], [usersQuery.data])
  const assignments = assignmentsQuery.data ?? []
  const usersByID = useMemo(
    () => new Map(users.map((user) => [user.id, user])),
    [users],
  )
  const selectedExceptionUser = usersByID.get(exceptionUserID)
  const administrationAudience =
    query.data?.audiences?.includes('mw-identity-admin') === true
  const selectedAssignment = assignments.find(
    (assignment) => assignment.user_id === exceptionUserID,
  )
  const selectedProtectedAdmin =
    administrationAudience &&
    (selectedExceptionUser?.is_superadmin ||
      exceptionRole !== 'user' ||
      selectedAssignment?.role === 'admin' ||
      selectedAssignment?.role === 'superadmin')

  function saveAccessException() {
    if (!exceptionUserID) return
    accessMutation.mutate({
      userID: exceptionUserID,
      method: 'PUT',
      role: exceptionRole,
      accessEffect: selectedProtectedAdmin ? 'allow' : exceptionEffect,
    })
  }

  const mutation = useMutation({
    mutationFn: (operation: {
      path: string
      method: 'DELETE' | 'PATCH' | 'POST'
      body?: unknown
    }) =>
      controlRequest<Record<string, unknown>>(operation.path, {
        method: operation.method,
        body: JSON.stringify(operation.body ?? {}),
      }),
    onSuccess: (body, operation) => {
      sessionStorage.removeItem(pendingApplicationKey)
      if (operation.method === 'DELETE') {
        toast.success(tr('Aplicação excluída permanentemente.'))
        void queryClient.invalidateQueries({
          queryKey: ['control-resource', '/identity/applications'],
        })
        navigate('/identity/applications')
        return
      }
      const secret = body.client_secret
      if (typeof secret === 'string' && secret.length > 0) {
        setRevealedCredential(secret)
        toast.success(tr('Credencial rotacionada.'))
      } else if (operation.path.endsWith('/archive')) {
        toast.success(tr('Aplicação arquivada.'))
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
          method: 'DELETE' | 'PATCH' | 'POST'
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

  async function archiveApplication() {
    const accepted = await confirm({
      title: tr('Arquivar aplicação?'),
      description: tr(
        'Novos logins serão bloqueados e a aplicação não poderá ser reativada. O Client ID continuará reservado até uma exclusão permanente.',
      ),
      confirmLabel: tr('Arquivar aplicação'),
    })
    if (accepted)
      mutation.mutate({
        path: `/identity/applications/${applicationID}/archive`,
        method: 'POST',
      })
  }

  async function deleteApplication() {
    if (!query.data) return
    const accepted = await confirm({
      title: tr('Excluir aplicação permanentemente?'),
      description: tr(
        `Esta ação não pode ser desfeita. O Client ID ${query.data.client_id} será liberado. A exclusão será recusada se existirem usuários associados, códigos de autorização ou tokens emitidos.`,
      ),
      confirmLabel: tr('Excluir permanentemente'),
    })
    if (accepted)
      mutation.mutate({
        path: `/identity/applications/${applicationID}`,
        method: 'DELETE',
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
              {query.data.needs_delete_callback && (
                <div
                  className="border-warning/20 bg-warning/10 text-warning flex gap-2 rounded-xl border p-3 text-sm"
                  role="status"
                >
                  <AlertTriangle className="mt-0.5 size-4 shrink-0" />
                  <span>
                    {language === 'en'
                      ? 'This legacy application needs a whole-identity deletion callback before it can be edited.'
                      : 'Esta aplicação precisa de um callback de exclusão da identidade MW antes de ser editada.'}
                  </span>
                </div>
              )}
              <fieldset
                className="contents"
                disabled={
                  Boolean(query.data.archived_at) || query.data.internal
                }
              >
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

                  {query.data.access_policy_v1 === false && (
                    <p role="status">
                      {language === 'en'
                        ? 'Legacy access rules are active. An operator must review and migrate this client before Public/Private rules are enforced.'
                        : 'Regras antigas de acesso ativas. Um operador precisa revisar e migrar este cliente antes de aplicar as regras de acesso.'}
                    </p>
                  )}
                  {query.data.internal && (
                    <p role="status">
                      {language === 'en'
                        ? 'Infrastructure configuration is managed through the operator workflow.'
                        : 'A configuração de infraestrutura é gerenciada pelo operador.'}
                    </p>
                  )}
                  <Field label={tr('Política de acesso')}>
                    <Select
                      onChange={(event) =>
                        setForm({ ...form, access_policy: event.target.value })
                      }
                      value={form.access_policy}
                    >
                      <option value="default_deny">
                        {language === 'en' ? 'Private' : 'Privado'}
                      </option>
                      <option value="default_allow">
                        {language === 'en' ? 'Public' : 'Público'}
                      </option>
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

                  <Field
                    className="sm:col-span-2"
                    label={tr(
                      form.internal
                        ? 'Callback de exclusão da conta MW inteira (opcional)'
                        : 'Callback de exclusão da conta MW inteira (obrigatório)',
                    )}
                  >
                    <Input
                      onChange={(delete_callback_uri) =>
                        setForm({ ...form, delete_callback_uri })
                      }
                      placeholder="https://app.example.com/internal/identity/account-deleted"
                      value={form.delete_callback_uri}
                    />
                  </Field>

                  <Field
                    label={
                      language === 'en'
                        ? 'Manage account URL (optional)'
                        : 'URL de gestão da conta (opcional)'
                    }
                  >
                    <Input
                      value={form.account_management_uri}
                      onChange={(account_management_uri) =>
                        setForm({ ...form, account_management_uri })
                      }
                    />
                  </Field>
                  <Field
                    label={
                      language === 'en'
                        ? 'Delete account URL (optional)'
                        : 'URL de exclusão da conta (opcional)'
                    }
                  >
                    <Input
                      value={form.account_deletion_uri}
                      onChange={(account_deletion_uri) =>
                        setForm({ ...form, account_deletion_uri })
                      }
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
              </fieldset>
              {!query.data.archived_at && (
                <div className="mt-6 flex flex-wrap gap-3">
                  <Button
                    disabled={
                      mutation.isPending ||
                      query.data.internal ||
                      !form.name.trim() ||
                      !form.redirect_uris.trim()
                    }
                    onClick={() =>
                      mutation.mutate({
                        path: `/identity/applications/${applicationID}`,
                        method: 'PATCH',
                        body: {
                          ...form,
                          redirect_uris: splitFormValues(form.redirect_uris),
                          post_logout_redirect_uris: splitFormValues(
                            form.post_logout_redirect_uris,
                          ),
                          delete_callback_uri: trimPastedValue(
                            form.delete_callback_uri,
                          ),
                          scopes: splitFormValues(form.scopes),
                          claims: splitFormValues(form.claims),
                          audiences: splitFormValues(form.audiences),
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
              )}
              {query.data.application_type === 'public' && (
                <p className="border-border/70 bg-muted/40 text-muted-foreground rounded-xl border p-3 text-sm leading-6">
                  {tr(
                    'Cliente público: use o Client ID com Authorization Code + PKCE. Este tipo de cliente não possui segredo e não requer rotação de credenciais.',
                  )}
                </p>
              )}
            </>
          )}
        </CardContent>
      </Card>
      {query.data && (
        <Card className="border-border/70 mt-6 shadow-sm">
          <CardHeader>
            <CardTitle>{tr('Exceções de acesso')}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-5">
            <p className="text-muted-foreground text-sm leading-6">
              {tr(
                administrationAudience
                  ? 'As exceções substituem a política padrão. Superadmins e administradores não podem ser bloqueados.'
                  : 'As exceções substituem a política padrão, inclusive para superadmins e administradores.',
              )}
            </p>
            <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_10rem_10rem_auto]">
              <Select
                disabled={accessMutation.isPending}
                onChange={(event) => setExceptionUserID(event.target.value)}
                value={exceptionUserID}
              >
                <option value="">{tr('Selecione um usuário')}</option>
                {users.map((user) => (
                  <option key={user.id} value={user.id}>
                    {user.display_name || user.email} ({user.email})
                    {user.is_superadmin ? ' — Superadmin' : ''}
                  </option>
                ))}
              </Select>
              <Select
                disabled={accessMutation.isPending}
                onChange={(event) => {
                  const role = event.target.value as Assignment['role']
                  setExceptionRole(role)
                  if (administrationAudience && role !== 'user')
                    setExceptionEffect('allow')
                }}
                value={exceptionRole}
              >
                <option value="user">User</option>
                <option value="admin">Admin</option>
                <option value="superadmin">Superadmin</option>
              </Select>
              <Select
                disabled={accessMutation.isPending || selectedProtectedAdmin}
                onChange={(event) =>
                  setExceptionEffect(
                    event.target.value as Assignment['access_effect'],
                  )
                }
                value={selectedProtectedAdmin ? 'allow' : exceptionEffect}
              >
                <option value="allow">{tr('Permitir')}</option>
                <option value="deny">{tr('Bloquear')}</option>
              </Select>
              <Button
                disabled={!exceptionUserID || accessMutation.isPending}
                onClick={saveAccessException}
              >
                {tr('Salvar exceção')}
              </Button>
            </div>
            <div className="space-y-2">
              {assignments.map((assignment) => {
                const user = usersByID.get(assignment.user_id)
                const protectedAdmin =
                  administrationAudience &&
                  (user?.is_superadmin ||
                    assignment.role === 'admin' ||
                    assignment.role === 'superadmin')
                return (
                  <div
                    className="border-border flex flex-wrap items-center gap-3 rounded-xl border p-3 text-sm"
                    key={assignment.user_id}
                  >
                    <div className="min-w-0 flex-1">
                      <p className="font-medium break-all">
                        {user?.display_name ||
                          user?.email ||
                          assignment.user_id}
                      </p>
                      <p className="text-muted-foreground break-all">
                        {user?.email ?? assignment.user_id} · {assignment.role}{' '}
                        ·{' '}
                        {tr(
                          assignment.access_effect === 'allow'
                            ? 'Permitido'
                            : 'Bloqueado',
                        )}
                      </p>
                    </div>
                    <Button
                      disabled={accessMutation.isPending}
                      onClick={() =>
                        accessMutation.mutate({
                          userID: assignment.user_id,
                          method: 'PUT',
                          role: assignment.role,
                          accessEffect: 'allow',
                        })
                      }
                      variant="outline"
                    >
                      {tr('Permitir')}
                    </Button>
                    <Button
                      disabled={accessMutation.isPending || protectedAdmin}
                      onClick={() =>
                        accessMutation.mutate({
                          userID: assignment.user_id,
                          method: 'PUT',
                          role: assignment.role,
                          accessEffect: 'deny',
                        })
                      }
                      variant="outline"
                    >
                      {tr('Bloquear')}
                    </Button>
                    <Button
                      disabled={accessMutation.isPending}
                      onClick={() =>
                        accessMutation.mutate({
                          userID: assignment.user_id,
                          method: 'DELETE',
                        })
                      }
                      variant="outline"
                    >
                      {tr('Usar padrão')}
                    </Button>
                  </div>
                )
              })}
              {!assignmentsQuery.isPending && assignments.length === 0 && (
                <p className="text-muted-foreground text-sm">
                  {tr('Nenhuma exceção configurada.')}
                </p>
              )}
            </div>
          </CardContent>
        </Card>
      )}
      {query.data && (
        <Card className="border-destructive/25 mt-6 shadow-sm">
          <CardHeader>
            <CardTitle className="text-destructive flex items-center gap-2">
              {query.data.archived_at ? <Trash2 /> : <Archive />}
              {tr('Zona de perigo')}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <p className="text-muted-foreground text-sm leading-6">
              {tr(
                query.data.archived_at
                  ? 'A exclusão permanente remove o cadastro e libera o Client ID. Ela só é permitida quando não existem usuários associados, códigos de autorização ou tokens emitidos.'
                  : 'Arquivar bloqueia logins e alterações, preserva o histórico e mantém o Client ID reservado.',
              )}
            </p>
            <Button
              disabled={mutation.isPending}
              variant="destructive"
              onClick={() =>
                void (query.data.archived_at
                  ? deleteApplication()
                  : archiveApplication())
              }
            >
              {query.data.archived_at ? <Trash2 /> : <Archive />}
              {tr(
                query.data.archived_at
                  ? 'Excluir permanentemente'
                  : 'Arquivar aplicação',
              )}
            </Button>
          </CardContent>
        </Card>
      )}
      <OneTimeCredentialDialog
        credential={revealedCredential}
        onClose={() => setRevealedCredential(null)}
      />
    </DetailShell>
  )
}

export function IdentityUserDetailPage() {
  const { userID = '' } = useParams()
  const { tr } = usePreferences()
  const confirm = useConfirm()
  const navigate = useNavigate()
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
  const remove = useMutation({
    mutationFn: () =>
      controlRequest<{ deleted: boolean }>(
        `/identity/users/${encodeURIComponent(userID)}`,
        { method: 'DELETE' },
      ),
    onSuccess: (result) => {
      sessionStorage.removeItem('mw-identity-user-delete')
      toast.success(
        tr(
          result.deleted
            ? 'Conta excluída.'
            : 'Conta desativada; callbacks pendentes podem ser tentados novamente.',
        ),
      )
      navigate('/identity/users', { replace: true })
    },
    onError: (error) => {
      if (
        error instanceof ControlApiError &&
        error.code === 'step_up_required'
      ) {
        sessionStorage.setItem('mw-identity-user-delete', userID)
        beginStepUp(window.location.pathname)
        return
      }
      toast.error(
        error instanceof Error
          ? error.message
          : tr('Não foi possível excluir a conta.'),
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

  useEffect(() => {
    if (sessionStorage.getItem('mw-identity-user-delete') !== userID) return
    sessionStorage.removeItem('mw-identity-user-delete')
    remove.mutate()
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

  async function deleteUser() {
    const accepted = await confirm({
      title: tr('Excluir conta?'),
      description: tr(
        'O acesso será revogado imediatamente e a exclusão será enviada aos aplicativos conectados.',
      ),
      confirmLabel: tr('Excluir conta'),
    })
    if (accepted) remove.mutate()
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
                label={tr('Tipo de conta')}
                value={tr(
                  user.account_type === 'public'
                    ? 'Conta pública'
                    : 'Conta corporativa',
                )}
              />
              <Value
                label={tr('Criado em')}
                value={new Date(String(user.created_at)).toLocaleString()}
              />
            </div>
          )}
          <div className="flex flex-wrap gap-3">
            <Button
              disabled={!user || revoke.isPending}
              onClick={() => void revokeSessions()}
              variant="destructive"
            >
              <ShieldCheck /> {tr('Revogar sessões')}
            </Button>
            <Button
              disabled={
                !user || user.is_superadmin === true || remove.isPending
              }
              onClick={() => void deleteUser()}
              variant="destructive"
            >
              <Trash2 /> {tr('Excluir conta')}
            </Button>
          </div>
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
