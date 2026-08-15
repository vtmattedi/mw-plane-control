// oxlint-disable react/only-export-components -- provider and hook form one public preferences API.
import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react'
import { applyTheme, preferredTheme, type Theme } from '@/lib/theme'

export type Language = 'pt' | 'en'
export type Density = 'comfortable' | 'compact'

type Preferences = {
  language: Language
  theme: Theme
  density: Density
  groupOpen: Record<string, boolean>
}

type PreferencesContextValue = Preferences & {
  setLanguage: (language: Language) => void
  setTheme: (theme: Theme) => void
  setDensity: (density: Density) => void
  toggleGroup: (group: string) => void
  tr: (text: string) => string
}

const storageKey = 'mw-control-preferences-v1'
const defaults: Preferences = {
  language:
    typeof navigator !== 'undefined' &&
    navigator.language.toLowerCase().startsWith('en')
      ? 'en'
      : 'pt',
  theme: typeof window === 'undefined' ? 'light' : preferredTheme(),
  density: 'comfortable',
  groupOpen: { Workspace: true, Identity: true, OAuth: true, Mail: true },
}

const english: Record<string, string> = {
  'Visão geral': 'Overview',
  Usuários: 'Users',
  Aplicações: 'Applications',
  Atribuições: 'Assignments',
  Sessões: 'Sessions',
  Auditoria: 'Audit',
  Projetos: 'Projects',
  Provedores: 'Providers',
  Métricas: 'Metrics',
  Servidor: 'Server',
  Documentação: 'Documentation',
  Configuração: 'Configuration',
  Mensagens: 'Messages',
  'Uso e métricas': 'Usage and metrics',
  'Conectado como': 'Signed in as',
  Perfil: 'Profile',
  Preferências: 'Preferences',
  Aparência: 'Appearance',
  Idioma: 'Language',
  Tema: 'Theme',
  Claro: 'Light',
  Escuro: 'Dark',
  Densidade: 'Density',
  Confortável: 'Comfortable',
  Compacta: 'Compact',
  Sair: 'Sign out',
  'Saindo…': 'Signing out…',
  'Abrir navegação': 'Open navigation',
  'Fechar navegação': 'Close navigation',
  'Expandir menu': 'Expand menu',
  'Recolher menu': 'Collapse menu',
  'Idioma e preferências': 'Language and preferences',
  'Navegação principal': 'Main navigation',
  Ações: 'Actions',
  Abrir: 'Open',
  Resumo: 'Summary',
  Buscar: 'Search',
  'Não foi possível concluir a operação.':
    'The operation could not be completed.',
  'O módulo ainda não respondeu. A navegação permanece disponível e os dados serão exibidos assim que o serviço estiver conectado.':
    'The module has not responded yet. Navigation remains available and data will appear as soon as the service is connected.',
  Registros: 'Records',
  'Uso mensal': 'Monthly usage',
  'Cota mensal de mensagens': 'Monthly message quota',
  Ativos: 'Active',
  Proteção: 'Protection',
  Sincronização: 'Synchronization',
  'Disponíveis neste ambiente': 'Available in this environment',
  'Com operação permitida': 'Allowed to operate',
  'Sessão BFF + escopo delegado': 'BFF session + delegated scope',
  'Dados lidos diretamente do serviço': 'Data read directly from the service',
  Agora: 'Now',
  'Em dia': 'Up to date',
  'Buscar registros…': 'Search records…',
  Atualizar: 'Refresh',
  'Gerencie e acompanhe este recurso pelo control plane.':
    'Manage and monitor this resource through the control plane.',
  'Operação concluída com sucesso.': 'Operation completed successfully.',
  'Copiar segredo': 'Copy secret',
  'Este segredo é exibido apenas uma vez. Guarde-o agora.':
    'This secret is shown only once. Store it now.',
  Desativar: 'Disable',
  Ativar: 'Enable',
  'Rotacionar chave': 'Rotate key',
  'Nome do novo projeto': 'New project name',
  'Projetos OAuth': 'OAuth projects',
  'Auditoria OAuth': 'OAuth audit',
  'Métricas OAuth': 'OAuth metrics',
  'Servidor e API': 'Server and API',
  'Documentação da API': 'API documentation',
  'Projetos de e-mail': 'Mail projects',
  'Mensagens e eventos': 'Messages and events',
  'Auditoria Mail': 'Mail audit',
  'Atribuições e funções': 'Assignments and roles',
  'Nova aplicação': 'New application',
  'Novo projeto': 'New project',
  'Adicionar usuário': 'Add user',
  'Criar usuário': 'Create user',
  'Criar aplicação': 'Create application',
  'Senha inicial': 'Initial password',
  'Crie a identidade com uma senha inicial segura.':
    'Create the identity with a secure initial password.',
  'Registre o cliente OIDC e seus endereços autorizados.':
    'Register the OIDC client and its authorized addresses.',
  'Use pelo menos 12 caracteres. O usuário poderá configurar TOTP após entrar.':
    'Use at least 12 characters. The user can configure TOTP after signing in.',
  'Autenticação forte confirmada.': 'Strong authentication confirmed.',
  'Digite novamente a senha inicial para criar o usuário.':
    'Enter the initial password again to create the user.',
  'Nova atribuição': 'New assignment',
  'Configurar provedor': 'Configure provider',
  'Configurar projeto': 'Configure project',
  'Criado em': 'Created at',
  'Atualizado em': 'Updated at',
  Status: 'Status',
  Projeto: 'Project',
  Aplicação: 'Application',
  Função: 'Role',
  Evento: 'Event',
  Ator: 'Actor',
  Data: 'Date',
  Versão: 'Version',
  Documento: 'Document',
  Assunto: 'Subject',
  Remetente: 'Sender',
  Entrega: 'Delivery',
  'Limite mensal': 'Monthly limit',
  'Domínios permitidos': 'Allowed domains',
  Provedor: 'Provider',
  Enviados: 'Sent',
  Entregues: 'Delivered',
  Falhas: 'Failures',
  Período: 'Period',
  Total: 'Total',
  Pendentes: 'Pending',
  'Configuração salva.': 'Configuration saved.',
  'Não foi possível salvar a configuração.':
    'The configuration could not be saved.',
  'Use somente hostnames separados por vírgula, sem http:// ou porta. Para o MailTester local, inclua localhost.':
    'Use comma-separated hostnames only, without http:// or a port. Include localhost for the local MailTester.',
  'Salvando…': 'Saving…',
  Salvar: 'Save',
  'Excluir projeto': 'Delete project',
  'A exclusão ficará disponível quando o serviço publicar o contrato administrativo necessário.':
    'Deletion will become available when the service publishes the required administrative contract.',
  'Gerencie as credenciais e o estado deste projeto OAuth.':
    'Manage this OAuth project’s credentials and status.',
  'Configuração do projeto': 'Project configuration',
  'Credencial rotacionada.': 'Credential rotated.',
  'Copie a nova credencial exibida na resposta segura.':
    'Copy the new credential from the secure response.',
  Ativo: 'Active',
  Inativo: 'Inactive',
  'Desativar projeto?': 'Disable project?',
  'Ativar projeto?': 'Enable project?',
  'As integrações deste projeto deixarão de funcionar até ele ser reativado.':
    'This project’s integrations will stop working until it is enabled again.',
  'As integrações deste projeto voltarão a funcionar.':
    'This project’s integrations will start working again.',
  'Rotacionar chave?': 'Rotate key?',
  'A chave atual deixará de funcionar e a nova será exibida apenas uma vez.':
    'The current key will stop working and the new key will be shown only once.',
  'Informe um nome para identificar o projeto.':
    'Enter a name to identify the project.',
  Cancelar: 'Cancel',
  Confirmar: 'Confirm',
  'Criar projeto': 'Create project',
  'Projeto excluído.': 'Project deleted.',
  'Excluir projeto?': 'Delete project?',
  'Esta ação é permanente. Credenciais e configurações deste projeto serão removidas.':
    'This action is permanent. This project’s credentials and configuration will be removed.',
  'A exclusão remove permanentemente as credenciais e configurações deste projeto.':
    'Deletion permanently removes this project’s credentials and configuration.',
  Nome: 'Name',
  'E-mail': 'Email',
  Tipo: 'Type',
  Política: 'Policy',
  'Client ID': 'Client ID',
  Confidencial: 'Confidential',
  Pública: 'Public',
  Padrão: 'Standard',
  Forte: 'Strong',
  'URIs de redirecionamento': 'Redirect URIs',
  'URIs de redirecionamento após logout (opcional)':
    'Post-logout redirect URIs (optional)',
  Escopos: 'Scopes',
  Audiências: 'Audiences',
  'Claims (opcional)': 'Claims (optional)',
  Usuário: 'User',
  Identidade: 'Identity',
  Método: 'Method',
  Início: 'Started',
  Expira: 'Expires',
  Severidade: 'Severity',
  Métrica: 'Metric',
  Valor: 'Value',
  Janela: 'Window',
  Componente: 'Component',
  Formato: 'Format',
  Superadmin: 'Superadmin',
  Sim: 'Yes',
  Não: 'No',
  'Configuração da aplicação': 'Application configuration',
  'Edite o cliente OIDC, suas URLs e sua política de autenticação.':
    'Edit the OIDC client, its URLs, and authentication policy.',
  'Desativar aplicação?': 'Disable application?',
  'Ativar aplicação?': 'Enable application?',
  'Novos logins para esta aplicação serão bloqueados.':
    'New sign-ins to this application will be blocked.',
  'Novos logins para esta aplicação voltarão a ser permitidos.':
    'New sign-ins to this application will be allowed again.',
  'Rotacionar segredo': 'Rotate secret',
  'Rotacionar segredo?': 'Rotate secret?',
  'O segredo atual deixará de funcionar e o novo será exibido apenas uma vez.':
    'The current secret will stop working and the new one will be shown only once.',
  'Não foi possível carregar esta aplicação.':
    'This application could not be loaded.',
  'Informações da identidade': 'Identity information',
  'Consulte a identidade e encerre acessos ativos quando necessário.':
    'Review the identity and end active access when needed.',
  'Não foi possível carregar este usuário.': 'This user could not be loaded.',
  'Revogar sessões': 'Revoke sessions',
  'Sessões revogadas.': 'Sessions revoked.',
  'Revogar todas as sessões?': 'Revoke all sessions?',
  'O usuário precisará entrar novamente em todos os aplicativos.':
    'The user will need to sign in again to every application.',
  'Gerencie sua identidade e preferências locais deste navegador.':
    'Manage your identity and local preferences for this browser.',
  'Método de autenticação': 'Authentication method',
  'Nível de garantia': 'Assurance level',
  'Escolha a aparência da interface.': 'Choose the interface appearance.',
  'Ajuste o espaçamento dos painéis.': 'Adjust panel spacing.',
  'Contrato público de integração do MW OAuth.':
    'Public MW OAuth integration contract.',
  'Carregando dados…': 'Loading data…',
  'Não foi possível carregar a documentação.':
    'The documentation could not be loaded.',
  'Acesse identidades, OAuth e e-mail com sua conta MattediWorks.':
    'Access Identity, OAuth, and Mail with your MattediWorks account.',
  'Entrar com MattediWorks': 'Sign in with MattediWorks',
  'Primeiro acesso ou configurar TOTP': 'First access or configure TOTP',
  'Senha, passkey e TOTP são verificados com segurança no MW Identity.':
    'Password, passkey, and TOTP are securely verified by MW Identity.',
  'Controle claro. Confiança por padrão.': 'Clear control. Trust by default.',
  'Uma visão central para identidades, autorização e comunicação em todos os produtos MattediWorks.':
    'A central view for identity, authorization, and communication across MattediWorks products.',
  'Resumo da plataforma': 'Platform summary',
  'Módulos online': 'Online modules',
  'Verificação automática a cada minuto': 'Automatic check every minute',
  'Autorização centralizada': 'Centralized authorization',
  Sessão: 'Session',
  'Tokens mantidos no BFF': 'Tokens kept in the BFF',
  Protegida: 'Protected',
  Plano: 'Plane',
  'Aplicações MattediWorks': 'MattediWorks applications',
  'Espaços de trabalho': 'Workspaces',
  'Módulos do control plane': 'Control plane modules',
  'Dados atualizados em tempo real': 'Data updated in real time',
  'Pessoas, aplicações e políticas de acesso.':
    'People, applications, and access policies.',
  'Projetos, provedores e credenciais.':
    'Projects, providers, and credentials.',
  'Entrega, projetos e consumo de e-mail.':
    'Delivery, projects, and email usage.',
  Conectado: 'Connected',
  Verificando: 'Checking',
  Indisponível: 'Unavailable',
  'Operações sensíveis exigem autenticação forte recente. Tokens e segredos permanecem no BFF e nunca são expostos ao navegador.':
    'Sensitive operations require recent strong authentication. Tokens and secrets remain in the BFF and are never exposed to the browser.',
  'Identidades, estado de acesso e postura de autenticação em todos os aplicativos MW.':
    'Identities, access state, and authentication posture across all MW applications.',
  'Clientes OIDC, URIs exatas, audiências e políticas de autenticação.':
    'OIDC clients, exact URIs, audiences, and authentication policies.',
  'Controle os vínculos entre pessoas, aplicações e funções de acesso.':
    'Control the links between people, applications, and access roles.',
  'Visibilidade e revogação de sessões ativas no ecossistema MattediWorks.':
    'Visibility and revocation of active sessions across MattediWorks.',
  'Linha do tempo imutável de autenticação e alterações administrativas.':
    'Immutable timeline of authentication and administrative changes.',
  'Projetos consumidores, credenciais e limites do serviço de autorização.':
    'Consumer projects, credentials, and authorization service limits.',
  'Políticas de acesso do Google e configuração por projeto.':
    'Google access policies and per-project configuration.',
  'Operações federadas, rota, escopo autorizado e ator humano preservado.':
    'Federated operations with route, authorized scope, and preserved human actor.',
  'Saúde, volume de autorizações e comportamento operacional.':
    'Health, authorization volume, and operational behavior.',
  'Estado do runtime, endpoints públicos e documentação versionada.':
    'Runtime status, public endpoints, and versioned documentation.',
  'Contratos públicos e guias de integração para consumidores do MW OAuth.':
    'Public contracts and integration guides for MW OAuth consumers.',
  'Contratos públicos e guias de integração para consumidores do MW Mail.':
    'Public contracts and integration guides for MW Mail consumers.',
  'Cliente público: use o Client ID com Authorization Code + PKCE. Este tipo de cliente não possui segredo e não requer rotação de credenciais.':
    'Public client: use the Client ID with Authorization Code + PKCE. This client type has no secret and requires no credential rotation.',
  'Projetos remetentes, chaves de API e domínios de requisição permitidos.':
    'Sender projects, API keys, and allowed request domains.',
  'Domínios de requisição, credenciais e políticas de entrega por projeto.':
    'Request domains, credentials, and per-project delivery policies.',
  'Inspeção de tentativas, entrega, bounce, supressão e reclamações.':
    'Inspect attempts, delivery, bounces, suppression, and complaints.',
  'Volume por projeto e leitura de consumo da capacidade mensal.':
    'Per-project volume and monthly capacity usage.',
  'Alterações administrativas e operações federadas do serviço de e-mail.':
    'Administrative changes and federated operations for the Mail service.',
  'Nenhum usuário encontrado': 'No users found',
  'As identidades cadastradas aparecerão aqui.':
    'Registered identities will appear here.',
  'Nenhuma aplicação registrada': 'No applications registered',
  'Registre um cliente OIDC para começar.':
    'Register an OIDC client to get started.',
  'Selecione uma aplicação': 'Select an application',
  'As atribuições são carregadas no contexto de cada aplicação.':
    'Assignments are loaded in each application context.',
  'Nenhuma sessão para exibir': 'No sessions to display',
  'Use a página de usuários para revogar todas as sessões de uma identidade.':
    'Use the users page to revoke every session for an identity.',
  'Nenhum evento registrado': 'No events recorded',
  'Eventos de segurança e administração aparecerão aqui.':
    'Security and administration events will appear here.',
  'Nenhum projeto OAuth': 'No OAuth projects',
  'Crie um projeto para emitir uma chave de integração.':
    'Create a project to issue an integration key.',
  'Selecione um projeto OAuth': 'Select an OAuth project',
  'A política do provedor é gerenciada dentro de cada projeto.':
    'The provider policy is managed within each project.',
  'Nenhum evento OAuth': 'No OAuth events',
  'As operações administrativas aparecerão aqui.':
    'Administrative operations will appear here.',
  'Métricas aguardando amostras': 'Metrics awaiting samples',
  'Os indicadores serão preenchidos pelo serviço OAuth.':
    'Indicators will be populated by the OAuth service.',
  'Servidor sem metadados': 'Server metadata unavailable',
  'Conecte o módulo OAuth para visualizar seus detalhes.':
    'Connect the OAuth module to view its details.',
  'Nenhuma versão publicada': 'No published versions',
  'A documentação publicada pelo serviço aparecerá aqui.':
    'Documentation published by the service will appear here.',
  'Nenhum projeto de e-mail': 'No Mail projects',
  'Crie um projeto para começar a enviar mensagens.':
    'Create a project to start sending messages.',
  'Selecione um projeto de e-mail': 'Select a Mail project',
  'A configuração é aplicada de forma isolada por projeto.':
    'Configuration is applied separately to each project.',
  'Nenhuma mensagem encontrada': 'No messages found',
  'As tentativas recentes aparecerão aqui.':
    'Recent attempts will appear here.',
  'Uso ainda não calculado': 'Usage not calculated yet',
  'Os totais serão exibidos após os primeiros envios.':
    'Totals will appear after the first sends.',
  'Auditoria indisponível': 'Audit unavailable',
  'O contrato atual do Mail ainda não publica uma rota de auditoria.':
    'The current Mail contract does not yet expose an audit route.',
}

