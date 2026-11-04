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
  Credenciais: 'Credentials',
  Atividade: 'Activity',
  'Carregando…': 'Loading…',
  'Tentar novamente': 'Try again',
  'Configuração e atividade deste projeto OAuth.':
    'Configuration and activity for this OAuth project.',
  'Navegação do projeto': 'Project navigation',
  Localização: 'Location',
  'Não foi possível carregar o projeto.': 'Could not load the project.',
  'Projeto não encontrado': 'Project not found',
  'Escolha um projeto para configurar seus provedores.':
    'Choose a project to configure its providers.',
  'Voltar aos projetos': 'Back to projects',
  'Google ativo': 'Google enabled',
  'Google inativo': 'Google disabled',
  'Estado indisponível': 'Status unavailable',
  'Atividade recente': 'Recent activity',
  'Últimos 50 eventos deste projeto.': 'Latest 50 events for this project.',
  'Não foi possível carregar a atividade.': 'Could not load activity.',
  'As operações deste projeto aparecerão aqui.':
    'Operations for this project will appear here.',
  'Políticas de autorização deste projeto.':
    'Authorization policies for this project.',
  'Gerencie as credenciais de integração deste projeto. Novos segredos são exibidos apenas uma vez.':
    'Manage integration credentials for this project. New secrets are shown only once.',
  'Chave de API': 'API key',
  'Autentica as chamadas deste projeto ao serviço OAuth.':
    'Authenticates this project’s calls to the OAuth service.',
  'Segredo de callback': 'Callback secret',
  'Valida os callbacks assinados enviados a este projeto.':
    'Validates signed callbacks sent to this project.',
  'Políticas do Google OAuth para {project}.':
    'Google OAuth policies for {project}.',
  'Salvar alterações para {project}': 'Save changes for {project}',
  'Funções de acesso': 'Access roles',
  'Nova função': 'New role',
  'Gerenciar funções': 'Manage roles',
  'Funções definem o que cada cliente pode publicar e assinar. Crie, edite e atribua funções aqui; as funções do próprio broker são reservadas.':
    "Roles define what each client may publish and subscribe to. Create, edit and assign roles here; the broker's own roles are reserved.",
  'Cadastradas no broker': 'Registered in the broker',
  Atribuíveis: 'Assignable',
  'Podem ser dadas a clientes': 'Can be given to clients',
  Reservadas: 'Reserved',
  'Administração do broker': 'Broker administration',
  Restrita: 'Restricted',
  Descrição: 'Description',
  Regras: 'Rules',
  Detalhes: 'Details',
  'Crie uma função para definir o que um cliente pode fazer.':
    'Create a role to define what a client may do.',
  'Nenhuma função': 'No roles',
  'Criar, editar e excluir funções, e dá-las a clientes, exigem autenticação forte recente. Nenhuma regra pode conceder o canal de controle do broker ($CONTROL).':
    "Creating, editing and deleting roles, and giving them to clients, require recent strong authentication. No rule can grant the broker's control channel ($CONTROL).",
  'Excluir função?': 'Delete role?',
  'Esta função está em uso por {n} cliente. Ele perderá imediatamente o acesso que ela concede.':
    'This role is used by {n} client. It will immediately lose the access the role grants.',
  'Esta função está em uso por {n} clientes. Eles perderão imediatamente o acesso que ela concede.':
    'This role is used by {n} clients. They will immediately lose the access the role grants.',
  'Nenhum cliente usa esta função. Esta ação não pode ser desfeita.':
    'No client uses this role. This cannot be undone.',
  'Escolha um ponto de partida. As regras podem ser editadas depois.':
    'Pick a starting point. The rules can be edited afterwards.',
  'Nome da função': 'Role name',
  'Descrição (opcional)': 'Description (optional)',
  'Use letras, números, ponto, hífen ou sublinhado (até 64 caracteres).':
    'Use letters, numbers, dot, hyphen or underscore (up to 64 characters).',
  'Já existe uma função com este nome.':
    'A role with this name already exists.',
  'Ponto de partida': 'Starting point',
  'Regras iniciais': 'Initial rules',
  'Sem regras: o cliente não poderá fazer nada até você adicionar.':
    'No rules: a client can do nothing until you add some.',
  'Criar função': 'Create role',
  'Em branco': 'Blank',
  'Sem regras. Adicione as regras depois.': 'No rules. Add them afterwards.',
  'Acesso total (confiável)': 'Full access (trusted)',
  'Publicar, assinar e receber em todos os tópicos. Use só para gateways e serviços confiáveis.':
    'Publish, subscribe and receive on every topic. Use only for trusted gateways and services.',
  'Somente leitura': 'Read only',
  'Assinar e receber em todos os tópicos, sem publicar.':
    'Subscribe and receive on every topic, without publishing.',
  'Isolado por cliente': 'Isolated per client',
  'Cada cliente só acessa devices/<seu usuário>/#. Um cliente não alcança os tópicos de outro.':
    "Each client only reaches devices/<its username>/#. One client cannot reach another's topics.",
  'Monitoramento do broker': 'Broker monitoring',
  'Ler as estatísticas do broker ($SYS), sem acesso a mensagens.':
    'Read the broker statistics ($SYS), with no access to messages.',
  'Regras que esta função concede.': 'Rules this role grants.',
  'Função do broker: não pode ser alterada aqui.':
    'Broker role: it cannot be changed here.',
  'Nenhum cliente usa esta função.': 'No client uses this role.',
  'Sem regras: nenhum acesso é concedido.': 'No rules: no access is granted.',
  Tópico: 'Topic',
  Efeito: 'Effect',
  Prioridade: 'Priority',
  Negar: 'Deny',
  'Remover regra': 'Remove rule',
  'Remover regra?': 'Remove rule?',
  'Adicionar regra': 'Add rule',
  'Filtro de tópico': 'Topic filter',
  '+ e # são curingas; %u e %c valem o usuário e o client ID do próprio cliente.':
    "+ and # are wildcards; %u and %c stand for the client's own username and client ID.",
  '{n} cliente usa esta função e perde esse acesso imediatamente.':
    '{n} client uses this role and loses that access immediately.',
  '{n} clientes usam esta função e perdem esse acesso imediatamente.':
    '{n} clients use this role and lose that access immediately.',
  Remover: 'Remove',
  Adicionar: 'Add',
  'Remover função?': 'Remove role?',
  'O cliente perde imediatamente o acesso que esta função concede.':
    'The client immediately loses the access this role grants.',
  'Funções do cliente': 'Client roles',
  'As funções definem o que o cliente pode publicar e assinar. Alterar exige autenticação forte recente.':
    'Roles define what the client may publish and subscribe to. Changing them requires recent strong authentication.',
  'Ainda não há funções que possam ser atribuídas.':
    'There are no roles that can be assigned yet.',
  'Criar uma função': 'Create a role',
  regras: 'rules',
  funções: 'roles',
  atribuíveis: 'assignable',
  'Criar, reativar, trocar senha, excluir e alterar funções exigem autenticação forte recente. Desativar é imediato. Clientes protegidos do broker não podem ser alterados aqui.':
    'Creating, re-enabling, changing the password, deleting and changing roles require recent strong authentication. Disabling is immediate. Protected broker clients cannot be changed here.',
  'As funções definem o que cada cliente pode publicar e assinar. Gerencie-as na página de funções.':
    'Roles define what each client may publish and subscribe to. Manage them on the roles page.',
  'O cliente publica': 'The client publishes',
  'O broker entrega ao cliente': 'The broker delivers to the client',
  'O cliente assina (filtro exato)': 'The client subscribes (exact filter)',
  'O cliente assina (filtros que combinam)':
    'The client subscribes (matching filters)',
  'O cliente remove assinatura (filtro exato)':
    'The client unsubscribes (exact filter)',
  'O cliente remove assinatura (filtros que combinam)':
    'The client unsubscribes (matching filters)',
  'Escolha um tipo de regra.': 'Choose a rule type.',
  'Filtro de tópico inválido: use + e # só como nível inteiro, e # apenas no fim.':
    'Invalid topic filter: use + and # only as a whole level, and # only at the end.',
  'A prioridade deve ser um número inteiro entre -1000 e 1000.':
    'The priority must be a whole number between -1000 and 1000.',
  'O canal de controle do broker ($CONTROL) não pode ser concedido a uma função.':
    "The broker's control channel ($CONTROL) cannot be granted to a role.",
  'Ninguém pode publicar em tópicos que começam com $. Para ler estatísticas, use assinar e receber.':
    'Nobody can publish to topics that start with $. To read statistics, use subscribe and receive.',
  'Esta função é reservada ao broker e não pode ser alterada aqui.':
    'This role is reserved to the broker and cannot be changed here.',
  'Esta função ainda está em uso por clientes.':
    'This role is still used by clients.',
  'A regra é inválida. Revise o tipo, o filtro de tópico e a prioridade.':
    'The rule is invalid. Review the type, topic filter and priority.',
  'Nenhuma função pode conceder o canal de controle do broker nem publicar em tópicos $.':
    "No role can grant the broker's control channel or publish to $ topics.",
  'Escolha a função do cliente.': "Choose the client's role.",
  'A descrição é longa demais ou tem caracteres inválidos.':
    'The description is too long or has invalid characters.',
  'Clientes, credenciais e estado do broker.':
    'Clients, credentials and broker status.',
  'Nova credencial': 'New credential',
  'Credenciais do projeto': 'Project credentials',
  Ativado: 'Enabled',
  Desativado: 'Disabled',
  Credencial: 'Credential',
  'Credencial gerada': 'Generated credential',
  Copiar: 'Copy',
  Copiado: 'Copied',
  'Copiar todas': 'Copy all',
  'Tudo copiado': 'All copied',
  Concluir: 'Done',
  Fechar: 'Close',
  'A operação foi concluída. Copie e armazene a credencial em um local seguro.':
    'The operation completed. Copy and store the credential in a safe place.',
  'A operação foi concluída. Copie e armazene as credenciais em um local seguro.':
    'The operation completed. Copy and store the credentials in a safe place.',
  'Esta credencial não será exibida novamente depois que esta janela for fechada.':
    'This credential will not be shown again after this window is closed.',
  'Estas credenciais não serão exibidas novamente depois que esta janela for fechada.':
    'These credentials will not be shown again after this window is closed.',
  'Sua sessão expirou. Entre novamente.':
    'Your session expired. Sign in again.',
  'Sua conta não tem permissão para esta operação.':
    'Your account does not have permission for this operation.',
  'Confirme sua identidade para continuar.':
    'Confirm your identity to continue.',
  'O serviço não está disponível no momento.':
    'The service is unavailable right now.',
  'Já existe uma identidade com este e-mail.':
    'An identity with this email already exists.',
  'Já existe uma aplicação com este Client ID.':
    'An application with this Client ID already exists.',
  'Revise os campos informados e tente novamente.':
    'Review the fields and try again.',
  'Arquive a aplicação antes de excluí-la permanentemente.':
    'Archive the application before deleting it permanently.',
  'Esta aplicação ainda possui usuários associados, códigos de autorização ou tokens emitidos.':
    'This application still has associated users, authorization codes or issued tokens.',
  'Não foi possível concluir a solicitação.':
    'The request could not be completed.',
  'Já existe um cliente MQTT com este nome.':
    'An MQTT client with this name already exists.',
  'O cliente MQTT não foi encontrado.': 'The MQTT client was not found.',
  'Este cliente é protegido e não pode ser alterado aqui.':
    'This client is protected and cannot be changed here.',
  'Esta função não pode ser atribuída por aqui.':
    'This role cannot be assigned here.',
  'Muitas tentativas recusadas. Aguarde um minuto e tente novamente.':
    'Too many refused attempts. Wait a minute and try again.',
  'O serviço MQTT recusou as credenciais do control plane.':
    'The MQTT service rejected the control plane credentials.',
  'Não foi possível obter autorização para o serviço MQTT. Saia e entre novamente.':
    'Could not get authorization for the MQTT service. Sign out and sign in again.',
  'O serviço MQTT não está disponível.': 'The MQTT service is unavailable.',
  'O serviço MQTT falhou ao processar a solicitação.':
    'The MQTT service failed to process the request.',
  'Guia de conexão e contrato de integração do serviço MQTT.':
    'Connection guide and integration contract for the MQTT service.',
  'Contrato público de integração do MW Mail.':
    'Public integration contract for MW Mail.',
  Clientes: 'Clients',
  Broker: 'Broker',
  'Novo cliente': 'New client',
  'Clientes MQTT': 'MQTT clients',
  'Usuários do broker MQTT: gateways, dispositivos e serviços. A senha é gerada automaticamente e exibida uma única vez.':
    'MQTT broker users: gateways, devices and services. The password is generated automatically and shown only once.',
  'Cadastrados no broker': 'Registered in the broker',
  Conectados: 'Connected',
  'Sessões abertas agora': 'Sessions open right now',
  Assinaturas: 'Subscriptions',
  'Ativas no broker': 'Active on the broker',
  'Mensagens retidas': 'Retained messages',
  'Estado guardado pelo broker': 'State kept by the broker',
  Cliente: 'Client',
  Funções: 'Roles',
  'Crie um cliente para um gateway, dispositivo ou serviço.':
    'Create a client for a gateway, device or service.',
  'Nenhum cliente MQTT': 'No MQTT clients',
  Protegido: 'Protected',
  'Ativar cliente?': 'Enable client?',
  'Desativar cliente?': 'Disable client?',
  'O cliente poderá se conectar novamente com a senha atual.':
    'The client will be able to connect again with its current password.',
  'O cliente será desconectado e não poderá se conectar até ser reativado.':
    'The client will be disconnected and cannot connect until re-enabled.',
  'Gerar nova senha?': 'Generate a new password?',
  'Gerar nova senha': 'Generate new password',
  'Nova senha': 'New password',
  'A senha atual deixará de funcionar e a nova será exibida apenas uma vez.':
    'The current password will stop working and the new one is shown only once.',
  'Excluir cliente?': 'Delete client?',
  Excluir: 'Delete',
  'Criar cliente': 'Create client',
  'O cliente será desconectado e removido do broker. Esta ação não pode ser desfeita.':
    'The client will be disconnected and removed from the broker. This cannot be undone.',
  'Criar, reativar, trocar senha e excluir exigem autenticação forte recente. Desativar é imediato. Clientes protegidos do broker não podem ser alterados aqui.':
    'Creating, re-enabling, changing the password and deleting require recent strong authentication. Disabling is immediate. Protected broker clients cannot be changed here.',
  'Informe o nome do cliente. A senha será gerada e exibida uma única vez.':
    'Enter the client name. The password will be generated and shown only once.',
  'Nome do cliente': 'Client name',
  'Função atribuída:': 'Assigned role:',
  'Use letras, números, ponto, hífen ou sublinhado (até 128 caracteres).':
    'Use letters, numbers, dot, hyphen or underscore (up to 128 characters).',
  'Credenciais MQTT': 'MQTT credentials',
  'O broker MQTT não está acessível no momento.':
    'The MQTT broker is not reachable right now.',
  'Não foi possível carregar os clientes.': 'Could not load the clients.',
  'Estado do broker MQTT: conexões, tráfego e funções de acesso. Os números vêm do próprio broker e são atualizados a cada 10 segundos.':
    'MQTT broker state: connections, traffic and access roles. The numbers come from the broker itself and refresh every 10 seconds.',
  'Broker MQTT': 'MQTT broker',
  'Conexão do serviço de administração': 'Admin service connection',
  'Tempo ativo': 'Uptime',
  'Desde a última reinicialização': 'Since the last restart',
  Atualizado: 'Updated',
  'Dados desatualizados': 'Stale data',
  'Leitura mais recente': 'Latest reading',
  'Sem atualização recente do broker': 'No recent update from the broker',
  'Não foi possível carregar as estatísticas.':
    'Could not load the statistics.',
  'O broker ainda não publicou estatísticas. Elas aparecem em até 10 segundos após a conexão.':
    'The broker has not published statistics yet. They appear within 10 seconds of connecting.',
  'Conexões e armazenamento': 'Connections and storage',
  'Clientes conectados': 'Connected clients',
  'Clientes (total)': 'Clients (total)',
  'Máximo de clientes': 'Peak clients',
  'Mensagens armazenadas': 'Stored messages',
  'Memória em uso': 'Memory in use',
  'Pico de memória': 'Peak memory',
  'Tráfego acumulado': 'Cumulative traffic',
  'Mensagens recebidas': 'Messages received',
  'Mensagens enviadas': 'Messages sent',
  'Publicações recebidas': 'Publishes received',
  'Publicações enviadas': 'Publishes sent',
  'Publicações descartadas': 'Publishes dropped',
  'Bytes recebidos': 'Bytes received',
  'Bytes enviados': 'Bytes sent',
  Conexões: 'Connections',
  'Carga média': 'Load average',
  'Funções de acesso (somente leitura)': 'Access roles (read-only)',
  'Não foi possível carregar as funções.': 'Could not load the roles.',
  Atribuível: 'Assignable',
  Reservada: 'Reserved',
  'As funções definem o que cada cliente pode publicar e assinar. Elas são mantidas no broker e não podem ser alteradas por esta página.':
    'Roles define what each client may publish and subscribe to. They are kept in the broker and cannot be changed from this page.',
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
  'Configurar Google': 'Configure Google',
  'Configure políticas e limites dos provedores OAuth por projeto.':
    'Configure OAuth provider policies and limits per project.',
  'Cada projeto possui sua própria política de provedor.':
    'Each project has its own provider policy.',
  'O Client ID e o segredo do Google são credenciais globais do servidor MW OAuth e não são expostos aqui. Esta página controla somente a política de cada projeto.':
    'The Google client ID and secret are global MW OAuth server credentials and are not exposed here. This page controls only each project’s policy.',
  'Ativar Google': 'Enable Google',
  'Quando desativado, novas autorizações deste projeto serão recusadas.':
    'When disabled, new authorizations for this project are rejected.',
  'Escopos permitidos': 'Allowed scopes',
  'Um escopo por linha ou separado por vírgula.':
    'Use one scope per line or separate scopes with commas.',
  'URLs de callback permitidas': 'Allowed callback URLs',
  'Endpoints backend que receberão o POST assinado do MW OAuth. Correspondência exata.':
    'Backend endpoints that receive the signed MW OAuth POST. Exact matching applies.',
  'URLs de retorno permitidas': 'Allowed return URLs',
  'Páginas para onde o usuário poderá retornar após a integração. Correspondência exata.':
    'Pages users may return to after integration. Exact matching applies.',
  Limites: 'Limits',
  'Por minuto': 'Per minute',
  'Por hora': 'Per hour',
  'Máximo pendente': 'Maximum pending',
  'Salvar configuração': 'Save configuration',
  'Configuração do provedor salva.': 'Provider configuration saved.',
  'Não foi possível carregar a configuração do Google.':
    'The Google configuration could not be loaded.',
  'Revise a configuração do provedor.': 'Review the provider configuration.',
  'Os limites devem ser números inteiros positivos.':
    'Limits must be positive integers.',
  'Use URLs HTTPS absolutas, exatas e sem query string ou fragmento. HTTP é permitido apenas para loopback local.':
    'Use exact absolute HTTPS URLs without query strings or fragments. HTTP is allowed only for local loopback.',
  'Informe pelo menos um escopo antes de ativar o Google.':
    'Add at least one scope before enabling Google.',
  'Informe pelo menos uma URL de callback antes de ativar o Google.':
    'Add at least one callback URL before enabling Google.',
  'Informe pelo menos uma URL de retorno antes de ativar o Google.':
    'Add at least one return URL before enabling Google.',
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
  'Cadastro público': 'Public registration',
  'Callback de exclusão de conta': 'Account deletion callback',
  'Callback de exclusão de conta (obrigatório)':
    'Account deletion callback (required)',
  'Callback de exclusão de conta (opcional)':
    'Account deletion callback (optional)',
  Uso: 'Usage',
  'Para usuários': 'User-facing',
  'Interna (MW BFF)': 'Internal (MW BFF)',
  Exclusão: 'Deletion',
  Interna: 'Internal',
  'Callback pendente': 'Callback missing',
  'Callback ativo': 'Callback set',
  'Aplicações para usuários precisam de um callback de exclusão de conta.':
    'User-facing applications need an account deletion callback.',
  'Esta aplicação não tem callback de exclusão de conta. Ela continua funcionando, mas a próxima edição exige um callback, ou que ela seja marcada como interna.':
    'This application has no account deletion callback. It keeps working, but its next edit needs a callback, or marking it internal.',
  'Tipo de conta': 'Account type',
  'Conta pública': 'Public account',
  'Conta corporativa': 'Workforce account',
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
  'Excluir conta': 'Delete account',
  'Excluir conta?': 'Delete account?',
  'O acesso será revogado imediatamente e a exclusão será enviada aos aplicativos conectados.':
    'Access will be revoked immediately and the deletion will be sent to connected applications.',
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
  'Política de acesso': 'Access policy',
  'Negar por padrão': 'Deny by default',
  'Permitir por padrão': 'Allow by default',
  'Exceções de acesso': 'Access exceptions',
  'As exceções substituem a política padrão. Superadmins e administradores não podem ser bloqueados.':
    'Exceptions override the default policy. Superadmins and administrators cannot be blocked.',
  'As exceções substituem a política padrão, inclusive para superadmins e administradores.':
    'Exceptions override the default policy, including for superadmins and administrators.',
  'Selecione um usuário': 'Select a user',
  Permitir: 'Allow',
  Bloquear: 'Block',
  Permitido: 'Allowed',
  Bloqueado: 'Blocked',
  'Salvar exceção': 'Save exception',
  'Usar padrão': 'Use default',
  'Nenhuma exceção configurada.': 'No access exceptions configured.',
  'Exceção de acesso atualizada.': 'Access exception updated.',
  'Não foi possível atualizar a exceção de acesso.':
    'Could not update the access exception.',
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

/** Like usePreferences, but returns null outside a provider (shared components, tests). */
export function useOptionalPreferences() {
  return useContext(PreferencesContext)
}

export function usePreferences() {
  const context = useContext(PreferencesContext)
  if (!context)
    throw new Error('usePreferences must be used inside PreferencesProvider')
  return context
}
