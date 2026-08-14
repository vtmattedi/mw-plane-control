import { type FormEvent, useEffect, useMemo, useState } from 'react'
import {
  CheckCircle2,
  Eye,
  EyeOff,
  FlaskConical,
  KeyRound,
  LoaderCircle,
  RefreshCw,
  Send,
  X,
  XCircle,
} from 'lucide-react'

import { Button } from '@/components/ui/button'
import { usePreferences } from '@/preferences/preferences-context'

const EMAIL_API_BASE_URL = (
  import.meta.env.VITE_EMAIL_API_BASE_URL || 'https://mailgo.mattediworks.com'
).replace(/\/+$/, '')

type BodyMode = 'html' | 'text'

type SendResult = {
  id?: string
  replayed?: boolean
  error?: string
  message?: string
  [key: string]: unknown
}

type RequestResult = {
  ok: boolean
  status: number
  statusText: string
  body: SendResult | string | null
}

type PersistedTesterState = {
  open: boolean
  bodyMode: BodyMode
  idempotencyKey: string
  from: string
  to: string
  subject: string
  html: string
  text: string
  result: RequestResult | null
}

const TESTER_STORAGE_KEY = 'mw-email-admin:tester:v1'

function createIdempotencyKey() {
  return `admin-test-${crypto.randomUUID()}`
}

function createDefaultPersistedState(): PersistedTesterState {
  return {
    open: false,
    bodyMode: 'html',
    idempotencyKey: createIdempotencyKey(),
    from: '',
    to: '',
    subject: '',
    html: '',
    text: '',
    result: null,
  }
}

function loadPersistedState(): PersistedTesterState {
  const fallback = createDefaultPersistedState()

  if (typeof window === 'undefined') {
    return fallback
  }

  try {
    const stored = window.localStorage.getItem(TESTER_STORAGE_KEY)

    if (!stored) {
      return fallback
    }

    const parsed = JSON.parse(stored) as Partial<PersistedTesterState>

    return {
      open: typeof parsed.open === 'boolean' ? parsed.open : fallback.open,
      bodyMode:
        parsed.bodyMode === 'html' || parsed.bodyMode === 'text'
          ? parsed.bodyMode
          : fallback.bodyMode,
      idempotencyKey:
        typeof parsed.idempotencyKey === 'string' &&
        parsed.idempotencyKey.trim().length > 0
          ? parsed.idempotencyKey
          : fallback.idempotencyKey,
      from: typeof parsed.from === 'string' ? parsed.from : fallback.from,
      to: typeof parsed.to === 'string' ? parsed.to : fallback.to,
      subject:
        typeof parsed.subject === 'string' ? parsed.subject : fallback.subject,
      html: typeof parsed.html === 'string' ? parsed.html : fallback.html,
      text: typeof parsed.text === 'string' ? parsed.text : fallback.text,
      result:
        parsed.result && typeof parsed.result === 'object'
          ? (parsed.result as RequestResult)
          : null,
    }
  } catch {
    return fallback
  }
}

function savePersistedState(state: PersistedTesterState) {
  if (typeof window === 'undefined') {
    return
  }

  try {
    window.localStorage.setItem(TESTER_STORAGE_KEY, JSON.stringify(state))
  } catch {
    // localStorage can fail in private/restricted contexts or when full.
    // The tester should remain usable without persistence.
  }
}

function parseRecipients(value: string) {
  return value
    .split(/[,;\n]/)
    .map((recipient) => recipient.trim())
    .filter(Boolean)
}

function formatResponseBody(body: RequestResult['body']) {
  if (body == null) return '(empty response)'
  if (typeof body === 'string') return body
  return JSON.stringify(body, null, 2)
}

