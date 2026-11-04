import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  AlertTriangle,
  Database,
  Gauge,
  KeyRound,
  RefreshCw,
  Search,
  ShieldCheck,
} from 'lucide-react'
import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { toast } from 'sonner'
import type { LucideIcon } from 'lucide-react'
import { MetricCard } from '@/components/control/metric-card'
import { PageHeader } from '@/components/control/page-header'
import {
  ResourceTable,
  type ResourceColumn,
} from '@/components/control/resource-table'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { useConfirm } from '@/components/confirm-context'
import {
  OneTimeCredentialDialog,
  type OneTimeCredential,
} from '@/components/one-time-credential-dialog'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import {
  beginStepUp,
  collectionFrom,
  ControlApiError,
  controlRequest,
} from '@/lib/control-api'
import { splitFormValues, trimPastedValue } from '@/lib/form-values'
import styles from './resource-page.module.scss'
import { usePreferences } from '@/preferences/preferences-context'

export type ResourcePageConfig = {
  module: string
  title: string
  description: string
  icon: LucideIcon
  endpoint?: string
  collectionKey?: string
  columns: ResourceColumn[]
  action?: string
  emptyTitle: string
  emptyDescription: string
  securityNote?: string
  projectActions?: 'oauth' | 'mail'
  createKind?: 'identity-application' | 'identity-user'
  monthlyMailLimit?: number
  usageEndpoint?: string
  rowHref?: (row: Record<string, unknown>) => string | undefined
}

type PendingMutation = {
  path: string
  method: 'POST' | 'PATCH'
  body: Record<string, unknown>
  returnTo: string
}

const pendingMutationKey = 'mw-control-pending-mutation'
const identityUserDraftKey = 'mw-identity-user-create-draft'

