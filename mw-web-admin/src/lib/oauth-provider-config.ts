import { splitFormValues } from './form-values'

export type GoogleOAuthConfig = {
  enabled: boolean
  allowed_scopes: string[]
  allowed_callback_urls: string[]
  allowed_return_urls: string[]
  limits: {
    per_minute: number
    per_hour: number
    max_pending: number
  }
}

export type GoogleOAuthForm = {
  enabled: boolean
  allowedScopes: string
  allowedCallbackURLs: string
  allowedReturnURLs: string
  perMinute: string
  perHour: string
  maxPending: string
}

export function googleConfigToForm(config: GoogleOAuthConfig): GoogleOAuthForm {
  return {
    enabled: config.enabled,
    allowedScopes: config.allowed_scopes.join('\n'),
    allowedCallbackURLs: config.allowed_callback_urls.join('\n'),
    allowedReturnURLs: config.allowed_return_urls.join('\n'),
    perMinute: String(config.limits.per_minute),
    perHour: String(config.limits.per_hour),
    maxPending: String(config.limits.max_pending),
  }
}

function uniqueValues(value: string) {
  return [...new Set(splitFormValues(value))]
}

function positiveInteger(value: string) {
  const parsed = Number(value)
  return Number.isInteger(parsed) && parsed > 0 ? parsed : null
}

function policyURLIsValid(raw: string) {
  try {
    const parsed = new URL(raw)
    if (
      parsed.username ||
      parsed.password ||
      parsed.search ||
      parsed.hash ||
      parsed.hostname.includes('*')
    ) {
      return false
    }
    if (parsed.protocol === 'https:') return true
    if (parsed.protocol !== 'http:') return false
    return (
      parsed.hostname === 'localhost' ||
      parsed.hostname === '127.0.0.1' ||
      parsed.hostname === '::1' ||
      parsed.hostname === '[::1]'
    )
  } catch {
    return false
  }
}

export function googleFormError(form: GoogleOAuthForm) {
  const scopes = uniqueValues(form.allowedScopes)
  const callbacks = uniqueValues(form.allowedCallbackURLs)
  const returns = uniqueValues(form.allowedReturnURLs)

  if (
    positiveInteger(form.perMinute) == null ||
    positiveInteger(form.perHour) == null ||
    positiveInteger(form.maxPending) == null
  ) {
    return 'Os limites devem ser números inteiros positivos.'
  }
  if (![...callbacks, ...returns].every(policyURLIsValid)) {
    return 'Use URLs HTTPS absolutas, exatas e sem query string ou fragmento. HTTP é permitido apenas para loopback local.'
  }
  if (form.enabled && scopes.length === 0) {
    return 'Informe pelo menos um escopo antes de ativar o Google.'
  }
  if (form.enabled && callbacks.length === 0) {
    return 'Informe pelo menos uma URL de callback antes de ativar o Google.'
  }
  if (form.enabled && returns.length === 0) {
    return 'Informe pelo menos uma URL de retorno antes de ativar o Google.'
  }
  return null
}

export function googleFormToConfig(form: GoogleOAuthForm): GoogleOAuthConfig {
  const error = googleFormError(form)
  if (error) throw new Error(error)

  return {
    enabled: form.enabled,
    allowed_scopes: uniqueValues(form.allowedScopes),
    allowed_callback_urls: uniqueValues(form.allowedCallbackURLs),
    allowed_return_urls: uniqueValues(form.allowedReturnURLs),
    limits: {
      per_minute: positiveInteger(form.perMinute)!,
      per_hour: positiveInteger(form.perHour)!,
      max_pending: positiveInteger(form.maxPending)!,
    },
  }
}
