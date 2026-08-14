// src/lib/api.ts

const configuredAdminApiBase =
  import.meta.env.VITE_ADMIN_API_BASE_URL ||
  'https://mailgo.mattediworks.com/admin/api'

function normalizeAdminApiBase(value: string): string {
  let url: URL

  try {
    url = new URL(value)
  } catch {
    throw new Error('VITE_ADMIN_API_BASE_URL deve ser uma URL absoluta válida.')
  }

  if (url.protocol !== 'http:' && url.protocol !== 'https:') {
    throw new Error('VITE_ADMIN_API_BASE_URL deve usar HTTP ou HTTPS.')
  }

  if (url.username || url.password || url.search || url.hash) {
    throw new Error(
      'VITE_ADMIN_API_BASE_URL não deve conter credenciais, query ou hash.',
    )
  }

  url.pathname = url.pathname.replace(/\/+$/, '')

  if (!url.pathname.endsWith('/api') && !url.pathname.endsWith('/admin/api')) {
    throw new Error('VITE_ADMIN_API_BASE_URL deve terminar em /api.')
  }

  return url.toString().replace(/\/$/, '')
}

export const adminApiBaseUrl = normalizeAdminApiBase(configuredAdminApiBase)

export const serviceBaseUrl = new URL('/', adminApiBaseUrl).origin

// -----------------------------------------------------------------------------
// Types
// -----------------------------------------------------------------------------

export interface PageMeta {
  currentPage: number
  perPage: number
  total: number
  lastPage: number
}

export interface AuthUser {
  id: number
  email: string
  enabled: boolean
  lastLoginAt: string | null
  createdAt: string
  updatedAt: string
}

export interface ProjectRecord {
  id: number
  name: string
  enabled: boolean
  monthlyLimit: number | null
  createdAt: string
  updatedAt: string
}

export type EmailSendStatus = 'pending' | 'sent' | 'failed'

export type EmailDeliveryStatus =
  | 'sent'
  | 'delivered'
  | 'delivery_delayed'
  | 'bounced'
  | 'failed'
  | 'suppressed'
  | 'complained'

export interface EmailAttemptRecord {
  id: number
  projectId: number
  projectName: string

  provider: string
  providerId: string | null

  sender: string
  subject: string
  recipientCount: number

  status: EmailSendStatus
  errorMessage: string | null

  deliveryStatus: EmailDeliveryStatus | null
  deliveryStatusAt: string | null

  createdAt: string
  updatedAt: string
}

export interface EmailEventRecord {
  id: number

  provider: string
  providerEventId: string
  providerEmailId: string | null

  eventType: string

  occurredAt: string
  receivedAt: string
}

export interface EmailAttemptDetails extends EmailAttemptRecord {
  events: EmailEventRecord[]
}

export interface CreateProjectInput {
  name: string
  monthlyLimit?: number | null
}

export interface UpdateProjectInput {
  name?: string
  enabled?: boolean
  monthlyLimit?: number
  clearMonthlyLimit?: boolean
}

export interface EmailListParams {
  projectId?: number
  status?: EmailSendStatus
  deliveryStatus?: EmailDeliveryStatus
  limit?: number
  offset?: number
}

export interface EmailListResult {
  emails: EmailAttemptRecord[]
  limit: number
  offset: number
}

// -----------------------------------------------------------------------------
// Wire types
// -----------------------------------------------------------------------------
//
// Keep the backend's snake_case representation private to api.ts.
// The rest of the React application gets camelCase objects.
//

interface WireAuthUser {
  id: number
  email: string
  enabled: boolean
  last_login_at?: string | null
  created_at: string
  updated_at: string
}

interface WireProject {
  id: number
  name: string
  enabled: boolean
  monthly_limit: number | null
  created_at: string
  updated_at: string
}

interface WireEmailAttempt {
  id: number
  project_id: number
  project_name: string

  provider: string
  provider_id?: string