export function ResourcePage({ config }: { config: ResourcePageConfig }) {
  const { language, tr } = usePreferences()
  const confirm = useConfirm()
  const [search, setSearch] = useState('')
  const [createOpen, setCreateOpen] = useState(false)
  const [projectName, setProjectName] = useState('')
  const [revealedCredentials, setRevealedCredentials] = useState<
    OneTimeCredential[] | null
  >(null)
  const [createFields, setCreateFields] = useState({
    email: '',
    displayName: '',
    password: '',
    clientID: '',
    applicationType: 'confidential',
    accessPolicy: 'default_deny',
    redirectURIs: '',
    postLogoutRedirectURIs: '',
    deleteCallbackURI: '',
    accountManagementURI: '',
    accountDeletionURI: '',
    scopes: 'openid, profile, email',
    claims: '',
    audiences: '',
  })
  const queryClient = useQueryClient()
  const query = useQuery({
    queryKey: ['control-resource', config.endpoint],
    queryFn: () => controlRequest<unknown>(config.endpoint!),
    enabled: Boolean(config.endpoint),
    retry: 1,
  })
  const usageQuery = useQuery({
    queryKey: ['control-resource-usage', config.usageEndpoint],
    queryFn: () => controlRequest<unknown>(config.usageEndpoint!),
    enabled: Boolean(config.usageEndpoint && config.monthlyMailLimit),
    retry: 1,
  })
  const monthlyMailUsage = useMemo(
    () =>
      collectionFrom(usageQuery.data, 'projects').reduce((total, project) => {
        const count = Number(project.email_count)
        return total + (Number.isFinite(count) ? count : 0)
      }, 0),
    [usageQuery.data],
  )
  const rows = useMemo(() => {
    const source = config.endpoint
      ? collectionFrom(query.data, config.collectionKey ?? 'items')
      : []
    const needle = search.trim().toLocaleLowerCase('pt-BR')
    if (!needle) return source
    return source.filter((row) =>
      Object.values(row).some((value) =>
        String(value ?? '')
          .toLocaleLowerCase('pt-BR')
          .includes(needle),
      ),
    )
  }, [config.collectionKey, config.endpoint, query.data, search])

  const active = rows.filter(
    (row) =>
      row.enabled === true || row.status === 'ok' || row.status === 'success',
  ).length

  const mutation = useMutation({
    mutationFn: (operation: PendingMutation) =>
      controlRequest<unknown>(operation.path, {
        method: operation.method,
        body: JSON.stringify(operation.body),
      }),
    onSuccess: (body) => {
      sessionStorage.removeItem(pendingMutationKey)
      const record = (body ?? {}) as Record<string, unknown>
      const credentials: OneTimeCredential[] = []
      if (typeof record.api_key === 'string' && record.api_key.length > 0) {
        credentials.push({
          label:
            config.projectActions === 'oauth'
              ? 'MW_OAUTH_PROJECT_KEY'
              : config.projectActions === 'mail'
                ? 'MW_EMAIL_API_KEY'
                : 'API key',
          value: record.api_key,
        })
      }
      if (
        typeof record.callback_secret === 'string' &&
        record.callback_secret.length > 0
      ) {
        credentials.push({
          label: 'MW_OAUTH_CALLBACK_SECRET',
          value: record.callback_secret,
        })
      }
      if (
        typeof record.client_secret === 'string' &&
        record.client_secret.length > 0
      ) {
        credentials.push({
          label: 'Client secret',
          value: record.client_secret,
        })
      }
      if (credentials.length > 0) {
        setRevealedCredentials(credentials)
      }
      toast.success(tr('Operação concluída com sucesso.'))
      setCreateOpen(false)
      setProjectName('')
      setCreateFields((current) => ({
        ...current,
        email: '',
        displayName: '',
        password: '',
        clientID: '',
        redirectURIs: '',
        postLogoutRedirectURIs: '',
        deleteCallbackURI: '',
        accountManagementURI: '',
        accountDeletionURI: '',
        claims: '',
        audiences: '',
      }))
      void queryClient.invalidateQueries({ queryKey: ['control-resource'] })
    },
    onError: (error, operation) => {
      if (
        error instanceof ControlApiError &&
        error.code === 'step_up_required'
      ) {
        if (operation.path === '/identity/users') {
          sessionStorage.setItem(
            identityUserDraftKey,
            JSON.stringify({
              email: operation.body.email,
              displayName: operation.body.display_name,
            }),
          )
          beginStepUp(operation.returnTo)
          return
        }
        sessionStorage.setItem(pendingMutationKey, JSON.stringify(operation))
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
    const userDraft = sessionStorage.getItem(identityUserDraftKey)
    if (config.createKind === 'identity-user' && userDraft) {
      sessionStorage.removeItem(identityUserDraftKey)
      try {
        const draft = JSON.parse(userDraft) as {
          email?: string
          displayName?: string
        }
        setCreateFields((current) => ({
          ...current,
          email: draft.email ?? '',
          displayName: draft.displayName ?? '',
          password: '',
        }))
        setCreateOpen(true)
        toast.info(tr('Autenticação forte confirmada.'), {
          description: tr(
            'Digite novamente a senha inicial para criar o usuário.',
          ),
        })
      } catch {
        // Invalid stale draft was already discarded.
      }
    }
    const raw = sessionStorage.getItem(pendingMutationKey)
    if (!raw) return
    try {
      const operation = JSON.parse(raw) as PendingMutation
      if (operation.returnTo === window.location.pathname) {
        // Claim before executing so React StrictMode cannot replay a
        // credential rotation during its development-only effect check.
        sessionStorage.removeItem(pendingMutationKey)
        mutation.mutate(operation)
      }
    } catch {
      sessionStorage.removeItem(pendingMutationKey)
    }
    // Resume only once after an Identity step-up callback.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  function runMutation(
    path: string,
    method: PendingMutation['method'],
    body: Record<string, unknown> = {},
  ) {
    mutation.mutate({ path, method, body, returnTo: window.location.pathname })
  }

  function primaryAction() {
    if (!config.projectActions && !config.createKind) {
      document.getElementById('resource-search')?.focus()
      return
    }
    setCreateOpen(true)
  }

  function submitCreate() {
    if (config.createKind === 'identity-user') {
      runMutation('/identity/users', 'POST', {
        email: createFields.email.trim(),
        display_name: createFields.displayName.trim(),
        password: createFields.password,
      })
      return
    }
    if (config.createKind === 'identity-application') {
      runMutation('/identity/applications', 'POST', {
        name: projectName.trim(),
        client_id: createFields.clientID.trim(),
        application_type: createFields.applicationType,

        access_policy: createFields.accessPolicy,
        redirect_uris: splitFormValues(createFields.redirectURIs),
        post_logout_redirect_uris: splitFormValues(
          createFields.postLogoutRedirectURIs,
        ),
        delete_callback_uri: trimPastedValue(createFields.deleteCallbackURI),
        account_management_uri: trimPastedValue(
          createFields.accountManagementURI,
        ),
        account_deletion_uri: trimPastedValue(createFields.accountDeletionURI),

        scopes: splitFormValues(createFields.scopes),
        claims: splitFormValues(createFields.claims),
        audiences: splitFormValues(createFields.audiences),
      })
      return
    }
    if (config.projectActions) {
      runMutation(`/${config.projectActions}/projects`, 'POST', {
        name: projectName.trim(),
      })
    }
  }

  return (
    <main className={styles.page}>
      <div className={styles.content}>
        <PageHeader
          action={config.action ? tr(config.action) : undefined}
          description={tr(config.description)}
          eyebrow={config.module}
          icon={config.icon}
          onAction={primaryAction}
          title={tr(config.title)}
        />

        <section
          className="mt-7 grid gap-4 sm:grid-cols-2 xl:grid-cols-4"
          aria-label={tr('Resumo')}
        >
          {config.monthlyMailLimit ? (
            <MetricCard
              icon={Gauge}
              label={tr('Uso mensal')}
              note={tr('Cota mensal de mensagens')}
              value={
                usageQuery.isLoading
                  ? '…'
                  : `${usageQuery.isError ? '—' : monthlyMailUsage.toLocaleString(language === 'pt' ? 'pt-BR' : 'en-US')} / ${config.monthlyMailLimit.toLocaleString(language === 'pt' ? 'pt-BR' : 'en-US')}`
              }
              tone="accent"
            />
          ) : (
            <MetricCard
              icon={Database}
              label={tr('Registros')}
              note={tr('Disponíveis neste ambiente')}
              value={query.isLoading ? '…' : rows.length}
              tone="accent"
            />
          )}
          <MetricCard
            icon={ShieldCheck}
            label={tr('Ativos')}
            note={tr('Com operação permitida')}
            value={query.isLoading ? '…' : active || '—'}
          />
          <MetricCard
            icon={KeyRound}
            label={tr('Proteção')}
            note={tr('Sessão BFF + escopo delegado')}
            value="OIDC"
          />
          <MetricCard
            icon={RefreshCw}
            label={tr('Sincronização')}
            note={tr('Dados lidos diretamente do serviço')}
            value={tr(query.isFetching ? 'Agora' : 'Em dia')}
          />
        </section>

        <Card className="border-border/70 mt-6 overflow-hidden shadow-sm">
          <div className="border-border/70 flex flex-col gap-4 border-b p-4 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h2 className="text-sm font-semibold">{tr(config.title)}</h2>
              <p className="text-muted-foreground mt-0.5 text-xs">
                {tr('Gerencie e acompanhe este recurso pelo control plane.')}
              </p>
            </div>
            <div className={styles.toolbar}>
              <label className={styles.search}>
                <span className="sr-only">{tr('Buscar')}</span>
                <Search aria-hidden="true" />
                <input
                  id="resource-search"
                  onChange={(event) => setSearch(event.target.value)}
                  placeholder={tr('Buscar registros…')}
                  type="search"
                  value={search}
                />
              </label>
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
          </div>

          {query.isError && (
            <div className={styles.error} role="status">
              <AlertTriangle className="text-warning mt-0.5 size-4 shrink-0" />
              <span>
                {tr(
                  'O módulo ainda não respondeu. A navegação permanece disponível e os dados serão exibidos assim que o serviço estiver conectado.',
                )}
              </span>
            </div>
          )}

          <ResourceTable
            columns={config.columns.map((column) => ({
              ...column,
              label: tr(column.label),
            }))}
            emptyDescription={tr(config.emptyDescription)}
            emptyTitle={
              query.isLoading ? tr('Carregando dados…') : tr(config.emptyTitle)
            }
            rows={rows}
            getRowHref={config.rowHref}
            renderActions={
              config.projectActions
                ? (row) => {
                    const id = row.id
                    if (id === undefined || id === null) return null
                    const base = `/${config.projectActions}/projects/${String(id)}`
                    return (
                      <div className="flex justify-end gap-2">
                        <Button
                          disabled={mutation.isPending}
                          onClick={async () => {
                            const enabled = row.enabled === true
                            const accepted = await confirm({
                              title: tr(
                                enabled
                                  ? 'Desativar projeto?'
                                  : 'Ativar projeto?',
                              ),
                              description: tr(
                                enabled
                                  ? 'As integrações deste projeto deixarão de funcionar até ele ser reativado.'
                                  : 'As integrações deste projeto voltarão a funcionar.',
                              ),
                              confirmLabel: tr(
                                enabled ? 'Desativar' : 'Ativar',
                              ),
                            })
                            if (accepted) {
                              runMutation(base, 'PATCH', { enabled: !enabled })
                            }
                          }}
                          size="sm"
                          variant="ghost"
                        >
                          {tr(row.enabled === true ? 'Desativar' : 'Ativar')}
                        </Button>
                        <Button
                          disabled={mutation.isPending}
                          onClick={async () => {
                            const accepted = await confirm({
                              title: tr('Rotacionar chave?'),
                              description: tr(
                                'A chave atual deixará de funcionar e a nova será exibida apenas uma vez.',
                              ),
                              confirmLabel: tr('Rotacionar chave'),
                            })
                            if (accepted) {
                              runMutation(`${base}/rotate-key`, 'POST')
                            }
                          }}
                          size="sm"
                          variant="outline"
                        >
                          {tr('Rotacionar chave')}
                        </Button>
                      </div>
                    )
                  }
                : undefined
            }
          />
        </Card>

        {config.securityNote && (
          <aside className="text-muted-foreground mt-5 flex items-start gap-2 text-xs leading-5">
            <ShieldCheck className="text-primary mt-0.5 size-4 shrink-0" />
            {tr(config.securityNote)}
          </aside>
        )}

        <Dialog open={createOpen} onOpenChange={setCreateOpen}>
          <DialogContent
            className={
              config.createKind === 'identity-application'
                ? 'max-h-[90dvh] overflow-y-auto sm:max-w-2xl'
                : undefined
            }
            closeDisabled={mutation.isPending}
          >
            <DialogHeader>
              <DialogTitle>{tr(config.action ?? 'Novo projeto')}</DialogTitle>
              <DialogDescription>
                {tr(
                  config.createKind === 'identity-user'
                    ? 'Crie a identidade com uma senha inicial segura.'
                    : config.createKind === 'identity-application'
                      ? 'Registre o cliente OIDC e seus endereços autorizados.'
                      : 'Informe um nome para identificar o projeto.',
                )}
              </DialogDescription>
            </DialogHeader>
            {config.createKind === 'identity-user' ? (
              <div className="grid gap-4 sm:grid-cols-2">
                <CreateField label={tr('Nome')}>
                  <Input
                    autoFocus
                    disabled={mutation.isPending}
                    onChange={(displayName) =>
                      setCreateFields((value) => ({ ...value, displayName }))
                    }
                    value={createFields.displayName}
                  />
                </CreateField>
                <CreateField label={tr('E-mail')}>
                  <Input
                    disabled={mutation.isPending}
                    onChange={(email) =>
                      setCreateFields((value) => ({ ...value, email }))
                    }
                    type="email"
                    value={createFields.email}
                  />
                </CreateField>
                <CreateField
                  className="sm:col-span-2"
                  label={tr('Senha inicial')}
                >
                  <Input
                    disabled={mutation.isPending}
                    maxLength={256}
                    minLength={12}
                    onChange={(password) =>
                      setCreateFields((value) => ({ ...value, password }))
                    }
                    type="password"
                    value={createFields.password}
                  />
                  <span className="text-muted-foreground text-xs">
                    {tr(
                      'Use pelo menos 12 caracteres. O usuário poderá configurar TOTP após entrar.',
                    )}
                  </span>
                </CreateField>
              </div>
            ) : config.createKind === 'identity-application' ? (
              <div className="grid gap-4 sm:grid-cols-2">
                <CreateField label={tr('Nome')}>
                  <Input
                    autoFocus
                    disabled={mutation.isPending}
                    onChange={setProjectName}
                    value={projectName}
                  />
                </CreateField>
                <CreateField label={tr('Client ID')}>
                  <Input
                    disabled={mutation.isPending}
                    onChange={(clientID) =>
                      setCreateFields((value) => ({ ...value, clientID }))
                    }
                    value={createFields.clientID}
                  />
                </CreateField>
                <CreateField
                  label={
                    language === 'en'
                      ? 'OAuth client type'
                      : 'Tipo de cliente OAuth'
                  }
                >
                  <Select
                    disabled={mutation.isPending}
                    onChange={(event) =>
                      setCreateFields((value) => ({
                        ...value,
                        applicationType: event.target.value,
                      }))
                    }
                    value={createFields.applicationType}
                  >
                    <option value="confidential">{tr('Confidencial')}</option>
                    <option value="public">{tr('Pública')}</option>
                  </Select>
                </CreateField>

                <CreateField label={tr('Política de acesso')}>
                  <Select
                    disabled={mutation.isPending}
                    onChange={(event) =>
                      setCreateFields((value) => ({
                        ...value,
                        accessPolicy: event.target.value,
                      }))
                    }
                    value={createFields.accessPolicy}
                  >
                    <option value="default_deny">
                      {language === 'en' ? 'Private' : 'Privado'}
                    </option>
                    <option value="default_allow">
                      {language === 'en' ? 'Public' : 'Público'}
                    </option>
                  </Select>
                </CreateField>
                <CreateField
                  className="sm:col-span-2"
                  label={tr('URIs de redirecionamento')}
                >
                  <Input
                    disabled={mutation.isPending}
                    onChange={(redirectURIs) =>
                      setCreateFields((value) => ({ ...value, redirectURIs }))
                    }
                    placeholder="https://app.example.com/callback"
                    value={createFields.redirectURIs}
                  />
                </CreateField>
                <CreateField
                  className="sm:col-span-2"
                  label={tr('URIs de redirecionamento após logout (opcional)')}
                >
                  <Input
                    disabled={mutation.isPending}
                    onChange={(postLogoutRedirectURIs) =>
                      setCreateFields((value) => ({
                        ...value,
                        postLogoutRedirectURIs,
                      }))
                    }
                    placeholder="https://app.example.com"
                    value={createFields.postLogoutRedirectURIs}
                  />
                </CreateField>

                <CreateField
                  className="sm:col-span-2"
                  label={tr(
                    'Callback de exclusão da conta MW inteira (obrigatório)',
                  )}
                >
                  <Input
                    disabled={mutation.isPending}
                    onChange={(deleteCallbackURI) =>
                      setCreateFields((value) => ({
                        ...value,
                        deleteCallbackURI,
                      }))
                    }
                    placeholder="https://app.example.com/internal/identity/account-deleted"
                    value={createFields.deleteCallbackURI}
                  />
                </CreateField>

                <CreateField
                  label={
                    language === 'en'
                      ? 'Manage account URL (optional)'
                      : 'URL de gestão da conta (opcional)'
                  }
                >
                  <Input
                    disabled={mutation.isPending}
                    value={createFields.accountManagementURI}
                    onChange={(accountManagementURI) =>
                      setCreateFields((v) => ({ ...v, accountManagementURI }))
                    }
                  />
                </CreateField>
                <CreateField
                  label={
                    language === 'en'
                      ? 'Delete account URL (optional)'
                      : 'URL de exclusão da conta (opcional)'
                  }
                >
                  <Input
                    disabled={mutation.isPending}
                    value={createFields.accountDeletionURI}
                    onChange={(accountDeletionURI) =>
                      setCreateFields((v) => ({ ...v, accountDeletionURI }))
                    }
                  />
                </CreateField>
                <CreateField label={tr('Escopos')}>
                  <Input
                    disabled={mutation.isPending}
                    onChange={(scopes) =>
                      setCreateFields((value) => ({ ...value, scopes }))
                    }
                    value={createFields.scopes}
                  />
                </CreateField>
                <CreateField label={tr('Audiências')}>
                  <Input
                    disabled={mutation.isPending}
                    onChange={(audiences) =>
                      setCreateFields((value) => ({ ...value, audiences }))
                    }
                    value={createFields.audiences}
                  />
                </CreateField>
                <CreateField
                  className="sm:col-span-2"
                  label={tr('Claims (opcional)')}
                >
                  <Input
                    disabled={mutation.isPending}
                    onChange={(claims) =>
                      setCreateFields((value) => ({ ...value, claims }))
                    }
                    value={createFields.claims}
                  />
                </CreateField>
              </div>
            ) : (
              <CreateField label={tr('Nome do novo projeto')}>
                <Input
                  autoFocus
                  disabled={mutation.isPending}
                  onChange={setProjectName}
                  value={projectName}
                />
              </CreateField>
            )}
            <DialogFooter>
              <Button
                disabled={mutation.isPending}
                onClick={() => setCreateOpen(false)}
                variant="secondary"
              >
                {tr('Cancelar')}
              </Button>
              <Button
                disabled={
                  mutation.isPending ||
                  (config.createKind === 'identity-user'
                    ? !createFields.email.trim() ||
                      createFields.password.length < 12
                    : !projectName.trim() ||
                      (config.createKind === 'identity-application' &&
                        (!createFields.clientID.trim() ||
                          !createFields.redirectURIs.trim())))
                }
                onClick={submitCreate}
              >
                {tr(
                  config.createKind === 'identity-user'
                    ? 'Criar usuário'
                    : config.createKind === 'identity-application'
                      ? 'Criar aplicação'
                      : 'Criar projeto',
                )}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
        <OneTimeCredentialDialog
          credentials={revealedCredentials}
          onClose={() => setRevealedCredentials(null)}
          title={
            revealedCredentials && revealedCredentials.length > 1
              ? tr('Credenciais do projeto')
              : tr('Nova credencial')
          }
        />
      </div>
    </main>
  )
}

function CreateField({
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
