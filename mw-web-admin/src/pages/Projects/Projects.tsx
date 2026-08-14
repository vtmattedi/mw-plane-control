import {
  Check,
  CircleAlert,
  Copy,
  FolderKanban,
  KeyRound,
  LoaderCircle,
  Pencil,
  Plus,
  RefreshCw,
  ShieldCheck,
  X,
} from 'lucide-react'
import {
  useEffect,
  useMemo,
  useState,
  type FormEvent,
  type ReactNode,
} from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { Button } from '@/components/ui/button'
import {
  ApiError,
  createProject,
  getProjects,
  rotateProjectApiKey,
  updateProject,
  type ProjectRecord,
} from '@/lib/api'
import { Input } from '@/components/ui/input'

type ProjectFormState = {
  name: string
  monthlyLimit: string
}

type RevealedKey = {
  projectId: number
  projectName: string
  apiKey: string
  kind: 'created' | 'rotated'
}

const emptyForm: ProjectFormState = {
  name: '',
  monthlyLimit: '',
}

function parseMonthlyLimit(value: string): number | null {
  const trimmed = value.trim()

  if (trimmed === '') return null

  const parsed = Number(trimmed)

  if (!Number.isInteger(parsed) || parsed < 0) {
    throw new Error(
      'O limite mensal deve ser um número inteiro maior ou igual a zero.',
    )
  }

  return parsed
}

function errorMessage(error: unknown, fallback: string) {
  if (error instanceof ApiError) return error.message
  if (error instanceof Error) return error.message
  return fallback
}