  sender: string
  subject: string
  recipient_count: number

  status: EmailSendStatus
  error_message?: string

  delivery_status?: EmailDeliveryStatus
  delivery_status_at?: string | null

  created_at: string
  updated_at: string
}

interface WireEmailEvent {
  id: number

  provider: string
  provider_event_id: string
  provider_email_id?: string

  event_type: string

  occurred_at: string
  received_at: string
}

interface WireEmailAttemptDetails extends WireEmailAttempt {
  events: WireEmailEvent[]
}

// -----------------------------------------------------------------------------
// Errors
// -----------------------------------------------------------------------------

interface ApiErrorBody {
  error?: string
  message?: string
  code?: string
  errors?: Record<string, string[]>
}

export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly errors: Record<string, string[]>

  constructor(
    message: string,
    status: number,
    code: string,
    errors: Record<string, string[]> = {},
  ) {
    super(message)

    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.errors = errors
  }
}

function errorCodeFromStatus(status: number): string {
  switch (status) {
    case 400:
      return 'BAD_REQUEST'

    case 401:
      return 'UNAUTHORIZED'

    case 403:
      return 'FORBIDDEN'

    case 404:
      return 'NOT_FOUND'

    case 409:
      return 'CONFLICT'

    case 429:
      return 'RATE_LIMITED'

    default:
      return status >= 500 ? 'SERVER_ERROR' : 'API_ERROR'
  }
}

async function parseResponse<T>(response: Response): Promise<T> {
  if (response.status === 204) {
    return undefined as T
  }

  const text = await response.text()

  let body: unknown = {}

  if (text) {
    try {
      body = JSON.parse(text)
    } catch {
      body = {}
    }
  }

  if (!response.ok) {
    const error = body as ApiErrorBody

    const message =
      error.error ||
      error.message ||
      (() => {
        switch (response.status) {
          case 400:
            return 'Solicitação inválida.'

          case 401:
            return 'Sessão inválida ou expirada.'

          case 403:
            return 'Acesso negado.'

          case 404:
            return 'Recurso não encontrado.'

          case 409:
            return 'Conflito ao processar a solicitação.'

          case 429:
            return 'Muitas solicitações. Tente novamente em instantes.'

          default:
            return 'Não foi possível concluir a solicitação.'
        }
      })()

    throw new ApiError(
      message,
      response.status,
      error.code || errorCodeFromStatus(response.status),
      error.errors || {},
    )
  }

  return body as T
}

// -----------------------------------------------------------------------------
// Mappers
// -----------------------------------------------------------------------------

function mapUser(user: WireAuthUser): AuthUser {
  return {
    id: user.id,
    email: user.email,
    enabled: user.enabled,
    lastLoginAt: user.last_login_at ?? null,
    createdAt: user.created_at,
    updatedAt: user.updated_at,
  }
}

function mapProject(project: WireProject): ProjectRecord {
  return {
    id: project.id,
    name: project.name,
    enabled: project.enabled,
    monthlyLimit: project.monthly_limit,
    createdAt: project.created_at,
    updatedAt: project.updated_at,
  }
}

function mapEmailAttempt(email: WireEmailAttempt): EmailAttemptRecord {
  return {
    id: email.id,

    projectId: email.project_id,
    projectName: email.project_name,

    provider: email.provider,
    providerId: email.provider_id ?? null,

    sender: email.sender,
    subject: email.subject,
    recipientCount: email.recipient_count,

    status: email.status,
    errorMessage: email.error_message ?? null,

    deliveryStatus: email.delivery_status ?? null,
    deliveryStatusAt: email.delivery_status_at ?? null,

    createdAt: email.created_at,
    updatedAt: email.updated_at,
  }
}

