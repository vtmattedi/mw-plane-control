import {
  AppWindow,
  BookOpen,
  ChartNoAxesCombined,
  FileClock,
  FolderKanban,
  Gauge,
  KeyRound,
  Link2,
  Mail,
  Network,
  ServerCog,
  Settings2,
  ShieldCheck,
  Users,
} from 'lucide-react'
import { ResourcePage, type ResourcePageConfig } from './ResourcePage'

const security =
  'Operações sensíveis exigem autenticação forte recente. Tokens e segredos permanecem no BFF e nunca são expostos ao navegador.'

const configs: Record<string, ResourcePageConfig> = {
  identityUsers: {
    module: 'Identity',
    title: 'Usuários',
    description:
      'Identidades, estado de acesso e postura de autenticação em todos os aplicativos MW.',
    icon: Users,
    endpoint: '/identity/users',
    collectionKey: 'users',
    columns: [
      { key: 'display_name', label: 'Nome' },
      { key: 'email', label: 'E-mail' },
      { key: 'enabled', label: 'Status' },
      { key: 'is_superadmin', label: 'Superadmin' },
      { key: 'created_at', label: 'Criado em' },
    ],
    action: 'Adicionar usuário',
    emptyTitle: 'Nenhum usuário encontrado',
    emptyDescription: 'As identidades cadastradas aparecerão aqui.',
    securityNote: security,
    createKind: 'identity-user',
    rowHref: (row) =>
      row.id != null ? `/identity/users/${String(row.id)}` : undefined,
  },
  identityApps: {
    module: 'Identity',
    title: 'Aplicações',
    description:
      'Clientes OIDC, URIs exatas, audiências e políticas de autenticação.',
    icon: AppWindow,
    endpoint: '/identity/applications',
    collectionKey: 'applications',
    columns: [
      { key: 'name', label: 'Aplicação' },
      { key: 'client_id', label: 'Client ID' },
      { key: 'application_type', label: 'Tipo' },
      { key: 'authentication_policy', label: 'Política' },
      { key: 'enabled', label: 'Status' },
    ],
    action: 'Nova aplicação',
    emptyTitle: 'Nenhuma aplicação registrada',
    emptyDescription: 'Registre um cliente OIDC para começar.',
    securityNote: security,
    createKind: 'identity-application',
    rowHref: (row) =>
      row.id != null ? `/identity/applications/${String(row.id)}` : undefined,
  },
  assignments: {
    module: 'Identity',
    title: 'Atribuições e funções',
    description:
      'Controle os vínculos entre pessoas, aplicações e funções de acesso.',
    icon: Link2,
    columns: [
      { key: 'user_id', label: 'Usuário' },
      { key: 'application_id', label: 'Aplicação' },
      { key: 'role', label: 'Função' },
      { key: 'updated_at', label: 'Atualizado em' },
    ],
    action: 'Nova atribuição',
    emptyTitle: 'Selecione uma aplicação',
    emptyDescription:
      'As atribuições são carregadas no contexto de cada aplicação.',
    securityNote: security,
  },
  sessions: {
    module: 'Identity',
    title: 'Sessões',
    description:
      'Visibilidade e revogação de sessões ativas no ecossistema MattediWorks.',
    icon: KeyRound,
    columns: [
      { key: 'email', label: 'Identidade' },
      { key: 'auth_method', label: 'Método' },
      { key: 'created_at', label: 'Início' },
      { key: 'expires_at', label: 'Expira' },
      { key: 'status', label: 'Status' },
    ],
    emptyTitle: 'Nenhuma sessão para exibir',
    emptyDescription:
      'Use a página de usuários para revogar todas as sessões de uma identidade.',
    securityNote: security,
  },
  identityAudit: {
    module: 'Identity',
    title: 'Auditoria',
    description:
      'Linha do tempo imutável de autenticação e alterações administrativas.',
    icon: FileClock,
    endpoint: '/identity/audit',
    collectionKey: 'audit',
    columns: [
      { key: 'event_type', label: 'Evento' },
      { key: 'status', label: 'Status' },
      { key: 'severity', label: 'Severidade' },
      { key: 'user_id', label: 'Ator' },
      { key: 'created_at', label: 'Data' },
    ],
    emptyTitle: 'Nenhum evento registrado',
    emptyDescription: 'Eventos de segurança e administração aparecerão aqui.',
  },
  oauthProjects: {
    module: 'OAuth',
    title: 'Projetos OAuth',
    description:
      'Projetos consumidores, credenciais e limites do serviço de autorização.',
    icon: FolderKanban,
    endpoint: '/oauth/projects',
    collectionKey: 'projects',
    columns: [
      { key: 'name', label: 'Projeto' },
      { key: 'id', label: 'ID' },
      { key: 'enabled', label: 'Status' },
      { key: 'created_at', label: 'Criado em' },
    ],
    action: 'Novo projeto',
    emptyTitle: 'Nenhum projeto OAuth',
    emptyDescription: 'Crie um projeto para emitir uma chave de integração.',
    securityNote: security,
    projectActions: 'oauth',
    rowHref: (row) =>
      row.id != null
        ? `/oauth/projects/${String(row.id)}/configuration`
        : undefined,
  },
  oauthProviders: {
    module: 'OAuth',
    title: 'Provedores',
    description: 'Políticas de acesso do Google e configuração por projeto.',
    icon: Network,
    columns: [
      { key: 'project', label: 'Projeto' },
      { key: 'provider', label: 'Provedor' },
      { key: 'status', label: 'Status' },
      { key: 'updated_at', label: 'Atualizado em' },
    ],
    action: 'Configurar provedor',
    emptyTitle: 'Selecione um projeto OAuth',
    emptyDescription:
      'A política do provedor é gerenciada dentro de cada projeto.',
    securityNote: security,
  },
  oauthAudit: {
    module: 'OAuth',
    title: 'Auditoria OAuth',
    description:
      'Operações federadas, rota, escopo autorizado e ator humano preservado.',
    icon: FileClock,
    endpoint: '/oauth/audit',
    collectionKey: 'audit',
    columns: [
      { key: 'event_type', label: 'Evento' },
      { key: 'actor', label: 'Ator' },
      { key: 'status', label: 'Status' },
      { key: 'request_id', label: 'Request ID' },
      { key: 'created_at', label: 'Data' },
    ],
    emptyTitle: 'Nenhum evento OAuth',
    emptyDescription: 'As operações administrativas aparecerão aqui.',
  },
  oauthMetrics: {
    module: 'OAuth',
    title: 'Métricas OAuth',
    description: 'Saúde, volume de autorizações e comportamento operacional.',
    icon: ChartNoAxesCombined,
    endpoint: '/oauth/metrics',
    collectionKey: 'metrics',
    columns: [
      { key: 'name', label: 'Métrica' },
      { key: 'value', label: 'Valor' },
      { key: 'window', label: 'Janela' },
      { key: 'status', label: 'Status' },
    ],
    emptyTitle: 'Métricas aguardando amostras',
    emptyDescription: 'Os indicadores serão preenchidos pelo serviço OAuth.',
  },
  oauthServer: {
    module: 'OAuth',
    title: 'Servidor e API',
    description:
      'Estado do runtime, endpoints públicos e documentação versionada.',
    icon: ServerCog,
    endpoint: '/oauth/server',
    collectionKey: 'servers',
    columns: [
      { key: 'name', label: 'Componente' },
      { key: 'version', label: 'Versão' },
      { key: 'status', label: 'Status' },
      { key: 'updated_at', label: 'Atualizado em' },
    ],
    emptyTitle: 'Servidor sem metadados',
    emptyDescription: 'Conecte o módulo OAuth para visualizar seus detalhes.',
  },
  oauthDocs: {
    module: 'OAuth',
    title: 'Documentação da API',
    description:
      'Contratos públicos e guias de integração para consumidores do MW OAuth.',
    icon: BookOpen,
    endpoint: '/oauth/docs',
    collectionKey: 'versions',
    columns: [
      { key: 'version', label: 'Versão' },
      { key: 'status', label: 'Status' },
      { key: 'content_type', label: 'Formato' },
    ],
    rowHref: (row) =>
      typeof row.version === 'string'
        ? `/oauth/docs/${row.version}`
        : undefined,
    emptyTitle: 'Nenhuma versão publicada',
    emptyDescription: 'A documentação publicada pelo serviço aparecerá aqui.',
  },
  mailProjects: {
    module: 'Mail',
    title: 'Projetos de e-mail',
    description:
      'Projetos remetentes, chaves de API e domínios de requisição permitidos.',
    icon: FolderKanban,
    endpoint: '/mail/projects',
    collectionKey: 'projects',
    columns: [
      { key: 'name', label: 'Projeto' },
      { key: 'id', label: 'ID' },
      { key: 'monthly_limit', label: 'Limite mensal' },
      { key: 'enabled', label: 'Status' },
      { key: 'updated_at', label: 'Atualizado em' },
    ],
    action: 'Novo projeto',
    emptyTitle: 'Nenhum projeto de e-mail',
    emptyDescription: 'Crie um projeto para começar a enviar mensagens.',
    securityNote: security,
    projectActions: 'mail',
    rowHref: (row) =>
      row.id != null ? `/mail/configuration/${String(row.id)}` : undefined,
  },
  mailConfiguration: {
    module: 'Mail',
    title: 'Configuração',
    description:
      'Domínios de requisição, credenciais e políticas de entrega por projeto.',
    icon: Settings2,
    columns: [
      { key: 'project', label: 'Projeto' },
      { key: 'domains', label: 'Domínios permitidos' },
      { key: 'provider', label: 'Provedor' },
      { key: 'status', label: 'Status' },
    ],
    action: 'Configurar projeto',
    emptyTitle: 'Selecione um projeto de e-mail',
    emptyDescription: 'A configuração é aplicada de forma isolada por projeto.',
    securityNote: security,
  },
  mailEvents: {
    module: 'Mail',
    title: 'Mensagens e eventos',
    description:
      'Inspeção de tentativas, entrega, bounce, supressão e reclamações.',
    icon: Mail,
    endpoint: '/mail/emails?limit=50&offset=0',
    collectionKey: 'emails',
    columns: [
      { key: 'subject', label: 'Assunto' },
      { key: 'project_name', label: 'Projeto' },
      { key: 'sender', label: 'Remetente' },
      { key: 'delivery_status', label: 'Entrega' },
      { key: 'created_at', label: 'Data' },
    ],
    emptyTitle: 'Nenhuma mensagem encontrada',
    emptyDescription: 'As tentativas recentes aparecerão aqui.',
  },
  mailUsage: {
    module: 'Mail',
    title: 'Uso e métricas',
    description:
      'Volume por projeto e leitura de consumo da capacidade mensal.',
    icon: Gauge,
    endpoint: '/mail/emails/by-project',
    collectionKey: 'projects',
    columns: [
      { key: 'name', label: 'Projeto' },
      { key: 'email_count', label: 'Total' },
      { key: 'send.pending', label: 'Pendentes' },
      { key: 'send.sent', label: 'Enviados' },
      { key: 'delivery.delivered', label: 'Entregues' },
      { key: 'send.failed', label: 'Falhas' },
    ],
    emptyTitle: 'Uso ainda não calculado',
    emptyDescription: 'Os totais serão exibidos após os primeiros envios.',
  },
  mailAudit: {
    module: 'Mail',
    title: 'Auditoria Mail',
    description:
      'Alterações administrativas e operações federadas do serviço de e-mail.',
    icon: ShieldCheck,
    columns: [
      { key: 'event_type', label: 'Evento' },
      { key: 'actor', label: 'Ator' },
      { key: 'status', label: 'Status' },
      { key: 'request_id', label: 'Request ID' },
      { key: 'created_at', label: 'Data' },
    ],
    emptyTitle: 'Auditoria indisponível',
    emptyDescription:
      'O contrato atual do Mail ainda não publica uma rota de auditoria.',
  },
  mailDocs: {
    module: 'Mail',
    title: 'Documentação da API',
    description:
      'Contratos públicos e guias de integração para consumidores do MW Mail.',
    icon: BookOpen,
    endpoint: '/mail/docs',
    collectionKey: 'versions',
    columns: [
      { key: 'version', label: 'Versão' },
      { key: 'status', label: 'Status' },
      { key: 'content_type', label: 'Formato' },
    ],
    rowHref: (row) =>
      typeof row.version === 'string' ? `/mail/docs/${row.version}` : undefined,
    emptyTitle: 'Nenhuma versão publicada',
    emptyDescription: 'A documentação publicada pelo serviço aparecerá aqui.',
  },
}