function Projects() {
  const queryClient = useQueryClient()

  const [search, setSearch] = useState('')
  const [createOpen, setCreateOpen] = useState(false)
  const [editingProject, setEditingProject] = useState<ProjectRecord | null>(
    null,
  )
  const [rotateProject, setRotateProject] = useState<ProjectRecord | null>(null)
  const [revealedKey, setRevealedKey] = useState<RevealedKey | null>(null)

  const [createForm, setCreateForm] = useState<ProjectFormState>(emptyForm)
  const [editForm, setEditForm] = useState<ProjectFormState>(emptyForm)

  const [createError, setCreateError] = useState<string | null>(null)
  const [editError, setEditError] = useState<string | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  const projectsQuery = useQuery({
    queryKey: ['mw-email', 'projects'],
    queryFn: getProjects,
  })

  const createMutation = useMutation({
    mutationFn: createProject,
    onSuccess: async (result) => {
      setCreateOpen(false)
      setCreateForm(emptyForm)
      setCreateError(null)

      setRevealedKey({
        projectId: result.project.id,
        projectName: result.project.name,
        apiKey: result.apiKey,
        kind: 'created',
      })

      await queryClient.invalidateQueries({
        queryKey: ['mw-email', 'projects'],
      })
    },
    onError: (error) => {
      setCreateError(errorMessage(error, 'Não foi possível criar o projeto.'))
    },
  })

  const updateMutation = useMutation({
    mutationFn: ({
      id,
      input,
    }: {
      id: number
      input: Parameters<typeof updateProject>[1]
    }) => updateProject(id, input),
    onSuccess: async () => {
      setEditingProject(null)
      setEditError(null)

      await queryClient.invalidateQueries({
        queryKey: ['mw-email', 'projects'],
      })
    },
    onError: (error) => {
      setEditError(errorMessage(error, 'Não foi possível atualizar o projeto.'))
    },
  })

  const toggleMutation = useMutation({
    mutationFn: ({
      project,
      enabled,
    }: {
      project: ProjectRecord
      enabled: boolean
    }) =>
      updateProject(project.id, {
        enabled,
      }),
    onSuccess: async () => {
      setActionError(null)

      await queryClient.invalidateQueries({
        queryKey: ['mw-email', 'projects'],
      })
    },
    onError: (error) => {
      setActionError(
        errorMessage(error, 'Não foi possível alterar o estado do projeto.'),
      )
    },
  })

  const rotateMutation = useMutation({
    mutationFn: async (project: ProjectRecord) => ({
      project,
      apiKey: await rotateProjectApiKey(project.id),
    }),
    onSuccess: ({ project, apiKey }) => {
      setRotateProject(null)
      setActionError(null)

      setRevealedKey({
        projectId: project.id,
        projectName: project.name,
        apiKey,
        kind: 'rotated',
      })
    },
    onError: (error) => {
      setActionError(
        errorMessage(error, 'Não foi possível rotacionar a chave.'),
      )
    },
  })

  const projects = projectsQuery.data ?? []

  const filteredProjects = useMemo(() => {
    const term = search.trim().toLocaleLowerCase('pt-BR')

    if (!term) return projects

    return projects.filter((project) =>
      project.name.toLocaleLowerCase('pt-BR').includes(term),
    )
  }, [projects, search])

  const enabledCount = projects.filter((project) => project.enabled).length

  useEffect(() => {
    setCopied(false)
  }, [revealedKey])

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return

      if (revealedKey) {
        setRevealedKey(null)
        return
      }

      if (rotateProject) {
        setRotateProject(null)
        return
      }

      if (editingProject) {
        setEditingProject(null)
        return
      }

      if (createOpen) {
        setCreateOpen(false)
      }
    }

    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [createOpen, editingProject, revealedKey, rotateProject])

  function openCreate() {
    setCreateError(null)
    setCreateForm(emptyForm)
    setCreateOpen(true)
  }

  function openEdit(project: ProjectRecord) {
    setEditError(null)
    setEditForm({
      name: project.name,
      monthlyLimit:
        project.monthlyLimit === null ? '' : String(project.monthlyLimit),
    })
    setEditingProject(project)
  }

  function submitCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setCreateError(null)

    const name = createForm.name.trim()

    if (!name) {
      setCreateError('Informe o nome do projeto.')
      return
    }

    try {
      const monthlyLimit = parseMonthlyLimit(createForm.monthlyLimit)

      createMutation.mutate({
        name,
        monthlyLimit,
      })
    } catch (error) {
      setCreateError(errorMessage(error, 'Verifique os dados informados.'))
    }
  }

  function submitEdit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    if (!editingProject) return

    setEditError(null)

    const name = editForm.name.trim()

    if (!name) {
      setEditError('Informe o nome do projeto.')
      return
    }

    try {
      const monthlyLimit = parseMonthlyLimit(editForm.monthlyLimit)

      updateMutation.mutate({
        id: editingProject.id,
        input: {
          name,
          ...(monthlyLimit === null
            ? { clearMonthlyLimit: true }
            : { monthlyLimit }),
        },
      })
    } catch (error) {
      setEditError(errorMessage(error, 'Verifique os dados informados.'))
    }
  }

  async function copyKey() {
    if (!revealedKey) return

    await navigator.clipboard.writeText(revealedKey.apiKey)
    setCopied(true)

    window.setTimeout(() => {
      setCopied(false)
    }, 1800)
  }

  return (
    <main className="mx-auto flex w-full max-w-[1600px] flex-col gap-8 px-4 py-8 sm:px-6">
      <header className="flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
        <div className="space-y-1">
          <p className="text-muted-foreground text-sm font-medium">
            MW Email Service
          </p>

          <h1 className="text-3xl font-semibold tracking-tight">Projetos</h1>

          <p className="text-muted-foreground max-w-2xl text-sm">
            Gerencie as aplicações autorizadas a enviar e-mails através do
            gateway.
          </p>
        </div>

        <Button onClick={openCreate}>
          <Plus aria-hidden="true" className="size-4" />
          Novo projeto
        </Button>
      </header>

      <section
        aria-label="Resumo dos projetos"
        className="grid gap-4 sm:grid-cols-3"
      >
        <SummaryCard
          label="Total"
          value={projects.length}
          description="Projetos cadastrados"
        />

        <SummaryCard
          label="Ativos"
          value={enabledCount}
          description="Podem autenticar no gateway"
        />

        <SummaryCard
          label="Inativos"
          value={projects.length - enabledCount}
          description="Acesso ao envio bloqueado"
        />
      </section>

      {actionError && (
        <div
          className="border-destructive/30 bg-destructive/5 flex items-start gap-3 rounded-xl border px-4 py-3 text-sm"
          role="alert"
        >
          <CircleAlert
            aria-hidden="true"
            className="text-destructive mt-0.5 size-4 shrink-0"
          />

          <p className="flex-1">{actionError}</p>

          <Button
            aria-label="Fechar aviso"
            onClick={() => setActionError(null)}
            size="icon"
            variant="ghost"
          >
            <X aria-hidden="true" className="size-4" />
          </Button>
        </div>
      )}

      <section className="bg-card overflow-hidden rounded-xl border">
        <div className="flex flex-col gap-4 border-b px-5 py-4 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h2 className="font-semibold">Projetos cadastrados</h2>

            <p className="text-muted-foreground text-sm">
              Cada projeto possui sua própria credencial de acesso.
            </p>
          </div>

          {/* <div className="flex gap-2">
            <label className="relative block min-w-0 sm:w-72">
              <span className="sr-only">Buscar projeto</span>

              <Search
                aria-hidden="true"
                className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
              />

              <input
                className="h-9 w-full rounded-md border bg-background pl-9 pr-9 text-sm outline-none transition focus-visible:ring-2 focus-visible:ring-ring"
                placeholder="Buscar projeto..."
                type="search"
                value={search}
                onChange={(event) => setSearch(event.target.value)}
              />

              {search && (
                <button
                  aria-label="Limpar busca"
                  className="absolute right-2 top-1/2 grid size-6 -translate-y-1/2 place-items-center rounded text-muted-foreground hover:bg-muted hover:text-foreground"
                  onClick={() => setSearch('')}
                  type="button"
                >
                  <X aria-hidden="true" className="size-3.5" />
                </button>
              )}
            </label>

            <Button
              aria-label="Atualizar projetos"
              disabled={projectsQuery.isFetching}
              onClick={() => void projectsQuery.refetch()}
              size="icon"
              variant="outline"
            >
              <RefreshCw
                aria-hidden="true"
                className={`size-4 ${
                  projectsQuery.isFetching ? 'animate-spin' : ''
                }`}
              />
            </Button>
          </div> */}
          <div className="flex gap-2">
            <Input
              variant="search"
              placeholder="Buscar projeto..."
              value={search}
              onChange={(text) => setSearch(text)}
            />
          </div>
        </div>

        {projectsQuery.isLoading ? (
          <LoadingState />
        ) : projectsQuery.isError ? (
          <ErrorState retry={() => void projectsQuery.refetch()} />
        ) : filteredProjects.length === 0 ? (
          <EmptyState
            hasSearch={Boolean(search.trim())}
            onCreate={openCreate}
          />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[760px] text-left text-sm">
              <thead className="bg-muted/40 text-muted-foreground border-b text-xs">
                <tr>
                  <th className="px-5 py-3 font-medium">Projeto</th>
                  <th className="px-5 py-3 font-medium">Limite mensal</th>
                  <th className="px-5 py-3 font-medium">Status</th>
                  <th className="px-5 py-3 text-right font-medium">Ações</th>
                </tr>
              </thead>

              <tbody>
                {filteredProjects.map((project) => (
                  <ProjectRow
                    key={project.id}
                    project={project}
                    toggling={
                      toggleMutation.isPending &&
                      toggleMutation.variables?.project.id === project.id
                    }
                    onEdit={() => openEdit(project)}
                    onRotate={() => {
                      setActionError(null)
                      setRotateProject(project)
                    }}
                    onToggle={() => {
                      setActionError(null)

                      toggleMutation.mutate({
                        project,
                        enabled: !project.enabled,
                      })
                    }}
                  />
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      {createOpen && (
        <Modal
          title="Novo projeto"
          description="Crie uma credencial independente para uma aplicação usar o MW Email Service."
          onClose={() => !createMutation.isPending && setCreateOpen(false)}
        >
          <ProjectForm
            error={createError}
            form={createForm}
            pending={createMutation.isPending}
            submitLabel="Criar projeto"
            onCancel={() => setCreateOpen(false)}
            onChange={setCreateForm}
            onSubmit={submitCreate}
          />
        </Modal>
      )}

      {editingProject && (
        <Modal
          title="Editar projeto"
          description={editingProject.name}
          onClose={() => !updateMutation.isPending && setEditingProject(null)}
        >
          <ProjectForm
            error={editError}
            form={editForm}
            pending={updateMutation.isPending}
            submitLabel="Salvar alterações"
            onCancel={() => setEditingProject(null)}
            onChange={setEditForm}
            onSubmit={submitEdit}
          />
        </Modal>
      )}

      {rotateProject && (
        <Modal
          title="Rotacionar chave"
          description={rotateProject.name}
          onClose={() => !rotateMutation.isPending && setRotateProject(null)}
        >
          <div className="space-y-5">
            <div className="flex gap-3 rounded-lg border border-amber-500/30 bg-amber-500/5 p-4">
              <CircleAlert
                aria-hidden="true"
                className="mt-0.5 size-5 shrink-0 text-amber-600 dark:text-amber-400"
              />

              <div className="space-y-1 text-sm">
                <p className="font-medium">
                  A chave atual deixará de funcionar.
                </p>

                <p className="text-muted-foreground">
                  Você precisará atualizar
                  <code className="bg-muted mx-1 rounded px-1 py-0.5 text-xs">
                    MW_EMAIL_API_KEY
                  </code>
                  na aplicação que utiliza este projeto.
                </p>
              </div>
            </div>

            <div className="flex justify-end gap-2">
              <Button
                disabled={rotateMutation.isPending}
                onClick={() => setRotateProject(null)}
                variant="outline"
              >
                Cancelar
              </Button>

              <Button
                disabled={rotateMutation.isPending}
                onClick={() => rotateMutation.mutate(rotateProject)}
                variant="destructive"
              >
                {rotateMutation.isPending ? (
                  <LoaderCircle
                    aria-hidden="true"
                    className="size-4 animate-spin"
                  />
                ) : (
                  <KeyRound aria-hidden="true" className="size-4" />
                )}
                Rotacionar chave
              </Button>
            </div>
          </div>
        </Modal>
      )}

      {revealedKey && (
        <Modal
          title={
            revealedKey.kind === 'created'
              ? 'Projeto criado'
              : 'Nova chave gerada'
          }
          description={revealedKey.projectName}
          onClose={() => setRevealedKey(null)}
        >
          <div className="space-y-5">
            <div className="flex gap-3 rounded-lg border border-emerald-500/30 bg-emerald-500/5 p-4">
              <ShieldCheck
                aria-hidden="true"
                className="mt-0.5 size-5 shrink-0 text-emerald-600 dark:text-emerald-400"
              />

              <div className="space-y-1 text-sm">
                <p className="font-medium">Guarde esta chave agora.</p>

                <p className="text-muted-foreground">
                  Por segurança, a chave completa não poderá ser consultada
                  novamente.
                </p>
              </div>
            </div>

            <div className="space-y-2">
              <label
                className="text-sm font-medium"
                htmlFor="generated-api-key"
              >
                MW_EMAIL_API_KEY
              </label>

              <div className="flex gap-2">
                <input
                  className="bg-muted/40 h-10 min-w-0 flex-1 rounded-md border px-3 font-mono text-xs outline-none"
                  id="generated-api-key"
                  readOnly
                  value={revealedKey.apiKey}
                />

                <Button
                  aria-label="Copiar chave"
                  onClick={() => void copyKey()}
                  size="icon"
                  variant="outline"
                >
                  {copied ? (
                    <Check aria-hidden="true" className="size-4" />
                  ) : (
                    <Copy aria-hidden="true" className="size-4" />
                  )}
                </Button>
              </div>
            </div>

            <div className="bg-muted/50 rounded-lg p-3 font-mono text-xs">
              MW_EMAIL_API_KEY={revealedKey.apiKey}
            </div>

            <div className="flex justify-end">
              <Button onClick={() => setRevealedKey(null)}>Concluído</Button>
            </div>
          </div>
        </Modal>
      )}
    </main>
  )
}

function ProjectRow({
  project,
  toggling,
  onEdit,
  onRotate,
  onToggle,
}: {
  project: ProjectRecord
  toggling: boolean
  onEdit: () => void
  onRotate: () => void
  onToggle: () => void
}) {
  return (
    <tr className="hover:bg-muted/30 border-b last:border-b-0">
      <td className="px-5 py-4">
        <div className="flex items-center gap-3">
          <div className="bg-muted/30 grid size-9 shrink-0 place-items-center rounded-lg border">
            <FolderKanban
              aria-hidden="true"
              className="text-muted-foreground size-4"
            />
          </div>

          <div className="min-w-0">
            <p className="truncate font-medium">{project.name}</p>
            <p className="text-muted-foreground text-xs">ID {project.id}</p>
          </div>
        </div>
      </td>

      <td className="px-5 py-4">
        {project.monthlyLimit === null ? (
          <span className="text-muted-foreground">Sem limite</span>
        ) : (
          project.monthlyLimit.toLocaleString('pt-BR')
        )}
      </td>

      <td className="px-5 py-4">
        <button
          aria-label={
            project.enabled
              ? `Desativar ${project.name}`
              : `Ativar ${project.name}`
          }
          className={`inline-flex items-center gap-2 rounded-full px-2.5 py-1 text-xs font-medium transition ${
            project.enabled
              ? 'bg-emerald-500/10 text-emerald-700 hover:bg-emerald-500/15 dark:text-emerald-400'
              : 'bg-muted text-muted-foreground hover:bg-muted/80'
          }`}
          disabled={toggling}
          onClick={onToggle}
          type="button"
        >
          {toggling ? (
            <LoaderCircle aria-hidden="true" className="size-3 animate-spin" />
          ) : (
            <span
              aria-hidden="true"
              className={`size-1.5 rounded-full ${
                project.enabled ? 'bg-emerald-500' : 'bg-muted-foreground'
              }`}
            />
          )}

          {project.enabled ? 'Ativo' : 'Inativo'}
        </button>
      </td>

      <td className="px-5 py-4">
        <div className="flex justify-end gap-2">
          <Button onClick={onEdit} size="sm" variant="outline">
            <Pencil aria-hidden="true" className="size-4" />
            Editar
          </Button>

          <Button onClick={onRotate} size="sm" variant="outline">
            <KeyRound aria-hidden="true" className="size-4" />
            Nova chave
          </Button>
        </div>
      </td>
    </tr>
  )
}

function ProjectForm({
  form,
  pending,
  error,
  submitLabel,
  onChange,
  onCancel,
  onSubmit,
}: {
  form: ProjectFormState
  pending: boolean
  error: string | null
  submitLabel: string
  onChange: (value: ProjectFormState) => void
  onCancel: () => void
  onSubmit: (event: FormEvent<HTMLFormElement>) => void
}) {
  return (
    <form className="space-y-5" onSubmit={onSubmit}>
      <div className="space-y-2">
        <label className="text-sm font-medium" htmlFor="project-name">
          Nome
        </label>

        <input
          autoFocus
          className="bg-background focus-visible:ring-ring h-10 w-full rounded-md border px-3 text-sm transition outline-none focus-visible:ring-2"
          disabled={pending}
          id="project-name"
          maxLength={120}
          placeholder="Ex.: Clínica Mantra"
          required
          value={form.name}
          onChange={(event) =>
            onChange({
              ...form,
              name: event.target.value,
            })
          }
        />
      </div>

      <div className="space-y-2">
        <label className="text-sm font-medium" htmlFor="project-monthly-limit">
          Limite mensal
        </label>

        <input
          className="bg-background focus-visible:ring-ring h-10 w-full rounded-md border px-3 text-sm transition outline-none focus-visible:ring-2"
          disabled={pending}
          id="project-monthly-limit"
          inputMode="numeric"
          min={0}
          placeholder="Sem limite"
          step={1}
          type="number"
          value={form.monthlyLimit}
          onChange={(event) =>
            onChange({
              ...form,
              monthlyLimit: event.target.value,
            })
          }
        />

        <p className="text-muted-foreground text-xs">
          Deixe vazio para não aplicar limite mensal.
        </p>
      </div>

      {error && (
        <div
          className="border-destructive/30 bg-destructive/5 flex items-start gap-2 rounded-lg border p-3 text-sm"
          role="alert"
        >
          <CircleAlert
            aria-hidden="true"
            className="text-destructive mt-0.5 size-4 shrink-0"
          />
          {error}
        </div>
      )}

      <div className="flex justify-end gap-2">
        <Button
          disabled={pending}
          onClick={onCancel}
          type="button"
          variant="outline"
        >
          Cancelar
        </Button>

        <Button disabled={pending} type="submit">
          {pending && (
            <LoaderCircle aria-hidden="true" className="size-4 animate-spin" />
          )}
          {submitLabel}
        </Button>
      </div>
    </form>
  )
}

function Modal({
  title,
  description,
  children,
  onClose,
}: {
  title: string
  description?: string
  children: ReactNode
  onClose: () => void
}) {
  return (
    <div
      className="fixed inset-0 z-50 grid place-items-center overflow-y-auto bg-black/45 p-4 backdrop-blur-[2px]"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) {
          onClose()
        }
      }}
    >
      <section
        aria-describedby={description ? 'project-modal-description' : undefined}
        aria-labelledby="project-modal-title"
        aria-modal="true"
        className="bg-background w-full max-w-lg rounded-xl border shadow-2xl"
        role="dialog"
      >
        <header className="flex items-start gap-4 border-b px-5 py-4">
          <div className="min-w-0 flex-1">
            <h2 className="font-semibold" id="project-modal-title">
              {title}
            </h2>

            {description && (
              <p
                className="text-muted-foreground mt-1 truncate text-sm"
                id="project-modal-description"
              >
                {description}
              </p>
            )}
          </div>

          <Button
            aria-label="Fechar"
            onClick={onClose}
            size="icon"
            variant="ghost"
          >
            <X aria-hidden="true" className="size-4" />
          </Button>
        </header>

        <div className="p-5">{children}</div>
      </section>
    </div>
  )
}

function SummaryCard({
  label,
  value,
  description,
}: {
  label: string
  value: number
  description: string
}) {
  return (
    <article className="bg-card rounded-xl border p-5">
      <p className="text-muted-foreground text-sm font-medium">{label}</p>
      <p className="mt-3 text-3xl font-semibold tracking-tight">
        {value.toLocaleString('pt-BR')}
      </p>
      <p className="text-muted-foreground mt-1 text-sm">{description}</p>
    </article>
  )
}

function LoadingState() {
  return (
    <div className="text-muted-foreground flex min-h-48 items-center justify-center gap-2 text-sm">
      <LoaderCircle aria-hidden="true" className="size-4 animate-spin" />
      Carregando projetos…
    </div>
  )
}

function ErrorState({ retry }: { retry: () => void }) {
  return (
    <div className="flex min-h-48 flex-col items-center justify-center gap-3 px-6 text-center">
      <CircleAlert aria-hidden="true" className="text-destructive size-5" />

      <div>
        <p className="font-medium">Não foi possível carregar os projetos.</p>

        <p className="text-muted-foreground mt-1 text-sm">
          Verifique a conexão com o MW Email Service.
        </p>
      </div>

      <Button onClick={retry} size="sm" variant="outline">
        <RefreshCw aria-hidden="true" className="size-4" />
        Tentar novamente
      </Button>
    </div>
  )
}

function EmptyState({
  hasSearch,
  onCreate,
}: {
  hasSearch: boolean
  onCreate: () => void
}) {
  return (
    <div className="flex min-h-56 flex-col items-center justify-center gap-4 px-6 text-center">
      <div className="bg-muted grid size-11 place-items-center rounded-full">
        <FolderKanban
          aria-hidden="true"
          className="text-muted-foreground size-5"
        />
      </div>

      <div>
        <p className="font-medium">
          {hasSearch
            ? 'Nenhum projeto encontrado'
            : 'Nenhum projeto cadastrado'}
        </p>

        <p className="text-muted-foreground mt-1 text-sm">
          {hasSearch
            ? 'Tente buscar por outro nome.'
            : 'Crie o primeiro projeto para gerar uma credencial de acesso.'}
        </p>
      </div>

      {!hasSearch && (
        <Button onClick={onCreate}>
          <Plus aria-hidden="true" className="size-4" />
          Novo projeto
        </Button>
      )}
    </div>
  )
}

export default Projects