function mapEmailEvent(event: WireEmailEvent): EmailEventRecord {
  return {
    id: event.id,

    provider: event.provider,
    providerEventId: event.provider_event_id,
    providerEmailId: event.provider_email_id ?? null,

    eventType: event.event_type,

    occurredAt: event.occurred_at,
    receivedAt: event.received_at,
  }
}

// -----------------------------------------------------------------------------
// Session / CSRF state
// -----------------------------------------------------------------------------

let csrfToken: string | null = null
let csrfInFlight: Promise<string> | null = null

export function clearCredentials(): void {
  csrfToken = null
  csrfInFlight = null
}

// -----------------------------------------------------------------------------
// Core request
// -----------------------------------------------------------------------------

interface AdminRequestOptions {
  skipCsrf?: boolean
}

function isMutation(method: string): boolean {
  return !['GET', 'HEAD', 'OPTIONS'].includes(method.toUpperCase())
}

async function request<T>(
  path: string,
  init: RequestInit = {},
  options: AdminRequestOptions = {},
): Promise<T> {
  const method = (init.method ?? 'GET').toUpperCase()

  const headers = new Headers(init.headers)

  headers.set('Accept', 'application/json')

  if (
    init.body !== undefined &&
    !(init.body instanceof FormData) &&
    !headers.has('Content-Type')
  ) {
    headers.set('Content-Type', 'application/json')
  }

  if (isMutation(method) && !options.skipCsrf) {
    const csrf = await ensureCsrf()

    headers.set('X-CSRF-Token', csrf)
  }

  const response = await fetch(`${adminApiBaseUrl}${path}`, {
    ...init,
    method,
    headers,

    // Required because the session cookie belongs to
    // mailgo.mattediworks.com while the SPA is on
    // another origin.
    credentials: 'include',
  })

  if (response.status === 401) {
    clearCredentials()
  }

  return parseResponse<T>(response)
}

// -----------------------------------------------------------------------------
// Authentication
// -----------------------------------------------------------------------------

interface LoginResponse {
  user: WireAuthUser
  csrf_token: string
}

interface MeResponse {
  user: WireAuthUser
}

interface CsrfResponse {
  csrf_token: string
}

export async function login(
  email: string,
  password: string,
): Promise<AuthUser> {
  const body = await request<LoginResponse>(
    '/auth/login',
    {
      method: 'POST',
      body: JSON.stringify({
        email,
        password,
      }),
    },
    {
      // There is no authenticated session/CSRF token
      // before login.
      skipCsrf: true,
    },
  )

  csrfToken = body.csrf_token

  return mapUser(body.user)
}

export async function getCurrentUser(): Promise<AuthUser> {
  const body = await request<MeResponse>('/auth/me')

  return mapUser(body.user)
}

export async function refreshCsrfToken(): Promise<string> {
  if (csrfInFlight !== null) {
    return csrfInFlight
  }

  csrfInFlight = request<CsrfResponse>(
    '/auth/csrf',
    {
      method: 'POST',
    },
    {
      // This endpoint exists specifically to issue a new
      // CSRF token for an already authenticated session.
      skipCsrf: true,
    },
  )
    .then((body) => {
      csrfToken = body.csrf_token

      return csrfToken
    })
    .finally(() => {
      csrfInFlight = null
    })

  return csrfInFlight
}

async function ensureCsrf(): Promise<string> {
  if (csrfToken !== null) {
    return csrfToken
  }

  return refreshCsrfToken()
}

export async function restoreSession(): Promise<AuthUser> {
  const user = await getCurrentUser()

  // React state is lost on reload, but the HttpOnly
  // session cookie survives. Re-establish an in-memory
  // CSRF token for subsequent mutations.
  await refreshCsrfToken()

  return user
}

export async function logout(): Promise<void> {
  try {
    await request<void>('/auth/logout', {
      method: 'POST',
    })
  } finally {
    clearCredentials()
  }
}

// -----------------------------------------------------------------------------
// Generic authenticated admin requests
// -----------------------------------------------------------------------------

