import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  AlertTriangle,
  Database,
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
import { OneTimeCredentialDialog } from '@/components/one-time-credential-dialog'
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
  const { tr } = usePreferences()
  const confirm = useConfirm()
  const [search, setSearch] = useState('')
  const [createOpen, setCreateOpen] = useState(false)
  const [projectName, setProjectName] = useState('')
  const [revealedCredential, setRevealedCredential] = useState<string | null>(
    null,
  )
  const [createFields, setCreateFields] = useState({
    email: '',
    displayName: '',
    password: '',
    clientID: '',
    applicationType: 'confidential',
    authenticationPolicy: 'standard',
    redirectURIs: '',
    postLogoutRedirectURIs: '',
    deleteCallbackURI: '',
    allowPublicSignup: false,
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
      const secret =
        record.api_key ?? record.callback_secret ?? record.client_secret
      if (typeof secret === 'string') {
        setRevealedCredential(secret)
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
        allowPublicSignup: false,
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

  const values = (value: string) =>
    value
      .split(/[\n,]/)
      .map((item) => item.trim())
      .filter(Boolean)

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
        authentication_policy: createFields.authenticationPolicy,
        redirect_uris: values(createFields.redirectURIs),
        post_logout_redirect_uris: values(createFields.postLogoutRedirectURIs),
        delete_callback_uri: createFields.deleteCallbackURI.trim(),
        allow_public_signup: createFields.allowPublicSignup,
        scopes: values(createFields.scopes),
        claims: values(createFields.claims),
        audiences: values(createFields.audiences),
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
          <MetricCard
            icon={Database}
            label={tr('Registros')}
            note={tr('Disponíveis neste ambiente')}
            value={query.isLoading ? '…' : rows.length}
            tone="accent"
          />
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
                <CreateField label="Client ID">
                  <Input
                    disabled={mutation.isPending}
                    onChange={(clientID) =>
                      setCreateFields((value) => ({ ...value, clientID }))
                    }
                    value={createFields.clientID}
                  />
                </CreateField>
                <CreateField label={tr('Tipo')}>
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
                    <option value="confidential">Confidential</option>
                    <option value="public">Public</option>
                  </Select>
                </CreateField>
                <CreateField label={tr('Política')}>
                  <Select
                    disabled={mutation.isPending}
                    onChange={(event) =>
                      setCreateFields((value) => ({
                        ...value,
                        authenticationPolicy: event.target.value,
                      }))
                    }
                    value={createFields.authenticationPolicy}
                  >
                    <option value="standard">Standard</option>
                    <option value="strong">Strong</option>
                  </Select>
                </CreateField>
                <CreateField className="sm:col-span-2" label="Redirect URIs">
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
                  label="Post logout redirect URIs"
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
                  label={tr('Callback de exclusão de conta')}
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
                <CreateField label={tr('Cadastro público')}>
                  <Select
                    disabled={mutation.isPending}
                    onChange={(event) =>
                      setCreateFields((value) => ({
                        ...value,
                        allowPublicSignup: event.target.value === 'enabled',
                      }))
                    }
                    value={
                      createFields.allowPublicSignup ? 'enabled' : 'disabled'
                    }
                  >
                    <option value="disabled">{tr('Desativado')}</option>
                    <option value="enabled">{tr('Ativado')}</option>
                  </Select>
                </CreateField>
                <CreateField label="Scopes">
                  <Input
                    disabled={mutation.isPending}
                    onChange={(scopes) =>
                      setCreateFields((value) => ({ ...value, scopes }))
                    }
                    value={createFields.scopes}
                  />
                </CreateField>
                <CreateField label="Audiences">
                  <Input
                    disabled={mutation.isPending}
                    onChange={(audiences) =>
                      setCreateFields((value) => ({ ...value, audiences }))
                    }
                    value={createFields.audiences}
                  />
                </CreateField>
                <CreateField className="sm:col-span-2" label="Claims">
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
          credential={revealedCredential}
          onClose={() => setRevealedCredential(null)}
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
