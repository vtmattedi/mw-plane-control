import * as Dialog from '@radix-ui/react-dialog'
import { CircleAlert, X } from 'lucide-react'
import { Button } from './ui/button'

interface AppAlertDialogProps {
  description: string
  onOpenChange: (open: boolean) => void
  open: boolean
  title: string
}

export function AppAlertDialog({
  description,
  onOpenChange,
  open,
  title,
}: AppAlertDialogProps) {
  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="app-alert-overlay" />
        <Dialog.Content className="app-alert-dialog">
          <span className="app-alert-icon" aria-hidden="true">
            <CircleAlert />
          </span>
          <Dialog.Title>{title}</Dialog.Title>
          <Dialog.Description>{description}</Dialog.Description>
          <Dialog.Close asChild>
            <Button type="button">Entendi</Button>
          </Dialog.Close>
          <Dialog.Close aria-label="Fechar aviso" className="app-alert-close">
            <X aria-hidden="true" />
          </Dialog.Close>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}
