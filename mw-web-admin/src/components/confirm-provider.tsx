import * as Dialog from '@radix-ui/react-dialog'
import { useCallback, useMemo, useRef, useState, type ReactNode } from 'react'
import { ConfirmContext, type Confirmation } from './confirm-context'
import { Button } from './ui/button'

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
      <Dialog.Root
        open={pending !== null}
        onOpenChange={(open) => {
          if (!open && pending) finish(false)
        }}
      >
        <Dialog.Portal>
          <Dialog.Overlay className="confirm-overlay" />
          <Dialog.Content
            className="confirm-dialog"
            onOpenAutoFocus={(event) => {
              event.preventDefault()
              cancelRef.current?.focus()
            }}
            role="alertdialog"
          >
            <Dialog.Title
              style={{
                fontSize: '1.25rem',
                fontWeight: 600,
                marginBottom: '0.5rem',
                borderBottom: '1px solid var(--border)',
                paddingBottom: '0.5rem',
              }}
            >
              {pending?.title}
            </Dialog.Title>
            <Dialog.Description style={{ marginBottom: '1rem' }}>
              {pending?.description}
            </Dialog.Description>
            <div className="button-row">
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
            </div>
          </Dialog.Content>
        </Dialog.Portal>
      </Dialog.Root>
    </ConfirmContext.Provider>
  )
}