export function adminRequest<T>(
  path: string,
  init: RequestInit = {},
): Promise<T> {
  return request<T>(path, init)
}

export function adminJsonRequest<T>(
  path: string,
  method: 'POST' | 'PUT' | 'PATCH' | 'DELETE',
  body: Record<string, unknown> = {},
): Promise<T> {
  return request<T>(path, {
    method,
    body: JSON.stringify(body),
  })
}

// -----------------------------------------------------------------------------
// Admin health
// -----------------------------------------------------------------------------

export function getAdminHealth(): Promise<{
  status: 'ok'
}> {
  return request('/health')
}

// -----------------------------------------------------------------------------
// Projects
// -----------------------------------------------------------------------------

export async function getProjects(): Promise<ProjectRecord[]> {
  const body = await request<{
    projects: WireProject[]
  }>('/projects')

  return body.projects.map(mapProject)
}

export async function createProject(input: CreateProjectInput): Promise<{
  project: ProjectRecord
  apiKey: string
}> {
  const body = await request<{
    project: WireProject
    api_key: string
  }>('/projects', {
    method: 'POST',
    body: JSON.stringify({
      name: input.name,
      monthly_limit: input.monthlyLimit ?? null,
    }),
  })

  return {
    project: mapProject(body.project),
    apiKey: body.api_key,
  }
}

export async function updateProject(
  id: number,
  input: UpdateProjectInput,
): Promise<ProjectRecord> {
  const payload: Record<string, unknown> = {}

  if (input.name !== undefined) {
    payload.name = input.name
  }

  if (input.enabled !== undefined) {
    payload.enabled = input.enabled
  }

  if (input.monthlyLimit !== undefined) {
    payload.monthly_limit = input.monthlyLimit
  }

  if (input.clearMonthlyLimit !== undefined) {
    payload.clear_monthly_limit = input.clearMonthlyLimit
  }

  const body = await request<{
    project: WireProject
  }>(`/projects/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(payload),
  })

  return mapProject(body.project)
}

export async function rotateProjectApiKey(id: number): Promise<string> {
  const body = await request<{
    api_key: string
  }>(`/projects/${id}/rotate-key`, {
    method: 'POST',
    body: JSON.stringify({}),
  })

  return body.api_key
}

// -----------------------------------------------------------------------------
// Emails
// -----------------------------------------------------------------------------

export async function getEmails(
  params: EmailListParams = {},
): Promise<EmailListResult> {
  const search = new URLSearchParams()

  if (params.projectId !== undefined) {
    search.set('project_id', String(params.projectId))
  }

  if (params.status !== undefined) {
    search.set('status', params.status)
  }

  if (params.deliveryStatus !== undefined) {
    search.set('delivery_status', params.deliveryStatus)
  }

  if (params.limit !== undefined) {
    search.set('limit', String(params.limit))
  }

  if (params.offset !== undefined) {
    search.set('offset', String(params.offset))
  }

  const query = search.toString()

  const body = await request<{
    emails: WireEmailAttempt[]
    limit: number
    offset: number
  }>(`/emails${query ? `?${query}` : ''}`)

  return {
    emails: body.emails.map(mapEmailAttempt),
    limit: body.limit,
    offset: body.offset,
  }
}

export async function getEmail(id: number): Promise<EmailAttemptDetails> {
  const body = await request<{
    email: WireEmailAttemptDetails
  }>(`/emails/${id}`)

  return {
    ...mapEmailAttempt(body.email),

    events: body.email.events.map(mapEmailEvent),
  }
}

export async function protectedImage(
  id: string,
  init: RequestInit = {},
): Promise<Blob> {
  const response = await fetch(
    `/api/v1/files/${encodeURIComponent(id)}/content`,
    init,
  )
  if (!response.ok)
    throw new ApiError(
      'Não foi possível carregar o arquivo.',
      response.status,
      'FILE_ERROR',
    )
  return response.blob()
}
