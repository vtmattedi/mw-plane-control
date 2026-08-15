import { useCallback, useMemo, useRef, useState, type ReactNode } from 'react'
import { ConfirmContext, type Confirmation } from './confirm-context'
import { Button } from './ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from './ui/dialog'

interface PendingConfirmation extends Confirmation {
  resolve: (confirmed: boolean) => void
}

export function AppConfirmProvider({ children }: { children: ReactNode }) {
  const [pending, setPending] = useState<PendingConfirmation | null>(null)
  const cancelRef = useRef<HTMLButtonElement>(null)
  const confirm = useCallback(
    (confirmation: Confirmation) =>
      new Promise<boolean>((resolve) => {
        setPending({ ...confirmation, resolve })
      }),
    [],
  )
  const value = useMemo(() => confirm, [confirm])

  function finish(confirmed: boolean) {
    pending?.resolve(confirmed)
    setPending(null)
  }

  return (
    <ConfirmContext.Provider value={value}>
      {children}
      <Dialog
        open={pending !== null}
        onOpenChange={(open) => {
          if (!open && pending) finish(false)
        }}
      >
        <DialogContent
          className="confirm-dialog max-w-lg"
          onOpenAutoFocus={(event) => {
            event.preventDefault()
            cancelRef.current?.focus()
          }}
          role="alertdialog"
        >
          <DialogHeader>
            <DialogTitle>{pending?.title}</DialogTitle>
            <DialogDescription>{pending?.description}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              ref={cancelRef}
              type="button"
              variant="secondary"
              onClick={() => finish(false)}
            >
              Cancelar
            </Button>
            <Button
              type="button"
              variant="destructive"
              onClick={() => finish(true)}
            >
              {pending?.confirmLabel ?? 'Confirmar'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </ConfirmContext.Provider>
  )
}