export default function EmailTester() {
  const { language } = usePreferences()
  const pt: Record<string, string> = {
    'API key is required.': 'A chave de API é obrigatória.',
    'Idempotency-Key is required.': 'A Idempotency-Key é obrigatória.',
    'From is required.': 'O remetente é obrigatório.',
    'At least one recipient is required.':
      'Informe pelo menos um destinatário.',
    'Subject is required.': 'O assunto é obrigatório.',
    'Provide an HTML or text body.': 'Informe um corpo HTML ou texto.',
    'The request could not be completed.':
      'Não foi possível concluir a requisição.',
    'Test email': 'Testar e-mail',
    'Test email API': 'Testar API de e-mail',
    'Sends a real request to': 'Envia uma requisição real para',
    'Close tester': 'Fechar testador',
    'Project API key': 'Chave de API do projeto',
    'Hide API key': 'Ocultar chave de API',
    'Show API key': 'Exibir chave de API',
    'Generate new idempotency key': 'Gerar nova chave de idempotência',
    From: 'De',
    To: 'Para',
    Subject: 'Assunto',
    'Test message': 'Mensagem de teste',
    'Plain text': 'Texto simples',
    'At least one body is required': 'Pelo menos um corpo é obrigatório',
    replayed: 'repetido',
    'new send': 'novo envio',
    Clear: 'Limpar',
    recipient: 'destinatário',
    recipients: 'destinatários',
    'Sending…': 'Enviando…',
    'Send test': 'Enviar teste',
  }
  const t = (text: string) => (language === 'pt' ? (pt[text] ?? text) : text)
  const [initialState] = useState(loadPersistedState)

  const [open, setOpen] = useState(initialState.open)
  const [showApiKey, setShowApiKey] = useState(false)
  const [bodyMode, setBodyMode] = useState<BodyMode>(initialState.bodyMode)

  // Intentionally never persisted.
  const [apiKey, setApiKey] = useState('')

  const [idempotencyKey, setIdempotencyKey] = useState(
    initialState.idempotencyKey,
  )
  const [from, setFrom] = useState(initialState.from)
  const [to, setTo] = useState(initialState.to)
  const [subject, setSubject] = useState(initialState.subject)
  const [html, setHtml] = useState(initialState.html)
  const [text, setText] = useState(initialState.text)

  const [sending, setSending] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [result, setResult] = useState<RequestResult | null>(
    initialState.result,
  )

  const recipients = useMemo(() => parseRecipients(to), [to])

  const canSend =
    apiKey.trim().length > 0 &&
    idempotencyKey.trim().length > 0 &&
    from.trim().length > 0 &&
    recipients.length > 0 &&
    subject.trim().length > 0 &&
    (html.trim().length > 0 || text.trim().length > 0) &&
    !sending

  useEffect(() => {
    savePersistedState({
      open,
      bodyMode,
      idempotencyKey,
      from,
      to,
      subject,
      html,
      text,
      result,
    })
  }, [open, bodyMode, idempotencyKey, from, to, subject, html, text, result])

  useEffect(() => {
    if (!open) return

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && !sending) {
        setOpen(false)
      }
    }

    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [open, sending])

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    setError(null)
    setResult(null)

    if (!apiKey.trim()) {
      setError(t('API key is required.'))
      return
    }

    if (!idempotencyKey.trim()) {
      setError(t('Idempotency-Key is required.'))
      return
    }

    if (!from.trim()) {
      setError(t('From is required.'))
      return
    }

    if (recipients.length === 0) {
      setError(t('At least one recipient is required.'))
      return
    }

    if (!subject.trim()) {
      setError(t('Subject is required.'))
      return
    }

    if (!html.trim() && !text.trim()) {
      setError(t('Provide an HTML or text body.'))
      return
    }

    const payload = {
      from: from.trim(),
      to: recipients,
      subject: subject.trim(),
      ...(html.trim() ? { html } : {}),
      ...(text.trim() ? { text } : {}),
    }

    setSending(true)

    try {
      const response = await fetch(`${EMAIL_API_BASE_URL}/v1/emails`, {
        method: 'POST',
        headers: {
          Authorization: `Bearer ${apiKey.trim()}`,
          'Content-Type': 'application/json',
          'Idempotency-Key': idempotencyKey.trim(),
        },
        body: JSON.stringify(payload),
      })

      const rawBody = await response.text()

      let parsedBody: SendResult | string | null = null

      if (rawBody) {
        try {
          parsedBody = JSON.parse(rawBody) as SendResult
        } catch {
          parsedBody = rawBody
        }
      }

      setResult({
        ok: response.ok,
        status: response.status,
        statusText: response.statusText,
        body: parsedBody,
      })

      if (!response.ok) {
        const serviceMessage =
          parsedBody && typeof parsedBody === 'object'
            ? String(parsedBody.error ?? parsedBody.message ?? '')
            : ''
        const originDenied =
          response.status === 403 && serviceMessage.includes('origin')
        setError(
          originDenied
            ? language === 'pt'
              ? `A origem ${window.location.hostname} não está permitida neste projeto. Adicione "${window.location.hostname}" em Mail → Configuração.`
              : `Origin ${window.location.hostname} is not allowed for this project. Add "${window.location.hostname}" under Mail → Configuration.`
            : language === 'pt'
              ? `A requisição falhou com HTTP ${response.status}.`
              : `Request failed with HTTP ${response.status}.`,
        )
      }
    } catch (requestError) {
      const message =
        requestError instanceof Error
          ? requestError.message
          : t('The request could not be completed.')

      setError(message)
    } finally {
      setSending(false)
    }
  }

  function resetDraft() {
    setApiKey('')
    setIdempotencyKey(createIdempotencyKey())
    setFrom('')
    setTo('')
    setSubject('')
    setHtml('')
    setText('')
    setBodyMode('html')
    setShowApiKey(false)
    setError(null)
    setResult(null)
  }

  return (
    <>
      <Button
        type="button"
        onClick={() => setOpen(true)}
        className="fixed right-5 bottom-5 z-30 gap-2 rounded-full px-5 shadow-lg"
      >
        <FlaskConical className="size-4" aria-hidden="true" />
        {t('Test email')}
      </Button>

      {open ? (
        <section
          role="dialog"
          aria-modal="true"
          aria-labelledby="email-tester-title"
          className="fixed right-4 bottom-4 z-50 flex h-[min(780px,calc(100dvh-2rem))] w-[min(760px,calc(100vw-2rem))] flex-col overflow-hidden rounded-xl border bg-[var(--secondary)] shadow-2xl"
        >
          <header className="flex shrink-0 items-center justify-between border-b px-4 py-3">
            <div className="min-w-0">
              <div className="flex items-center gap-2">
                <FlaskConical
                  className="text-muted-foreground size-4"
                  aria-hidden="true"
                />
                <h2
                  id="email-tester-title"
                  className="truncate text-sm font-semibold"
                >
                  {t('Test email API')}
                </h2>
              </div>
              <p className="text-muted-foreground mt-0.5 text-xs">
                {t('Sends a real request to')} {EMAIL_API_BASE_URL}/v1/emails
              </p>
            </div>

            <Button
              type="button"
              variant="ghost"
              size="icon"
              onClick={() => setOpen(false)}
              disabled={sending}
              aria-label={t('Close tester')}
            >
              <X className="size-4" aria-hidden="true" />
            </Button>
          </header>

          <form
            onSubmit={handleSubmit}
            className="flex min-h-0 flex-1 flex-col"
          >
            <div className="min-h-0 flex-1 overflow-y-auto">
              <div className="space-y-4 p-4">
                <div className="grid gap-4 md:grid-cols-[1fr_1.15fr]">
                  <label className="space-y-1.5">
                    <span className="text-muted-foreground flex items-center gap-1.5 text-xs font-medium">
                      <KeyRound className="size-3.5" aria-hidden="true" />
                      {t('Project API key')}
                    </span>

                    <div className="bg-background focus-within:ring-ring flex rounded-md border focus-within:ring-2">
                      <input
                        type={showApiKey ? 'text' : 'password'}
                        value={apiKey}
                        onChange={(event) => setApiKey(event.target.value)}
                        autoComplete="off"
                        spellCheck={false}
                        placeholder="mw_live_..."
                        className="min-w-0 flex-1 bg-transparent px-3 py-2 text-sm outline-none"
                      />

                      <button
                        type="button"
                        onClick={() => setShowApiKey((value) => !value)}
                        className="text-muted-foreground hover:text-foreground grid w-10 place-items-center"
                        aria-label={t(
                          showApiKey ? 'Hide API key' : 'Show API key',
                        )}
                      >
                        {showApiKey ? (
                          <EyeOff className="size-4" aria-hidden="true" />
                        ) : (
                          <Eye className="size-4" aria-hidden="true" />
                        )}
                      </button>
                    </div>
                  </label>

                  <label className="space-y-1.5">
                    <span className="text-muted-foreground text-xs font-medium">
                      Idempotency-Key
                    </span>

                    <div className="bg-background focus-within:ring-ring flex rounded-md border focus-within:ring-2">
                      <input
                        type="text"
                        value={idempotencyKey}
                        onChange={(event) =>
                          setIdempotencyKey(event.target.value)
                        }
                        spellCheck={false}
                        className="min-w-0 flex-1 bg-transparent px-3 py-2 font-mono text-xs outline-none"
                      />

                      <button
                        type="button"
                        onClick={() =>
                          setIdempotencyKey(createIdempotencyKey())
                        }
                        className="text-muted-foreground hover:text-foreground grid w-10 place-items-center"
                        aria-label={t('Generate new idempotency key')}
                        title={t('Generate new idempotency key')}
                      >
                        <RefreshCw className="size-4" aria-hidden="true" />
                      </button>
                    </div>
                  </label>
                </div>

                <div className="space-y-3 rounded-lg border">
                  <label className="grid grid-cols-[4.5rem_1fr] items-center border-b px-3">
                    <span className="text-muted-foreground text-xs font-medium">
                      {t('From')}
                    </span>
                    <input
                      type="text"
                      value={from}
                      onChange={(event) => setFrom(event.target.value)}
                      placeholder="MattediWorks <no-reply@example.com>"
                      className="min-w-0 bg-transparent py-2.5 text-sm outline-none"
                    />
                  </label>

                  <label className="grid grid-cols-[4.5rem_1fr] items-start border-b px-3">
                    <span className="text-muted-foreground pt-2.5 text-xs font-medium">
                      {t('To')}
                    </span>
                    <textarea
                      value={to}
                      onChange={(event) => setTo(event.target.value)}
                      placeholder="user@example.com, other@example.com"
                      rows={2}
                      className="min-h-10 min-w-0 resize-none bg-transparent py-2.5 text-sm outline-none"
                    />
                  </label>

                  <label className="grid grid-cols-[4.5rem_1fr] items-center px-3">
                    <span className="text-muted-foreground text-xs font-medium">
                      {t('Subject')}
                    </span>
                    <input
                      type="text"
                      value={subject}
                      onChange={(event) => setSubject(event.target.value)}
                      placeholder={t('Test message')}
                      className="min-w-0 bg-transparent py-2.5 text-sm outline-none"
                    />
                  </label>
                </div>

                <div className="overflow-hidden rounded-lg border">
                  <div className="bg-muted/30 flex items-center gap-1 border-b px-2 py-1.5">
                    <button
                      type="button"
                      onClick={() => setBodyMode('html')}
                      className={`rounded-md px-3 py-1.5 text-xs font-medium transition-colors ${
                        bodyMode === 'html'
                          ? 'bg-background text-foreground shadow-sm'
                          : 'text-muted-foreground hover:text-foreground'
                      }`}
                    >
                      HTML
                      {html.trim() ? ' •' : ''}
                    </button>

                    <button
                      type="button"
                      onClick={() => setBodyMode('text')}
                      className={`rounded-md px-3 py-1.5 text-xs font-medium transition-colors ${
                        bodyMode === 'text'
                          ? 'bg-background text-foreground shadow-sm'
                          : 'text-muted-foreground hover:text-foreground'
                      }`}
                    >
                      {t('Plain text')}
                      {text.trim() ? ' •' : ''}
                    </button>

                    <span className="text-muted-foreground ml-auto pr-2 text-[11px]">
                      {t('At least one body is required')}
                    </span>
                  </div>

                  {bodyMode === 'html' ? (
                    <textarea
                      value={html}
                      onChange={(event) => setHtml(event.target.value)}
                      placeholder={
                        '<h1>Hello</h1>\n<p>This is a test email.</p>'
                      }
                      spellCheck={false}
                      className="bg-background min-h-64 w-full resize-y p-4 font-mono text-sm leading-6 outline-none"
                    />
                  ) : (
                    <textarea
                      value={text}
                      onChange={(event) => setText(event.target.value)}
                      placeholder={'Hello\n\nThis is a test email.'}
                      className="bg-background min-h-64 w-full resize-y p-4 text-sm leading-6 outline-none"
                    />
                  )}
                </div>

                {error ? (
                  <div
                    role="alert"
                    className="border-destructive/30 bg-destructive/5 flex items-start gap-2 rounded-lg border p-3 text-sm"
                  >
                    <XCircle
                      className="text-destructive mt-0.5 size-4 shrink-0"
                      aria-hidden="true"
                    />
                    <span>{error}</span>
                  </div>
                ) : null}

                {result ? (
                  <div className="overflow-hidden rounded-lg border">
                    <div className="flex items-center gap-2 border-b px-3 py-2">
                      {result.ok ? (
                        <CheckCircle2
                          className="size-4 text-emerald-600"
                          aria-hidden="true"
                        />
                      ) : (
                        <XCircle
                          className="text-destructive size-4"
                          aria-hidden="true"
                        />
                      )}

                      <span className="text-sm font-medium">
                        HTTP {result.status}
                        {result.statusText ? ` ${result.statusText}` : ''}
                      </span>

                      {typeof result.body === 'object' &&
                      result.body &&
                      'replayed' in result.body ? (
                        <span className="text-muted-foreground ml-auto rounded-full border px-2 py-0.5 text-[11px]">
                          {t(result.body.replayed ? 'replayed' : 'new send')}
                        </span>
                      ) : null}
                    </div>

                    <pre className="bg-muted/20 max-h-52 overflow-auto p-3 font-mono text-xs leading-5 break-words whitespace-pre-wrap">
                      {formatResponseBody(result.body)}
                    </pre>
                  </div>
                ) : null}
              </div>
            </div>

            <footer className="bg-background flex shrink-0 items-center justify-between gap-3 border-t px-4 py-3">
              <button
                type="button"
                onClick={resetDraft}
                disabled={sending}
                className="text-muted-foreground hover:text-foreground text-xs disabled:opacity-50"
              >
                {t('Clear')}
              </button>

              <div className="flex items-center gap-3">
                <span className="text-muted-foreground hidden text-xs sm:inline">
                  {recipients.length}{' '}
                  {t(recipients.length === 1 ? 'recipient' : 'recipients')}
                </span>

                <Button
                  type="submit"
                  disabled={!canSend}
                  className="min-w-28 gap-2"
                >
                  {sending ? (
                    <LoaderCircle
                      className="size-4 animate-spin"
                      aria-hidden="true"
                    />
                  ) : (
                    <Send className="size-4" aria-hidden="true" />
                  )}

                  {t(sending ? 'Sending…' : 'Send test')}
                </Button>
              </div>
            </footer>
          </form>
        </section>
      ) : null}
    </>
  )
}