function loadPreferences(): Preferences {
  if (typeof window === 'undefined') return defaults
  try {
    const parsed = JSON.parse(
      localStorage.getItem(storageKey) ?? '{}',
    ) as Partial<Preferences>
    return {
      language: parsed.language === 'en' ? 'en' : defaults.language,
      theme: parsed.theme === 'dark' ? 'dark' : defaults.theme,
      density: parsed.density === 'compact' ? 'compact' : 'comfortable',
      groupOpen: { ...defaults.groupOpen, ...(parsed.groupOpen ?? {}) },
    }
  } catch {
    return defaults
  }
}

const PreferencesContext = createContext<PreferencesContextValue | null>(null)

export function PreferencesProvider({ children }: { children: ReactNode }) {
  const [preferences, setPreferences] = useState(loadPreferences)

  useEffect(() => {
    localStorage.setItem(storageKey, JSON.stringify(preferences))
    document.documentElement.lang =
      preferences.language === 'pt' ? 'pt-BR' : 'en'
    document.documentElement.dataset.density = preferences.density
    applyTheme(preferences.theme, true)
  }, [preferences])

  const value = useMemo<PreferencesContextValue>(
    () => ({
      ...preferences,
      setLanguage: (language) =>
        setPreferences((current) => ({ ...current, language })),
      setTheme: (theme) => setPreferences((current) => ({ ...current, theme })),
      setDensity: (density) =>
        setPreferences((current) => ({ ...current, density })),
      toggleGroup: (group) =>
        setPreferences((current) => ({
          ...current,
          groupOpen: {
            ...current.groupOpen,
            [group]: current.groupOpen[group] === false,
          },
        })),
      tr: (text) =>
        preferences.language === 'en' ? (english[text] ?? text) : text,
    }),
    [preferences],
  )

  return (
    <PreferencesContext.Provider value={value}>
      {children}
    </PreferencesContext.Provider>
  )
}

export function usePreferences() {
  const context = useContext(PreferencesContext)
  if (!context)
    throw new Error('usePreferences must be used inside PreferencesProvider')
  return context
}