export const IdentityUsersPage = () => (
  <ResourcePage config={configs.identityUsers} />
)
export const IdentityApplicationsPage = () => (
  <ResourcePage config={configs.identityApps} />
)
export const IdentityAssignmentsPage = () => (
  <ResourcePage config={configs.assignments} />
)
export const IdentitySessionsPage = () => (
  <ResourcePage config={configs.sessions} />
)
export const IdentityAuditPage = () => (
  <ResourcePage config={configs.identityAudit} />
)
export const OAuthProjectsPage = () => (
  <ResourcePage config={configs.oauthProjects} />
)
export const OAuthProvidersPage = () => (
  <ResourcePage config={configs.oauthProviders} />
)
export const OAuthAuditPage = () => <ResourcePage config={configs.oauthAudit} />
export const OAuthMetricsPage = () => (
  <ResourcePage config={configs.oauthMetrics} />
)
export const OAuthServerPage = () => (
  <ResourcePage config={configs.oauthServer} />
)
export const OAuthDocsPage = () => <ResourcePage config={configs.oauthDocs} />
export const MailProjectsPage = () => (
  <ResourcePage config={configs.mailProjects} />
)
export const MailEventsPage = () => <ResourcePage config={configs.mailEvents} />
export const MailUsagePage = () => <ResourcePage config={configs.mailUsage} />
export const MailAuditPage = () => <ResourcePage config={configs.mailAudit} />
export const MailDocsPage = () => <ResourcePage config={configs.mailDocs} />
