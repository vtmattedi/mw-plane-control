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

type OneTimeCredentialDialogProps = {
  credential: string | null
  onClose: () => void
  title?: string
}

export function OneTimeCredentialDialog({
  credential,
  onClose,
  title = 'Nova credencial',
}: OneTimeCredentialDialogProps) {
  const [copied, setCopied] = useState(false)

  useEffect(() => setCopied(false), [credential])

  async function copyCredential() {
    if (!credential) return
    await navigator.clipboard.writeText(credential)
    setCopied(true)
  }

  return (
    <Dialog
      open={credential !== null}
      onOpenChange={(open) => !open && onClose()}
    >
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <KeyRound className="text-brand-orange size-5" aria-hidden="true" />
            {title}
          </DialogTitle>
          <DialogDescription>
            A operação foi concluída. Copie e armazene a credencial em um local
            seguro.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-3">
          <Input
            aria-label="Credencial gerada"
            onChange={() => undefined}
            readOnly
            value={credential ?? ''}
          />
          <div className="border-warning/30 bg-warning/10 text-warning flex items-start gap-2 rounded-xl border p-3 text-sm leading-5">
            <TriangleAlert
              className="mt-0.5 size-4 shrink-0"
              aria-hidden="true"
            />
            <p>
              Esta credencial não será exibida novamente depois que esta janela
              for fechada.
            </p>
          </div>
        </div>

        <DialogFooter>
          <Button onClick={() => void copyCredential()} variant="outline">
            {copied ? <Check /> : <Copy />}
            {copied ? 'Copiado' : 'Copiar credencial'}
          </Button>
          <Button onClick={onClose}>Concluir</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
