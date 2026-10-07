import { Check, Copy, KeyRound, TriangleAlert } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Button } from './ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from './ui/dialog'
import { Input } from './ui/input'
import { useOptionalPreferences } from '@/preferences/preferences-context'

type OneTimeCredentialDialogProps = {
  credential?: string | null
  credentials?: OneTimeCredential[] | null
  onClose: () => void
  title?: string
}

export type OneTimeCredential = {
  label: string
  value: string
}

export function OneTimeCredentialDialog({
  credential,
  credentials,
  onClose,
  title,
}: OneTimeCredentialDialogProps) {
  const tr = useOptionalPreferences()?.tr ?? ((text: string) => text)
  const [copied, setCopied] = useState<string | null>(null)
  const entries =
    credentials ??
    (typeof credential === 'string'
      ? [{ label: tr('Credencial'), value: credential }]
      : [])
  const open =
    credentials !== undefined ? credentials !== null : credential !== null

  useEffect(() => setCopied(null), [credential, credentials])

  async function copyCredential(entry: OneTimeCredential) {
    await navigator.clipboard.writeText(entry.value)
    setCopied(entry.label)
  }

  async function copyAll() {
    await navigator.clipboard.writeText(
      entries.map((entry) => `${entry.label}=${entry.value}`).join('\n'),
    )
    setCopied('all')
  }

  return (
    <Dialog open={open} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <KeyRound className="text-brand-orange size-5" aria-hidden="true" />
            {title ?? tr('Nova credencial')}
          </DialogTitle>
          <DialogDescription>
            {tr(
              entries.length === 1
                ? 'A operação foi concluída. Copie e armazene a credencial em um local seguro.'
                : 'A operação foi concluída. Copie e armazene as credenciais em um local seguro.',
            )}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-3">
          {entries.map((entry) => (
            <div className="space-y-2" key={entry.label}>
              <label className="text-sm font-medium">{entry.label}</label>
              <div className="flex flex-col gap-2 sm:flex-row">
                <Input
                  aria-label={
                    entries.length === 1 ? tr('Credencial gerada') : entry.label
                  }
                  onChange={() => undefined}
                  readOnly
                  value={entry.value}
                />
                <Button
                  aria-label={`${tr('Copiar')} ${entry.label}`}
                  onClick={() => void copyCredential(entry)}
                  variant="outline"
                >
                  {copied === entry.label ? <Check /> : <Copy />}
                  {copied === entry.label ? tr('Copiado') : tr('Copiar')}
                </Button>
              </div>
            </div>
          ))}
          <div className="border-warning/30 bg-warning/10 text-warning flex items-start gap-2 rounded-xl border p-3 text-sm leading-5">
            <TriangleAlert
              className="mt-0.5 size-4 shrink-0"
              aria-hidden="true"
            />
            <p>
              {tr(
                entries.length === 1
                  ? 'Esta credencial não será exibida novamente depois que esta janela for fechada.'
                  : 'Estas credenciais não serão exibidas novamente depois que esta janela for fechada.',
              )}
            </p>
          </div>
        </div>

        <DialogFooter>
          {entries.length > 1 && (
            <Button onClick={() => void copyAll()} variant="outline">
              {copied === 'all' ? <Check /> : <Copy />}
              {copied === 'all' ? tr('Tudo copiado') : tr('Copiar todas')}
            </Button>
          )}
          <Button onClick={onClose}>{tr('Concluir')}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
